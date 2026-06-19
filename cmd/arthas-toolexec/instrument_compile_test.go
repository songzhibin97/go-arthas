package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// traceStub 是 arthastrace 的最小桩实现，提供注入代码用到的 API，使织入产物可在
// 不联网、不依赖真实仓库的临时 module 内编译并运行。enabled 默认为 true，让 defer
// 中的 recover 分支生效，用于验证 panic 传播。
const traceStub = `package arthastrace

type Arg struct{ Name, Value string }
type Invocation struct{}

var enabled = true

func Enabled(id string) bool                  { return enabled }
func SetWatch(id string, on bool)             { enabled = on }
func Enter(id string, args []Arg) *Invocation { return &Invocation{} }
func (inv *Invocation) Exit(results []Arg, rec any) {}
func Register(id string)                            {}
func Format(v any) string                           { return "" }
`

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupTraceModule 写出一个自包含的 module github.com/songzhibin97/go-arthas，内含
// arthastrace 桩。织入产物可放到该 module 下编译/运行（无外部依赖、无 go.sum）。
func setupTraceModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module github.com/songzhibin97/go-arthas\n\ngo 1.25\n")
	mustWrite(t, filepath.Join(dir, "arthastrace", "arthastrace.go"), traceStub)
	return dir
}

// TestInstrument_CompilesNamedBlankReturn 守护 (a int, _ error) 这类「具名 + 空白」
// 混合返回值的改写：旧实现把所有返回值统一重命名为 __arthas_retN，丢掉了函数体仍在
// 引用的具名返回值 a，导致织入产物 `undefined: a` 编译失败。仅 re-parse 的测试无法发现
// （语法合法、语义错误），故这里实际编译织入产物。
func TestInstrument_CompilesNamedBlankReturn(t *testing.T) {
	src := `package demo

func F(x int) (a int, _ error) { // 具名 + 空白混合
	a = x + 1
	return
}

func G(x int) (int, error) { // 全无名
	return x, nil
}

func H(x int) (n int, err error) { // 全具名、无空白
	n = x
	return
}
`
	out, injected, err := instrumentSource("demo.go", "demo", []byte(src), map[string]bool{
		"demo.F": true, "demo.G": true, "demo.H": true,
	})
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	if len(injected) != 3 {
		t.Fatalf("expected 3 functions injected, got %v", injected)
	}

	mod := setupTraceModule(t)
	mustWrite(t, filepath.Join(mod, "demo", "demo.go"), string(out))

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = mod
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("instrumented code failed to compile:\n%s\nerr=%v\n--- instrumented source ---\n%s", combined, err, out)
	}
}

// TestInstrument_PreservesPanicPropagation 守护本阶段最高风险行为：注入的 recover()
// 必须在记录后重新 panic，绝不能吞掉异常。否则被观察函数的 panic 会被静默吃掉，改变
// 程序语义。这里实际编译并运行织入产物，断言 panic 仍传播到调用方。
func TestInstrument_PreservesPanicPropagation(t *testing.T) {
	src := `package demo

func P(x int) int {
	if x < 0 {
		panic("neg")
	}
	return x
}
`
	out, injected, err := instrumentSource("demo.go", "demo", []byte(src), map[string]bool{"demo.P": true})
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	if len(injected) != 1 {
		t.Fatalf("expected P injected, got %v", injected)
	}

	mod := setupTraceModule(t)
	mustWrite(t, filepath.Join(mod, "demo", "demo.go"), string(out))
	mainSrc := `package main

import (
	"fmt"

	"github.com/songzhibin97/go-arthas/demo"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("RECOVERED:%v\n", r)
			return
		}
		fmt.Println("NO_PANIC")
	}()
	_ = demo.P(-1) // watch 默认开启；织入后必须仍 panic("neg")
}
`
	mustWrite(t, filepath.Join(mod, "cmd", "main.go"), mainSrc)

	cmd := exec.Command("go", "run", "./cmd")
	cmd.Dir = mod
	combined, _ := cmd.CombinedOutput() // 调用方 recover 了 panic，故正常退出
	got := string(combined)
	if !strings.Contains(got, "RECOVERED:neg") {
		t.Fatalf("injected recover() swallowed the panic (must re-panic); output:\n%s\n--- instrumented source ---\n%s", got, out)
	}
}
