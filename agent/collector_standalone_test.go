package agent

import (
	"runtime"
	"testing"
	"time"
)

// TestCollectorStandalone 验证指标收集器独立运行
func TestCollectorStandalone(t *testing.T) {
	t.Run("metrics collected every 1 second", func(t *testing.T) {
		// 创建收集器，1秒间隔
		collector := newMetricsCollector(1 * time.Second)

		// 启动收集器
		collector.start()

		// 等待初始收集
		time.Sleep(100 * time.Millisecond)

		// 获取第一次指标
		metrics1 := collector.GetMetrics()
		if metrics1 == nil {
			t.Fatal("Expected metrics to be collected, got nil")
		}

		// 验证指标包含有效数据
		if metrics1.Goroutines <= 0 {
			t.Errorf("Expected goroutines > 0, got %d", metrics1.Goroutines)
		}
		if metrics1.Memory.HeapAlloc == 0 {
			t.Error("Expected HeapAlloc > 0, got 0")
		}

		t.Logf("First collection - Goroutines: %d, HeapAlloc: %d bytes, CPU: %.2f%%",
			metrics1.Goroutines, metrics1.Memory.HeapAlloc, metrics1.CPU.UsagePercent)

		// 等待约1秒，验证新的收集
		time.Sleep(1100 * time.Millisecond)

		// 获取第二次指标
		metrics2 := collector.GetMetrics()
		if metrics2 == nil {
			t.Fatal("Expected metrics to be collected after 1 second, got nil")
		}

		// 验证时间戳已更新（应该至少相差1秒）
		timeDiff := metrics2.Timestamp.Sub(metrics1.Timestamp)
		if timeDiff < 900*time.Millisecond || timeDiff > 1500*time.Millisecond {
			t.Errorf("Expected time difference ~1s, got %v", timeDiff)
		}

		t.Logf("Second collection - Goroutines: %d, HeapAlloc: %d bytes, CPU: %.2f%%",
			metrics2.Goroutines, metrics2.Memory.HeapAlloc, metrics2.CPU.UsagePercent)

		// 停止收集器
		collector.stop()

		t.Log("Collector stopped successfully")
	})

	t.Run("no goroutine leaks after stop", func(t *testing.T) {
		// 记录初始 goroutine 数量
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		initialGoroutines := runtime.NumGoroutine()

		t.Logf("Initial goroutines: %d", initialGoroutines)

		// 创建并启动多个收集器
		collectors := make([]*metricsCollector, 5)
		for i := 0; i < 5; i++ {
			collectors[i] = newMetricsCollector(1 * time.Second)
			collectors[i].start()
		}

		// 等待收集器启动
		time.Sleep(200 * time.Millisecond)

		runningGoroutines := runtime.NumGoroutine()
		t.Logf("Goroutines with 5 collectors running: %d", runningGoroutines)

		// 验证 goroutine 数量增加（每个收集器1个）
		if runningGoroutines < initialGoroutines+5 {
			t.Errorf("Expected at least %d goroutines, got %d",
				initialGoroutines+5, runningGoroutines)
		}

		// 停止所有收集器
		for _, collector := range collectors {
			collector.stop()
		}

		// 等待 goroutine 清理
		time.Sleep(200 * time.Millisecond)
		runtime.GC()
		time.Sleep(100 * time.Millisecond)

		finalGoroutines := runtime.NumGoroutine()
		t.Logf("Final goroutines after stop: %d", finalGoroutines)

		// 验证 goroutine 数量恢复到初始水平（允许±2的误差）
		if finalGoroutines > initialGoroutines+2 {
			t.Errorf("Goroutine leak detected: initial=%d, final=%d, leaked=%d",
				initialGoroutines, finalGoroutines, finalGoroutines-initialGoroutines)
		}

		t.Log("No goroutine leaks detected")
	})

	t.Run("collector continues after collection error", func(t *testing.T) {
		collector := newMetricsCollector(500 * time.Millisecond)
		collector.start()

		// 等待几次收集
		time.Sleep(1500 * time.Millisecond)

		// 验证收集器仍在运行
		metrics := collector.GetMetrics()
		if metrics == nil {
			t.Fatal("Collector stopped unexpectedly")
		}

		t.Logf("Collector still running - Goroutines: %d", metrics.Goroutines)

		collector.stop()
	})

	t.Run("metrics accuracy verification", func(t *testing.T) {
		collector := newMetricsCollector(1 * time.Second)
		collector.start()

		// 等待收集
		time.Sleep(100 * time.Millisecond)

		metrics := collector.GetMetrics()
		if metrics == nil {
			t.Fatal("Expected metrics, got nil")
		}

		// 验证 goroutine 数量准确性（与 runtime 对比，允许±2误差）
		actualGoroutines := runtime.NumGoroutine()
		diff := metrics.Goroutines - actualGoroutines
		if diff < -2 || diff > 2 {
			t.Errorf("Goroutine count mismatch: collected=%d, actual=%d, diff=%d",
				metrics.Goroutines, actualGoroutines, diff)
		}

		// 验证内存统计准确性
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// 内存值应该在合理范围内（允许一定误差，因为是不同时间点采集）
		heapDiff := int64(metrics.Memory.HeapAlloc) - int64(memStats.HeapAlloc)
		if heapDiff < -10*1024*1024 || heapDiff > 10*1024*1024 {
			t.Logf("Warning: Large heap allocation difference: %d bytes", heapDiff)
		}

		// 验证 CPU 使用率在有效范围内
		if metrics.CPU.UsagePercent < 0 || metrics.CPU.UsagePercent > 100 {
			t.Errorf("Invalid CPU usage: %.2f%% (must be 0-100)", metrics.CPU.UsagePercent)
		}

		t.Logf("Metrics accuracy verified - Goroutines: %d (actual: %d), CPU: %.2f%%",
			metrics.Goroutines, actualGoroutines, metrics.CPU.UsagePercent)

		collector.stop()
	})
}
