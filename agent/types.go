package agent

import (
	"sync"
	"time"
)

// Config 定义 Agent 的配置选项
type Config struct {
	Port          int    // HTTP 服务器端口，默认 8563
	EnablePprof   bool   // 是否启用 pprof 端点，默认 true
	EnableMetrics bool   // 是否启用指标收集，默认 true
	LogLevel      string // 日志级别：debug, info, warn, error
}

// Metrics 包含所有运行时指标的快照
type Metrics struct {
	Timestamp  time.Time     `json:"timestamp"`  // 采集时间戳
	Goroutines int           `json:"goroutines"` // 当前 goroutine 数量
	Memory     MemoryMetrics `json:"memory"`     // 内存指标
	CPU        CPUMetrics    `json:"cpu"`        // CPU 指标
	GC         GCMetrics     `json:"gc"`         // GC 指标
}

// MemoryMetrics 内存相关指标
type MemoryMetrics struct {
	HeapAlloc    uint64 `json:"heap_alloc"`    // 堆已分配字节数
	HeapInuse    uint64 `json:"heap_inuse"`    // 堆正在使用字节数
	HeapIdle     uint64 `json:"heap_idle"`     // 堆空闲字节数
	HeapReleased uint64 `json:"heap_released"` // 已释放给 OS 的字节数
	StackInuse   uint64 `json:"stack_inuse"`   // 栈正在使用字节数
	TotalAlloc   uint64 `json:"total_alloc"`   // 累计分配字节数
	Sys          uint64 `json:"sys"`           // 从 OS 获取的总字节数
}

// CPUMetrics CPU 相关指标
type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"` // CPU 使用率百分比
}

// GCMetrics GC 相关指标
type GCMetrics struct {
	NumGC      uint32        `json:"num_gc"`      // GC 执行次数
	PauseTotal time.Duration `json:"pause_total"` // GC 总暂停时间
	LastPause  time.Duration `json:"last_pause"`  // 最后一次 GC 暂停时间
	PauseAvg   time.Duration `json:"pause_avg"`   // 平均 GC 暂停时间
}

// SystemInfo 系统信息
type SystemInfo struct {
	GoVersion string    `json:"go_version"` // Go 版本
	GOOS      string    `json:"goos"`       // 操作系统
	GOARCH    string    `json:"goarch"`     // 架构
	NumCPU    int       `json:"num_cpu"`    // CPU 核心数
	ProcessID int       `json:"process_id"` // 进程 ID
	StartTime time.Time `json:"start_time"` // 启动时间
	Uptime    string    `json:"uptime"`     // 运行时长
}

// safeMetrics 线程安全的指标存储
type safeMetrics struct {
	mu      sync.RWMutex
	current *Metrics
}

// Get 获取当前指标的副本
func (sm *safeMetrics) Get() *Metrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.current == nil {
		return nil
	}
	// 返回深拷贝以避免并发修改
	return sm.current.Clone()
}

// Set 设置新的指标
func (sm *safeMetrics) Set(metrics *Metrics) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.current = metrics
}

// Clone 创建 Metrics 的深拷贝
func (m *Metrics) Clone() *Metrics {
	if m == nil {
		return nil
	}
	return &Metrics{
		Timestamp:  m.Timestamp,
		Goroutines: m.Goroutines,
		Memory:     m.Memory,
		CPU:        m.CPU,
		GC:         m.GC,
	}
}
