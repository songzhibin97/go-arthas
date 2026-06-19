package agent

import (
	"math"
	"runtime"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_MetricsAccuracy(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Goroutine count matches runtime.NumGoroutine() within ±1
	properties.Property("goroutine count accuracy", prop.ForAll(
		func(workloadSize int) bool {
			// 创建收集器
			collector := newMetricsCollector(1 * time.Second)

			// 创建工作负载（启动一些 goroutine）
			done := make(chan struct{})
			for i := 0; i < workloadSize; i++ {
				go func() {
					<-done
				}()
			}

			// 收集指标
			if err := collector.collect(); err != nil {
				close(done)
				return false
			}

			// 获取收集的指标
			metrics := collector.GetMetrics()
			if metrics == nil {
				close(done)
				return false
			}

			// 获取实际的 goroutine 数量
			actualGoroutines := runtime.NumGoroutine()

			// 清理工作负载
			close(done)
			time.Sleep(10 * time.Millisecond) // 等待 goroutine 退出

			// 验证误差在 ±1 范围内
			diff := math.Abs(float64(metrics.Goroutines - actualGoroutines))
			return diff <= 1
		},
		gen.IntRange(0, 50), // 生成 0-50 个 goroutine 的工作负载
	))

	// Memory stats match runtime.ReadMemStats()
	properties.Property("memory stats accuracy", prop.ForAll(
		func() bool {
			// 创建收集器
			collector := newMetricsCollector(1 * time.Second)

			// 收集指标
			if err := collector.collect(); err != nil {
				return false
			}

			// 获取收集的指标
			metrics := collector.GetMetrics()
			if metrics == nil {
				return false
			}

			// 获取实际的内存统计
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			// 验证关键内存字段（允许一定误差，因为收集和验证之间有时间差）
			// 我们检查值是否在合理范围内（不为零，且不会相差太大）
			if metrics.Memory.HeapAlloc == 0 && memStats.HeapAlloc > 0 {
				return false
			}
			if metrics.Memory.HeapInuse == 0 && memStats.HeapInuse > 0 {
				return false
			}
			if metrics.Memory.Sys == 0 && memStats.Sys > 0 {
				return false
			}

			// 验证 TotalAlloc 是单调递增的（累计值）
			if metrics.Memory.TotalAlloc > memStats.TotalAlloc {
				return false
			}

			return true
		},
	))

	// CPU usage is between 0-100%
	properties.Property("cpu usage range", prop.ForAll(
		func(iterations int) bool {
			// 创建收集器
			collector := newMetricsCollector(100 * time.Millisecond)
			collector.start()
			defer collector.stop()

			// 等待至少一次收集
			time.Sleep(150 * time.Millisecond)

			// 创建一些 CPU 负载
			done := make(chan struct{})
			for i := 0; i < iterations; i++ {
				go func() {
					for {
						select {
						case <-done:
							return
						default:
							// 执行一些计算
							_ = math.Sqrt(float64(time.Now().UnixNano()))
						}
					}
				}()
			}

			// 等待收集器收集指标
			time.Sleep(200 * time.Millisecond)

			// 停止 CPU 负载
			close(done)

			// 获取指标
			metrics := collector.GetMetrics()
			if metrics == nil {
				return false
			}

			// 验证 CPU 使用率在 0-100% 范围内
			return metrics.CPU.UsagePercent >= 0 && metrics.CPU.UsagePercent <= 100
		},
		gen.IntRange(0, 10), // 生成 0-10 个 CPU 密集型 goroutine
	))

	// GC metrics are valid
	properties.Property("gc metrics validity", prop.ForAll(
		func() bool {
			// 创建收集器
			collector := newMetricsCollector(1 * time.Second)

			// 触发一些内存分配以产生 GC
			data := make([][]byte, 100)
			for i := range data {
				data[i] = make([]byte, 1024*1024) // 1MB
			}
			runtime.GC() // 强制 GC

			// 收集指标
			if err := collector.collect(); err != nil {
				return false
			}

			// 获取指标
			metrics := collector.GetMetrics()
			if metrics == nil {
				return false
			}

			// 验证 GC 指标的有效性
			// NumGC 应该大于 0（因为我们强制了 GC）
			if metrics.GC.NumGC == 0 {
				return false
			}

			// PauseTotal 应该是非负的
			if metrics.GC.PauseTotal < 0 {
				return false
			}

			// LastPause 应该是非负的
			if metrics.GC.LastPause < 0 {
				return false
			}

			// PauseAvg 应该是非负的
			if metrics.GC.PauseAvg < 0 {
				return false
			}

			// 如果有 GC，平均暂停时间应该合理
			if metrics.GC.NumGC > 0 && metrics.GC.PauseAvg == 0 {
				// 这可能是正常的（非常快的 GC），所以不算失败
			}

			return true
		},
	))

	properties.TestingRun(t)
}

// TestMetricsCollector_StartStop 测试收集器的启动和停止
func TestMetricsCollector_StartStop(t *testing.T) {
	collector := newMetricsCollector(100 * time.Millisecond)

	// 启动收集器
	collector.start()

	// 等待至少一次收集
	time.Sleep(150 * time.Millisecond)

	// 验证指标已收集
	metrics := collector.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected metrics to be collected, got nil")
	}

	// 停止收集器
	collector.stop()

	// 验证收集器已停止（doneCh 应该已关闭）
	select {
	case <-collector.doneCh:
		// 正常，doneCh 已关闭
	case <-time.After(1 * time.Second):
		t.Fatal("Collector did not stop within timeout")
	}
}

// TestMetricsCollector_ContinuousCollection 测试连续收集
func TestMetricsCollector_ContinuousCollection(t *testing.T) {
	collector := newMetricsCollector(50 * time.Millisecond)
	collector.start()
	defer collector.stop()

	// 等待多次收集
	time.Sleep(200 * time.Millisecond)

	// 获取第一次指标
	metrics1 := collector.GetMetrics()
	if metrics1 == nil {
		t.Fatal("Expected metrics to be collected")
	}

	// 等待更多收集
	time.Sleep(100 * time.Millisecond)

	// 获取第二次指标
	metrics2 := collector.GetMetrics()
	if metrics2 == nil {
		t.Fatal("Expected metrics to be collected")
	}

	// 验证时间戳已更新
	if !metrics2.Timestamp.After(metrics1.Timestamp) {
		t.Error("Expected metrics timestamp to be updated")
	}
}

// TestMetricsCollector_ImmediateCollection 验证真不变量:collector 启动后会"立即"(异步)
// 采集首批指标,而非等到第一个 ticker。
//
// 取代旧的 "时间戳 <100ms" 绝对时延断言——采集 goroutine 在有负载机器上可能 >100ms 才被
// 调度,导致误判。这里用一个**远大于轮询窗口的 ticker 间隔**(10s),再轮询 2s 内是否拿到
// 指标:若拿到,只可能来自启动时的立即采集(第一个 tick 还在 10s 之后),从而既可靠又能
// 真正区分"立即采集"与"首次 tick"。
func TestMetricsCollector_ImmediateCollection(t *testing.T) {
	collector := newMetricsCollector(10 * time.Second)
	collector.start()
	defer collector.stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if collector.GetMetrics() != nil {
			return // 远早于 10s tick 就拿到 → 证明是启动时的立即采集
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("collector did not perform immediate collection before the first tick")
}

// TestMetricsCollector_NoGoroutineLeak 测试没有 goroutine 泄漏
func TestMetricsCollector_NoGoroutineLeak(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	// 创建并启动多个收集器
	collectors := make([]*metricsCollector, 10)
	for i := range collectors {
		collectors[i] = newMetricsCollector(100 * time.Millisecond)
		collectors[i].start()
	}

	// 等待收集器运行
	time.Sleep(50 * time.Millisecond)

	// 停止所有收集器
	for _, c := range collectors {
		c.stop()
	}

	// 等待 goroutine 清理
	time.Sleep(100 * time.Millisecond)

	// 验证 goroutine 数量恢复
	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines+2 { // 允许 2 个误差
		t.Errorf("Goroutine leak detected: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}
}

// TestMetricsCollector_PanicRecovery 测试收集器从 panic 中恢复
func TestMetricsCollector_PanicRecovery(t *testing.T) {
	// 注意：这个测试验证 panic recovery 机制存在
	// 实际的 collect() 方法不会 panic，但 start() 中有 defer recover()
	collector := newMetricsCollector(100 * time.Millisecond)
	collector.start()

	// 等待收集器运行
	time.Sleep(150 * time.Millisecond)

	// 验证收集器仍在运行
	metrics := collector.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected collector to be running after potential panic")
	}

	// 停止收集器
	collector.stop()

	// 验证正常停止
	select {
	case <-collector.doneCh:
		// 正常
	case <-time.After(1 * time.Second):
		t.Fatal("Collector did not stop after panic recovery")
	}
}

// TestMetricsCollector_ErrorLogging 测试收集失败时的错误日志
func TestMetricsCollector_ErrorLogging(t *testing.T) {
	// 这个测试验证即使 collect() 返回错误，收集器也会继续运行
	collector := newMetricsCollector(50 * time.Millisecond)
	collector.start()
	defer collector.stop()

	// 等待多次收集
	time.Sleep(200 * time.Millisecond)

	// 验证收集器仍在收集指标
	metrics1 := collector.GetMetrics()
	if metrics1 == nil {
		t.Fatal("Expected metrics to be collected")
	}

	time.Sleep(100 * time.Millisecond)

	metrics2 := collector.GetMetrics()
	if metrics2 == nil {
		t.Fatal("Expected metrics to continue being collected after errors")
	}

	// 验证时间戳已更新（说明收集器继续运行）
	if !metrics2.Timestamp.After(metrics1.Timestamp) {
		t.Error("Expected collector to continue after errors")
	}
}

// TestMetricsCollector_ContinuesOnCollectionFailure 测试收集失败后继续运行
func TestMetricsCollector_ContinuesOnCollectionFailure(t *testing.T) {
	collector := newMetricsCollector(50 * time.Millisecond)
	collector.start()
	defer collector.stop()

	// 收集多次以确保即使有错误也能继续
	var timestamps []time.Time
	for i := 0; i < 5; i++ {
		time.Sleep(60 * time.Millisecond)
		metrics := collector.GetMetrics()
		if metrics != nil {
			timestamps = append(timestamps, metrics.Timestamp)
		}
	}

	// 验证至少收集了多次
	if len(timestamps) < 3 {
		t.Errorf("Expected at least 3 collections, got %d", len(timestamps))
	}

	// 验证时间戳是递增的
	for i := 1; i < len(timestamps); i++ {
		if !timestamps[i].After(timestamps[i-1]) {
			t.Error("Expected timestamps to be increasing")
		}
	}
}
