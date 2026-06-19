//go:build !go1.25

package agent

import (
	"fmt"
	"runtime"
)

// newFlightRecorder 在低于 Go 1.25 的工具链上不可用：runtime/trace.FlightRecorder
// 自 Go 1.25 才进入标准库。此处优雅降级，让 Agent 其余功能照常工作。
func newFlightRecorder() (flightRecorder, error) {
	return nil, fmt.Errorf("flight recorder requires Go 1.25+ (built with %s)", runtime.Version())
}
