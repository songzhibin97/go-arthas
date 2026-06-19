package agent

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/songzhibin97/go-arthas/arthastrace"
)

// handleTraceMethods 处理 GET /api/v1/trace/methods：
// 列出所有编译期织入并注册的可观察方法及其状态（enabled/calls）。
func (a *agent) handleTraceMethods(w http.ResponseWriter, r *http.Request) {
	defer traceRecover(w)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSONResponse(w, arthastrace.Methods())
}

// handleTraceWatch 处理 POST /api/v1/trace/methods/watch?id=X&on=true|false：
// 动态开关某方法的 watch（对应 Arthas watch 的开/关）。
func (a *agent) handleTraceWatch(w http.ResponseWriter, r *http.Request) {
	defer traceRecover(w)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id parameter", http.StatusBadRequest)
		return
	}
	on := r.URL.Query().Get("on") != "false" // 缺省为开启
	arthastrace.SetWatch(id, on)
	writeJSONResponse(w, map[string]interface{}{"id": id, "enabled": on})
}

// handleTraceRecords 处理 GET /api/v1/trace/methods/records?id=X：
// 返回某方法最近的调用记录（入参/返回值/耗时/panic），对应 Arthas 的 tt 时间隧道。
func (a *agent) handleTraceRecords(w http.ResponseWriter, r *http.Request) {
	defer traceRecover(w)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id parameter", http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, arthastrace.Records(id))
}

func traceRecover(w http.ResponseWriter) {
	if rec := recover(); rec != nil {
		log.Printf("[ERROR] Trace handler panic recovered: %v\nStack trace:\n%s", rec, debug.Stack())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
