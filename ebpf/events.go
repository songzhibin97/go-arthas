package ebpf

// EventKind 区分函数进入与返回
type EventKind uint8

const (
	EventEnter EventKind = iota
	EventExit
)

func (k EventKind) String() string {
	if k == EventExit {
		return "exit"
	}
	return "enter"
}

// Event 一次被 eBPF uprobe 捕获的函数进入或返回。
// Regs 是按 Go ABIInternal 顺序读取的整数寄存器原始值：
// 进入时为参数寄存器，返回时（uprobe-on-RET 处）为返回值寄存器。
// 复合类型（结构体/接口/字符串）需要在用户态结合目标二进制类型信息进一步解释；
// MVP 先暴露原始寄存器值。
type Event struct {
	Kind    EventKind `json:"kind"`
	Func    string    `json:"func"`
	PID     uint32    `json:"pid"`
	TID     uint32    `json:"tid"`
	KTimeNs uint64    `json:"ktime_ns"`
	Regs    []uint64  `json:"regs"`
}

// AttachOptions 配置一次 eBPF attach
type AttachOptions struct {
	BinaryPath  string        // 目标可执行文件路径（通常 /proc/<pid>/exe）
	PID         int           // 目标进程 PID（>0 时仅观察该进程）
	Targets     []*FuncTarget // 要观察的函数（含相对入口的 RET 偏移）
	RegisterABI bool          // 目标是否使用寄存器调用约定（决定寄存器读取语义）
}

// Attacher 表示一组已挂载的 eBPF uprobe 及其事件流
type Attacher interface {
	// Events 返回捕获事件的只读通道；Close 后关闭
	Events() <-chan Event
	// Close 卸载所有 uprobe 并释放资源
	Close() error
}
