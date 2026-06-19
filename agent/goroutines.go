package agent

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GoroutineInfo 描述单个 goroutine
type GoroutineInfo struct {
	ID          int    `json:"id"`                     // goroutine 编号
	State       string `json:"state"`                  // 规范化状态，如 running / chan receive / select
	WaitMinutes int    `json:"wait_minutes,omitempty"` // 阻塞时长（分钟）；runtime 仅对 >=1 分钟的阻塞报告
	Stack       string `json:"stack,omitempty"`        // 完整调用栈（按需）
}

// GoroutineDump 是某一时刻所有 goroutine 的转储与聚合，对应 Arthas 的 thread 命令
type GoroutineDump struct {
	Timestamp   time.Time       `json:"timestamp"`                   // 抓取时间
	Total       int             `json:"total"`                       // goroutine 总数
	StateCounts map[string]int  `json:"state_counts"`                // 按状态聚合的计数
	Suspected   []GoroutineInfo `json:"suspected_blocked,omitempty"` // 疑似长阻塞（runtime 标注 >=N 分钟，始终带栈）
	Goroutines  []GoroutineInfo `json:"goroutines,omitempty"`        // 全部 goroutine（仅当请求 stacks 时填充）
}

// fullStackTrace 返回所有 goroutine 的完整栈文本，自动扩容缓冲区直到容纳全部内容
func fullStackTrace() []byte {
	n := 1 << 20 // 1MB 起步
	for {
		buf := make([]byte, n)
		m := runtime.Stack(buf, true)
		if m < n {
			return buf[:m]
		}
		n *= 2
	}
}

// captureGoroutineDump 抓取并解析当前所有 goroutine。
// suspectMinWait：阻塞达到该分钟数即视为疑似（runtime 仅对 >=1 分钟的阻塞报告时长）；
// 传 0 表示不做疑似判定。
func captureGoroutineDump(includeStacks bool, suspectMinWait int) *GoroutineDump {
	return parseGoroutineDump(fullStackTrace(), includeStacks, suspectMinWait, time.Now())
}

// parseGoroutineDump 解析 runtime.Stack 的文本输出，与具体 runtime 调用解耦以便测试
func parseGoroutineDump(raw []byte, includeStacks bool, suspectMinWait int, now time.Time) *GoroutineDump {
	dump := &GoroutineDump{
		Timestamp:   now,
		StateCounts: map[string]int{},
	}

	for _, blk := range splitGoroutineBlocks(string(raw)) {
		gi, ok := parseGoroutineHeader(blk)
		if !ok {
			continue
		}
		dump.Total++
		dump.StateCounts[gi.State]++

		// 疑似长阻塞：runtime 报告了 >=1 分钟的等待时长。这通常指示死锁、
		// 泄漏或卡住的 goroutine（正常的长驻监听也可能出现，列为"疑似"供人工研判）。
		//
		// 局限（务必知晓）：本启发式只识别「长阻塞」，不做等待环（互相等待）分析。
		// 且 Go runtime 仅对阻塞 >=60s 的 goroutine 标注分钟数（traceback：
		// waitfor = (now-waitsince)/60e9，>=1 才打印），所以**刚发生的死锁在满
		// 60 秒前不会出现在这里**。需要秒级或环检测时应配合 GODEBUG 调度跟踪/
		// 多次采样比对，而非依赖本字段。
		if suspectMinWait > 0 && gi.WaitMinutes >= suspectMinWait {
			si := gi
			si.Stack = strings.TrimRight(blk, "\n") // 疑似项始终带栈以便定位
			dump.Suspected = append(dump.Suspected, si)
		}

		if includeStacks {
			gi.Stack = strings.TrimRight(blk, "\n")
			dump.Goroutines = append(dump.Goroutines, gi)
		}
	}

	// 疑似项按阻塞时长降序，最久的在前
	sort.SliceStable(dump.Suspected, func(i, j int) bool {
		return dump.Suspected[i].WaitMinutes > dump.Suspected[j].WaitMinutes
	})

	return dump
}

// splitGoroutineBlocks 以 "goroutine " 行为边界把全栈文本切分为单个 goroutine 块
func splitGoroutineBlocks(s string) []string {
	var blocks []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, ln := range strings.Split(s, "\n") {
		if strings.HasPrefix(ln, "goroutine ") {
			flush()
		}
		cur = append(cur, ln)
	}
	flush()
	return blocks
}

// parseGoroutineHeader 解析形如 "goroutine 123 [chan receive, 5 minutes]:" 的首行
func parseGoroutineHeader(block string) (GoroutineInfo, bool) {
	header := block
	if nl := strings.IndexByte(block, '\n'); nl >= 0 {
		header = block[:nl]
	}
	if !strings.HasPrefix(header, "goroutine ") {
		return GoroutineInfo{}, false
	}
	open := strings.IndexByte(header, '[')
	closeIdx := strings.LastIndexByte(header, ']')
	if open < 0 || closeIdx < 0 || closeIdx < open {
		return GoroutineInfo{}, false
	}
	id, err := strconv.Atoi(strings.TrimSpace(header[len("goroutine "):open]))
	if err != nil {
		return GoroutineInfo{}, false
	}

	gi := GoroutineInfo{ID: id}
	inside := header[open+1 : closeIdx]
	if comma := strings.IndexByte(inside, ','); comma >= 0 {
		gi.State = strings.TrimSpace(inside[:comma])
		gi.WaitMinutes = parseWaitMinutes(strings.TrimSpace(inside[comma+1:]))
	} else {
		gi.State = strings.TrimSpace(inside)
	}
	return gi, true
}

// parseWaitMinutes 从 "5 minutes" / "1 minute" 解析出分钟数
func parseWaitMinutes(s string) int {
	fields := strings.Fields(s)
	if len(fields) >= 2 && strings.HasPrefix(fields[1], "minute") {
		if n, err := strconv.Atoi(fields[0]); err == nil {
			return n
		}
	}
	return 0
}

// handleGoroutines 处理 /api/v1/goroutines 请求（Arthas thread 等价）
// 查询参数：
//
//	format=text   返回 runtime.Stack 原始全栈文本
//	stacks=true   JSON 响应中包含每个 goroutine 的完整栈
//	min_wait=N    阻塞 >=N 分钟视为疑似（默认 1；0 表示不判定）
func (a *agent) handleGoroutines(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[ERROR] Goroutines handler panic recovered: %v\nStack trace:\n%s", rec, debug.Stack())
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}()

	if r.Method != http.MethodGet {
		log.Printf("[WARN] Invalid method for /api/v1/goroutines: %s from %s", r.Method, r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Query().Get("format") == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(fullStackTrace())
		return
	}

	includeStacks := r.URL.Query().Get("stacks") == "true"
	minWait := 1
	if v := r.URL.Query().Get("min_wait"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			minWait = n
		}
	}

	dump := captureGoroutineDump(includeStacks, minWait)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dump); err != nil {
		log.Printf("[ERROR] Failed to encode goroutine dump for %s: %v", r.RemoteAddr, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
