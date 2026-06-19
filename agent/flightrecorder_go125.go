//go:build go1.25

package agent

import (
	"io"
	"runtime/trace"
	"time"
)

// realFlightRecorder 包装 Go 1.25 标准库 runtime/trace.FlightRecorder
type realFlightRecorder struct {
	fr *trace.FlightRecorder
}

// newFlightRecorder 创建基于标准库的飞行记录器，保留最近至少 5 秒的执行轨迹
func newFlightRecorder() (flightRecorder, error) {
	fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{
		MinAge: 5 * time.Second,
	})
	return &realFlightRecorder{fr: fr}, nil
}

func (r *realFlightRecorder) Start() error                       { return r.fr.Start() }
func (r *realFlightRecorder) WriteTo(w io.Writer) (int64, error) { return r.fr.WriteTo(w) }
func (r *realFlightRecorder) Stop()                              { r.fr.Stop() }
