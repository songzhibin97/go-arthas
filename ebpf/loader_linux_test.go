//go:build linux

package ebpf

import (
	"strings"
	"testing"
)

// TestAttach_RejectsStackABI 守护：目标用栈传参（RegisterABI=false，Go < 1.17）时，
// Attach 必须明确报错而非静默上报垃圾寄存器值。该守护在加载 BPF 之前短路，无需 root。
func TestAttach_RejectsStackABI(t *testing.T) {
	_, err := Attach(AttachOptions{RegisterABI: false})
	if err == nil {
		t.Fatal("expected error for stack-ABI target, got nil (would report garbage register values)")
	}
	if !strings.Contains(err.Error(), "stack-based") {
		t.Fatalf("error should explain the stack-ABI limitation, got: %v", err)
	}
}
