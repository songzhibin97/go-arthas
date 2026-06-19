// Command arthas-toolexec 是 `go build -toolexec` 的包装器，用于在编译期对
// 配置选中的函数注入 watch/trace 钩子（路线 B：编译期插桩）。
//
// 它拦截 `compile` 调用：对命中目标的包，重写其 .go 源文件注入钩子，并把
// arthastrace 的归档加入该包的 importcfg；其余所有 tool 调用原样透传。
//
// 配置经环境变量传入（由 go-arthas build 包装器设置）：
//
//	ARTHAS_TARGETS  逗号分隔的方法 id（pkg.Func 或 pkg.(*T).Method）
//	ARTHAS_ARCHIVE  arthastrace 包归档(.a)路径，注入目标包 importcfg
//	ARTHAS_DEBUG    非空则向 stderr 打印调试信息
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "arthas-toolexec: no tool specified")
		os.Exit(2)
	}
	tool, toolArgs := args[0], args[1:]

	if isCompile(tool) {
		if newArgs, err := rewriteCompile(toolArgs); err != nil {
			debugf("rewrite failed, falling back to original args: %v", err)
		} else {
			toolArgs = newArgs
		}
	}
	runTool(tool, toolArgs)
}

func isCompile(tool string) bool {
	base := filepath.Base(tool)
	return base == "compile" || base == "compile.exe"
}

// rewriteCompile 若当前编译的包含有目标函数，则注入并返回新的参数列表
func rewriteCompile(args []string) ([]string, error) {
	targets := parseTargets(os.Getenv("ARTHAS_TARGETS"))
	if len(targets) == 0 {
		return args, nil
	}
	pkgPath := flagValue(args, "-p")
	if pkgPath == "" || !pkgHasTarget(pkgPath, targets) {
		return args, nil
	}

	tmpDir, err := os.MkdirTemp("", "arthas-toolexec-")
	if err != nil {
		return args, err
	}

	newArgs := append([]string(nil), args...)
	anyInjected := false
	for i, a := range newArgs {
		if strings.HasPrefix(a, "-") || !strings.HasSuffix(a, ".go") {
			continue
		}
		src, err := os.ReadFile(a)
		if err != nil {
			continue
		}
		out, injected, err := instrumentSource(a, pkgPath, src, targets)
		if err != nil {
			debugf("instrument %s: %v", a, err)
			continue
		}
		if len(injected) == 0 {
			continue
		}
		newPath := filepath.Join(tmpDir, filepath.Base(a))
		if err := os.WriteFile(newPath, out, 0o644); err != nil {
			continue
		}
		newArgs[i] = newPath
		anyInjected = true
		debugf("instrumented %s in %s: %v", filepath.Base(a), pkgPath, injected)
	}

	if !anyInjected {
		return args, nil
	}

	if archive := os.Getenv("ARTHAS_ARCHIVE"); archive != "" {
		if err := patchImportcfg(newArgs, archive); err != nil {
			return args, fmt.Errorf("patch importcfg: %w", err)
		}
	} else {
		return args, fmt.Errorf("ARTHAS_ARCHIVE not set; cannot resolve arthastrace for %s", pkgPath)
	}
	return newArgs, nil
}

func pkgHasTarget(pkgPath string, targets map[string]bool) bool {
	prefix := pkgPath + "."
	for id := range targets {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

// patchImportcfg 把 arthastrace 的归档追加进 compile 的 -importcfg 文件
func patchImportcfg(args []string, archive string) error {
	idx := -1
	for i, a := range args {
		if a == "-importcfg" && i+1 < len(args) {
			idx = i + 1
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("no -importcfg in compile args")
	}
	data, err := os.ReadFile(args[idx])
	if err != nil {
		return err
	}
	if strings.Contains(string(data), "packagefile "+tracePkg+"=") {
		return nil // 已包含
	}
	newData := string(data) + fmt.Sprintf("packagefile %s=%s\n", tracePkg, archive)
	newFile := args[idx] + ".arthas"
	if err := os.WriteFile(newFile, []byte(newData), 0o644); err != nil {
		return err
	}
	args[idx] = newFile
	return nil
}

func runTool(tool string, args []string) {
	cmd := exec.Command(tool, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "arthas-toolexec: run %s: %v\n", tool, err)
		os.Exit(1)
	}
}

func parseTargets(s string) map[string]bool {
	m := map[string]bool{}
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			m[t] = true
		}
	}
	return m
}

func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, flag+"=") {
			return strings.TrimPrefix(a, flag+"=")
		}
	}
	return ""
}

func debugf(format string, a ...interface{}) {
	if os.Getenv("ARTHAS_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[arthas-toolexec] "+format+"\n", a...)
	}
}
