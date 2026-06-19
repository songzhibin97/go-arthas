// Package arthastrace 是编译期 watch/trace 织入的运行时支撑库。
//
// 由 arthas-toolexec 在编译期注入的钩子调用本包：在被观察函数入口判断开关并
// 记录入参（Enabled/Enter），在出口记录返回值、耗时与 panic（Invocation.Exit）。
// 每个方法 id 维护一个环形缓冲，保存最近 N 次调用快照（对应 Arthas 的 tt 时间隧道）。
// 控制面（agent 包）通过 SetWatch/Methods/Records 动态开关与查询。
//
// 设计要点：watch 关闭时 Enabled 是一次 atomic load，织入点据此短路，
// 不构造参数、不记录，开销可忽略。
package arthastrace

import (
	"fmt"
	"runtime/debug"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRingSize = 64  // 每个方法保留的最近调用数（tt 深度）
	maxValueLen     = 256 // 单个参数/返回值序列化后的最大长度
)

// Arg 表示一个被捕获的参数或返回值
type Arg struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Record 一次完整调用的快照（tt 时间隧道条目）
type Record struct {
	ID       string        `json:"id"`
	Seq      uint64        `json:"seq"`
	Start    time.Time     `json:"start"`
	Duration time.Duration `json:"duration_ns"`
	Args     []Arg         `json:"args,omitempty"`
	Results  []Arg         `json:"results,omitempty"`
	Panic    string        `json:"panic,omitempty"`
	Stack    string        `json:"stack,omitempty"`
}

// Invocation 表示一次进行中的被观察调用，由 Enter 创建、Exit 结束
type Invocation struct {
	id    string
	start time.Time
	args  []Arg
}

type methodState struct {
	id      string
	enabled atomic.Bool
	calls   atomic.Uint64
	mu      sync.Mutex
	ring    []Record
	pos     int
	filled  bool
}

var (
	registry  sync.Map // id(string) → *methodState
	globalSeq atomic.Uint64
)

func getState(id string) *methodState {
	if v, ok := registry.Load(id); ok {
		return v.(*methodState)
	}
	st := &methodState{id: id, ring: make([]Record, defaultRingSize)}
	actual, _ := registry.LoadOrStore(id, st)
	return actual.(*methodState)
}

// Register 注册一个可织入方法 id（默认关闭）。由织入代码在 init 中调用，
// 使该 id 在被首次调用前即可被控制面发现与开关。
func Register(id string) { getState(id) }

// Enabled 报告某 id 的 watch 是否开启。一次 atomic load，供织入点快速短路。
func Enabled(id string) bool {
	if v, ok := registry.Load(id); ok {
		return v.(*methodState).enabled.Load()
	}
	return false
}

// SetWatch 开关某 id 的 watch
func SetWatch(id string, on bool) { getState(id).enabled.Store(on) }

// Enter 开始记录一次调用。织入点应已通过 Enabled 短路，故此处直接记录。
func Enter(id string, args []Arg) *Invocation {
	return &Invocation{id: id, start: time.Now(), args: args}
}

// Exit 完成一次调用记录。inv 为 nil 时安全空操作（watch 关闭路径）。
// recovered 非 nil 表示函数 panic（调用方负责在记录后重新 panic 以保持行为）。
func (inv *Invocation) Exit(results []Arg, recovered any) {
	if inv == nil {
		return
	}
	st := getState(inv.id)
	st.calls.Add(1)
	rec := Record{
		ID:       inv.id,
		Seq:      globalSeq.Add(1),
		Start:    inv.start,
		Duration: time.Since(inv.start),
		Args:     inv.args,
		Results:  results,
	}
	if recovered != nil {
		rec.Panic = fmt.Sprintf("%v", recovered)
		rec.Stack = string(debug.Stack())
	}
	st.add(rec)
}

func (st *methodState) add(rec Record) {
	st.mu.Lock()
	st.ring[st.pos] = rec
	st.pos = (st.pos + 1) % len(st.ring)
	if st.pos == 0 {
		st.filled = true
	}
	st.mu.Unlock()
}

// Records 返回某 id 最近的调用记录，按时间顺序（旧 → 新）
func Records(id string) []Record {
	v, ok := registry.Load(id)
	if !ok {
		return nil
	}
	st := v.(*methodState)
	st.mu.Lock()
	defer st.mu.Unlock()

	var out []Record
	if st.filled {
		out = append(out, st.ring[st.pos:]...)
		out = append(out, st.ring[:st.pos]...)
	} else {
		out = append(out, st.ring[:st.pos]...)
	}
	return out
}

// MethodInfo 某 id 的状态摘要
type MethodInfo struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Calls   uint64 `json:"calls"`
}

// Methods 列出所有已注册方法的状态，按 id 升序
func Methods() []MethodInfo {
	var out []MethodInfo
	registry.Range(func(_, v any) bool {
		st := v.(*methodState)
		out = append(out, MethodInfo{ID: st.id, Enabled: st.enabled.Load(), Calls: st.calls.Load()})
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Format 把任意值序列化为限长字符串，供织入点捕获参数/返回值。
// 对 String()/Error() 可能 panic 的值做兜底，绝不影响被观察函数。
func Format(v any) (s string) {
	defer func() {
		if r := recover(); r != nil {
			s = fmt.Sprintf("<format panic: %v>", r)
		}
	}()
	s = fmt.Sprintf("%+v", v)
	if len(s) > maxValueLen {
		s = s[:maxValueLen] + "...(truncated)"
	}
	return s
}
