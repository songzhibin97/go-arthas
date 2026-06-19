package ebpf

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildSample 交叉编译一个 Linux 目标二进制用于静态分析测试（不依赖运行 Linux）
func buildSample(t *testing.T, goarch string) string {
	t.Helper()
	dir := t.TempDir()
	src := `package main

import "fmt"

//go:noinline
func Target(a, b int) (int, error) {
	if a < 0 {
		return 0, fmt.Errorf("neg")
	}
	return a + b, nil
}

func main() {
	r, _ := Target(1, 2)
	fmt.Println(r)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module sample\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "sample")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+goarch, "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build sample (%s): %v\n%s", goarch, err, out)
	}
	return bin
}

func TestOpenTargetAndResolve(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			bin := buildSample(t, arch)
			tb, err := OpenTarget(bin)
			if err != nil {
				t.Fatalf("OpenTarget: %v", err)
			}
			defer tb.Close()

			if tb.GOARCH != arch {
				t.Errorf("GOARCH=%q want %q", tb.GOARCH, arch)
			}
			if tb.GoVersion == "" {
				t.Error("GoVersion should be populated from buildinfo")
			}
			if tb.GoMajorMinor() < 117 {
				t.Errorf("GoMajorMinor=%d, expected >=117 for a modern toolchain", tb.GoMajorMinor())
			}
			if !tb.UsesRegisterABI() {
				t.Errorf("modern %s build should use register ABI", arch)
			}

			ft, err := tb.ResolveFunc("main.Target")
			if err != nil {
				t.Fatalf("ResolveFunc: %v", err)
			}
			if ft.EntryAddr == 0 || ft.Size == 0 {
				t.Errorf("bad func target: %+v", ft)
			}
			// main.Target 有两个 return 语句，编译器为每个出口各发一条 RET。
			// 断言**精确数量**：仅检查 len>0 时，「只挂第一个 RET」这类回归不会被发现——
			// 而漏挂 RET = 漏观察函数返回，正是 uprobe-on-RET 安全方案的核心前提。
			const wantRets = 2
			if len(ft.ReturnOffs) != wantRets {
				t.Errorf("RET count = %d, want %d (offsets=%v)", len(ft.ReturnOffs), wantRets, ft.ReturnOffs)
			}
			// RET 偏移必须落在函数体内，且按反汇编顺序严格递增（不重复、不乱序）
			prev := -1
			for _, off := range ft.ReturnOffs {
				if off >= ft.Size {
					t.Errorf("RET offset %d out of function bounds (size %d)", off, ft.Size)
				}
				if int(off) <= prev {
					t.Errorf("RET offsets not strictly increasing: %v", ft.ReturnOffs)
				}
				prev = int(off)
			}
		})
	}
}

func TestResolveFuncMissing(t *testing.T) {
	bin := buildSample(t, "amd64")
	tb, err := OpenTarget(bin)
	if err != nil {
		t.Fatal(err)
	}
	defer tb.Close()

	if _, err := tb.ResolveFunc("main.DoesNotExist"); err == nil {
		t.Error("expected error for missing function")
	}

	funcs := tb.ListFuncs("main.Target")
	if len(funcs) == 0 {
		t.Error("ListFuncs should find main.Target")
	}
}

func TestGoMajorMinorParsing(t *testing.T) {
	cases := map[string]int{
		"go1.25.0": 125,
		"go1.21":   121,
		"go1.17.5": 117,
		"go1.9":    109,
		"":         0,
		"weird":    0,
	}
	for v, want := range cases {
		tb := &TargetBinary{GoVersion: v}
		if got := tb.GoMajorMinor(); got != want {
			t.Errorf("GoMajorMinor(%q)=%d want %d", v, got, want)
		}
	}
}
