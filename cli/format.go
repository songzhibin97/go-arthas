package cli

import (
	"fmt"
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

// printTable 打印表格（通用函数）
func printTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	// 计算每列的最大宽度
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// 打印表头
	printRow(headers, colWidths)
	printSeparator(colWidths)

	// 打印数据行
	for _, row := range rows {
		printRow(row, colWidths)
	}
}

// printRow 打印表格行
func printRow(cells []string, widths []int) {
	for i, cell := range cells {
		if i < len(widths) {
			fmt.Printf("%-*s", widths[i]+2, cell)
		}
	}
	fmt.Println()
}

// printSeparator 打印表格分隔线
func printSeparator(widths []int) {
	for _, width := range widths {
		fmt.Print(strings.Repeat("-", width+2))
	}
	fmt.Println()
}
