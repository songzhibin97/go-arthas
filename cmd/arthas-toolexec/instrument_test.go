package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const sampleSrc = `package demo

import "fmt"

func Add(a, b int) int {
	return a + b
}

func Named(x string) (n int, err error) {
	_ = fmt.Sprint(x)
	return len(x), nil
}

func Multi(a int) (int, string) {
	return a, "x"
}

func Variadic(prefix string, xs ...int) int {
	return len(xs)
}

type T struct{}

func (t *T) Method(v int) int {
	return v
}

func Generic[K any](k K) K {
	return k
}

func NotTarget() {}
`

func TestInstrumentSource(t *testing.T) {
	targets := map[string]bool{
		"demo.Add":         true,
		"demo.Named":       true,
		"demo.Multi":       true,
		"demo.Variadic":    true,
		"demo.(*T).Method": true,
		"demo.Generic":     true, // 泛型，应跳过
	}

	out, injected, err := instrumentSource("demo.go", "demo", []byte(sampleSrc), targets)
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}

	got := map[string]bool{}
	for _, id := range injected {
		got[id] = true
	}
	for _, want := range []string{"demo.Add", "demo.Named", "demo.Multi", "demo.Variadic", "demo.(*T).Method"} {
		if !got[want] {
			t.Errorf("expected %s to be injected, injected=%v", want, injected)
		}
	}
	if got["demo.Generic"] {
		t.Error("generic function should be skipped")
	}

	s := string(out)
	if !strings.Contains(s, tracePkg) {
		t.Error("missing arthastrace import")
	}
	if !strings.Contains(s, "func init()") || !strings.Contains(s, `Register("demo.Add")`) {
		t.Error("missing init Register call")
	}
	if !strings.Contains(s, `Enabled("demo.Add")`) {
		t.Error("missing Enabled gate for Add")
	}
	if !strings.Contains(s, "__arthas_ret0") {
		t.Error("unnamed results should be rewritten to named returns")
	}
	if strings.Contains(s, `Enabled("demo.NotTarget")`) {
		t.Error("non-target function should not be instrumented")
	}

	// 输出必须是合法 Go 源码（format.Source 已保证；再 reparse 确认）
	if _, err := parser.ParseFile(token.NewFileSet(), "", out, 0); err != nil {
		t.Fatalf("instrumented output does not parse: %v\n%s", err, s)
	}
}

func TestInstrumentSource_NoTargets(t *testing.T) {
	out, injected, err := instrumentSource("demo.go", "demo", []byte(sampleSrc), map[string]bool{"other.X": true})
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	if injected != nil {
		t.Errorf("expected no injection, got %v", injected)
	}
	if string(out) != sampleSrc {
		t.Error("source should be returned unchanged when no targets match")
	}
}

func TestFuncIDMethod(t *testing.T) {
	targets := map[string]bool{"demo.(*T).Method": true}
	_, injected, err := instrumentSource("demo.go", "demo", []byte(sampleSrc), targets)
	if err != nil {
		t.Fatal(err)
	}
	if len(injected) != 1 || injected[0] != "demo.(*T).Method" {
		t.Errorf("method id mismatch: %v", injected)
	}
}
