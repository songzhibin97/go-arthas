package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"sync"
)

// flightRecorder 抽象 Go 执行轨迹的飞行记录器，便于在不支持的 Go 版本上降级。
// 实际实现见 flightrecorder_go125.go（Go 1.25+）与 flightrecorder_stub.go（更低版本）。
type flightRecorder interface {
	Start() error
	WriteTo(w io.Writer) (int64, error)
	Stop()
}

var (
	errFlightAlreadyRunning = fmt.Errorf("flight recorder already running")
	errFlightNotRunning     = fmt.Errorf("flight recorder not running")
)

// flightRecorderManager 管理飞行记录器生命周期（运行时全局至多一个）
type flightRecorderManager struct {
	mu      sync.Mutex
	fr      flightRecorder
	running bool
}

func (m *flightRecorderManager) start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return errFlightAlreadyRunning
	}
	fr, err := newFlightRecorder()
	if err != nil {
		return err
	}
	if err := fr.Start(); err != nil {
		return err
	}
	m.fr = fr
	m.running = true
	return nil
}

// snapshot 把当前轨迹窗口写入 w（持锁写入内存缓冲，由调用方负责后续网络发送）
func (m *flightRecorderManager) snapshot(w io.Writer) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return 0, errFlightNotRunning
	}
	return m.fr.WriteTo(w)
}

func (m *flightRecorderManager) stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return errFlightNotRunning
	}
	m.fr.Stop()
	m.fr = nil
	m.running = false
	return nil
}

// flightRecover 是飞行记录器各 handler 共用的 panic 兜底
func flightRecover(w http.ResponseWriter) {
	if rec := recover(); rec != nil {
		log.Printf("[ERROR] Flight recorder handler panic recovered: %v\nStack trace:\n%s", rec, debug.Stack())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// writeFlightError 按错误类型映射状态码：状态冲突→409，Go 版本不支持→501
func writeFlightError(w http.ResponseWriter, err error) {
	status := http.StatusConflict
	if err != errFlightAlreadyRunning && err != errFlightNotRunning {
		status = http.StatusNotImplemented
	}
	http.Error(w, err.Error(), status)
}

func writeJSONResponse(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// handleFlightStart 处理 POST /api/v1/trace/flight/start
func (a *agent) handleFlightStart(w http.ResponseWriter, r *http.Request) {
	defer flightRecover(w)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := a.flightRec.start(); err != nil {
		log.Printf("[WARN] Flight recorder start failed: %v", err)
		writeFlightError(w, err)
		return
	}
	writeJSONResponse(w, map[string]string{"status": "started"})
}

// handleFlightSnapshot 处理 GET /api/v1/trace/flight/snapshot，返回二进制 trace 供 `go tool trace` 分析
func (a *agent) handleFlightSnapshot(w http.ResponseWriter, r *http.Request) {
	defer flightRecover(w)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var buf bytes.Buffer
	if _, err := a.flightRec.snapshot(&buf); err != nil {
		writeFlightError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="flight.trace"`)
	_, _ = buf.WriteTo(w)
}

// handleFlightStop 处理 POST /api/v1/trace/flight/stop
func (a *agent) handleFlightStop(w http.ResponseWriter, r *http.Request) {
	defer flightRecover(w)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := a.flightRec.stop(); err != nil {
		writeFlightError(w, err)
		return
	}
	writeJSONResponse(w, map[string]string{"status": "stopped"})
}
