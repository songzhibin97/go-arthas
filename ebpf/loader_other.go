//go:build !linux

package ebpf

import (
	"fmt"
	"runtime"
)

// Attach 在非 Linux 平台不可用：eBPF uprobe 需要 Linux 内核（且通常需要
// root / CAP_BPF 与较新内核）。二进制静态分析（OpenTarget/ResolveFunc）
// 仍可在任意平台对 Linux 目标二进制运行。
func Attach(opts AttachOptions) (Attacher, error) {
	return nil, fmt.Errorf("eBPF attach requires Linux with root/CAP_BPF (current OS: %s)", runtime.GOOS)
}
