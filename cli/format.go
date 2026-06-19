package cli

import (
	"fmt"
	"sort"
	"strings"
)

// FormatMetrics 格式化并打印运行时指标
func FormatMetrics(m *Metrics) {
	fmt.Println("=== Runtime Metrics ===")
	fmt.Println()

	// 时间戳
	fmt.Printf("Timestamp: %s\n", m.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Goroutines
	fmt.Println("Goroutines:")
	fmt.Printf("  Count: %d\n", m.Goroutines)
	fmt.Println()

	// CPU
	fmt.Println("CPU:")
	fmt.Printf("  Usage: %.2f%%\n", m.CPU.UsagePercent)
	fmt.Println()

	// Memory
	fmt.Println("Memory:")
	fmt.Printf("  Heap Allocated:  %s\n", formatBytes(m.Memory.HeapAlloc))
	fmt.Printf("  Heap In-Use:     %s\n", formatBytes(m.Memory.HeapInuse))
	fmt.Printf("  Heap Idle:       %s\n", formatBytes(m.Memory.HeapIdle))
	fmt.Printf("  Heap Released:   %s\n", formatBytes(m.Memory.HeapReleased))
	fmt.Printf("  Stack In-Use:    %s\n", formatBytes(m.Memory.StackInuse))
	fmt.Printf("  Total Allocated: %s\n", formatBytes(m.Memory.TotalAlloc))
	fmt.Printf("  System Memory:   %s\n", formatBytes(m.Memory.Sys))
	fmt.Println()

	// GC
	fmt.Println("Garbage Collection:")
	fmt.Printf("  GC Count:        %d\n", m.GC.NumGC)
	fmt.Printf("  Total Pause:     %v\n", m.GC.PauseTotal)
	fmt.Printf("  Last Pause:      %v\n", m.GC.LastPause)
	fmt.Printf("  Average Pause:   %v\n", m.GC.PauseAvg)
	fmt.Println()
}

// FormatSystemInfo 格式化并打印系统信息
func FormatSystemInfo(info *SystemInfo) {
	fmt.Println("=== System Information ===")
	fmt.Println()

	// 创建表格数据
	rows := [][]string{
		{"Go Version", info.GoVersion},
		{"Operating System", info.GOOS},
		{"Architecture", info.GOARCH},
		{"CPU Cores", fmt.Sprintf("%d", info.NumCPU)},
		{"Process ID", fmt.Sprintf("%d", info.ProcessID)},
		{"Start Time", info.StartTime.Format("2006-01-02 15:04:05")},
		{"Uptime", info.Uptime},
	}

	// 计算最大列宽
	maxKeyLen := 0
	for _, row := range rows {
		if len(row[0]) > maxKeyLen {
			maxKeyLen = len(row[0])
		}
	}

	// 打印表格
	for _, row := range rows {
		fmt.Printf("  %-*s: %s\n", maxKeyLen, row[0], row[1])
	}
	fmt.Println()
}

// FormatBytesSize 格式化字节数为人类可读格式
func FormatBytesSize(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatBytes 格式化字节数为人类可读格式（内部使用）
func formatBytes(bytes uint64) string {
	return FormatBytesSize(bytes)
}

// FormatGoroutineDump 格式化并打印 goroutine 转储（Arthas thread 等价）
func FormatGoroutineDump(d *GoroutineDump, showStacks bool) {
	fmt.Println("=== Goroutine Dump ===")
	fmt.Println()
	fmt.Printf("Timestamp: %s\n", d.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Total goroutines: %d\n", d.Total)
	fmt.Println()

	// 按计数降序打印状态聚合
	fmt.Println("By state:")
	type stateCount struct {
		state string
		count int
	}
	states := make([]stateCount, 0, len(d.StateCounts))
	for s, c := range d.StateCounts {
		states = append(states, stateCount{s, c})
	}
	sort.Slice(states, func(i, j int) bool {
		if states[i].count != states[j].count {
			return states[i].count > states[j].count
		}
		return states[i].state < states[j].state
	})
	for _, s := range states {
		fmt.Printf("  %-28s %d\n", s.state, s.count)
	}
	fmt.Println()

	// 疑似阻塞
	if len(d.Suspected) > 0 {
		fmt.Printf("[!] Suspected blocked goroutines (%d):\n", len(d.Suspected))
		for _, g := range d.Suspected {
			fmt.Printf("  goroutine %d [%s, %d minutes]\n", g.ID, g.State, g.WaitMinutes)
			if g.Stack != "" {
				for _, line := range strings.Split(g.Stack, "\n") {
					fmt.Printf("    %s\n", line)
				}
			}
		}
		fmt.Println()
	} else {
		fmt.Println("No suspected blocked goroutines.")
		fmt.Println()
	}

	// 全部栈（可选）
	if showStacks && len(d.Goroutines) > 0 {
		fmt.Println("All goroutines:")
		for _, g := range d.Goroutines {
			if g.Stack != "" {
				fmt.Println(g.Stack)
				fmt.Println()
			}
		}
	}
}

// FormatMethods 格式化并打印可观察方法列表
func FormatMethods(ms []MethodInfo) {
	fmt.Println("=== Watched Methods ===")
	if len(ms) == 0 {
		fmt.Println("(none registered; build target with `go-arthas build --targets ...`)")
		return
	}
	for _, m := range ms {
		state := "off"
		if m.Enabled {
			state = "ON"
		}
		fmt.Printf("  [%-3s] %-50s calls=%d\n", state, m.ID, m.Calls)
	}
}

// FormatRecords 格式化并打印某方法的调用记录（tt）
func FormatRecords(id string, recs []TraceRecord) {
	fmt.Printf("=== Records: %s (%d) ===\n", id, len(recs))
	for _, r := range recs {
		fmt.Printf("#%d  %s  dur=%v\n", r.Seq, r.Start.Format("15:04:05.000"), r.Duration)
		if len(r.Args) > 0 {
			fmt.Printf("    args:    ")
			for _, a := range r.Args {
				fmt.Printf("%s=%s ", a.Name, a.Value)
			}
			fmt.Println()
		}
		if len(r.Results) > 0 {
			fmt.Printf("    returns: ")
			for _, a := range r.Results {
				fmt.Printf("%s=%s ", a.Name, a.Value)
			}
			fmt.Println()
		}
		if r.Panic != "" {
			fmt.Printf("    panic:   %s\n", r.Panic)
		}
	}
}
