package agent

import (
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// metricsCollector 负责定期收集运行时指标
type metricsCollector struct {
	interval         time.Duration    // 收集间隔
	metrics          *safeMetrics     // 线程安全的指标存储
	stopCh           chan struct{}    // 停止信号通道
	doneCh           chan struct{}    // 完成信号通道
	lastCPU          cpuStats         // 上次 CPU 统计
	stopped          bool             // 是否已停止
	mu               sync.Mutex       // 保护 stopped 字段
	wsManager        *wsManager       // WebSocket 管理器（可选）
	cpuStatsWarned   bool             // CPU 统计不可用警告是否已发出
	platformFeatures platformFeatures // 平台特性检测结果
}

// platformFeatures 记录平台特性的可用性
type platformFeatures struct {
	cpuStatsAvailable bool // CPU 统计是否可用
	checkedOnce       bool // 是否已检测过
	mu                sync.RWMutex
}

// cpuStats 存储 CPU 时间统计
type cpuStats struct {
	userTime   int64
	systemTime int64
	timestamp  time.Time
}

// newMetricsCollector 创建新的指标收集器
func newMetricsCollector(interval time.Duration) *metricsCollector {
	return &metricsCollector{
		interval: interval,
		metrics:  &safeMetrics{},
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// start 启动指标收集循环
func (mc *metricsCollector) start() {
	go func() {
		defer close(mc.doneCh)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Metrics collector panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
			}
		}()

		// 初始化 CPU 统计
		mc.lastCPU = mc.getCPUStats()

		ticker := time.NewTicker(mc.interval)
		defer ticker.Stop()

		// 立即收集一次
		if err := mc.collect(); err != nil {
			log.Printf("[WARN] Initial metrics collection failed: %v", err)
		}

		for {
			select {
			case <-ticker.C:
				if err := mc.collect(); err != nil {
					log.Printf("[WARN] Metrics collection failed: %v", err)
				}
			case <-mc.stopCh:
				return
			}
		}
	}()
}

// stop 停止指标收集
func (mc *metricsCollector) stop() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.stopped {
		return // 已经停止，直接返回
	}

	close(mc.stopCh)
	<-mc.doneCh
	mc.stopped = true
}

// collect 执行一次指标收集
func (mc *metricsCollector) collect() error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] Metrics collection panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
		}
	}()

	now := time.Now()

	// 收集 goroutine 数量
	goroutines := runtime.NumGoroutine()
	if goroutines < 0 {
		log.Printf("[WARN] Invalid goroutine count: %d, using 0", goroutines)
		goroutines = 0
	}

	// 收集内存统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 收集 CPU 使用率
	cpuUsage := mc.calculateCPUUsage()
	if cpuUsage < 0 || cpuUsage > 100 {
		log.Printf("[WARN] Invalid CPU usage: %.2f%%, clamping to valid range", cpuUsage)
		if cpuUsage < 0 {
			cpuUsage = 0
		}
		if cpuUsage > 100 {
			cpuUsage = 100
		}
	}

	// 构建指标对象
	metrics := &Metrics{
		Timestamp:  now,
		Goroutines: goroutines,
		Memory: MemoryMetrics{
			HeapAlloc:    memStats.HeapAlloc,
			HeapInuse:    memStats.HeapInuse,
			HeapIdle:     memStats.HeapIdle,
			HeapReleased: memStats.HeapReleased,
			StackInuse:   memStats.StackInuse,
			TotalAlloc:   memStats.TotalAlloc,
			Sys:          memStats.Sys,
		},
		CPU: CPUMetrics{
			UsagePercent: cpuUsage,
		},
		GC: GCMetrics{
			NumGC:      memStats.NumGC,
			PauseTotal: time.Duration(memStats.PauseTotalNs),
			LastPause:  time.Duration(memStats.PauseNs[(memStats.NumGC+255)%256]),
			PauseAvg:   mc.calculateAvgGCPause(&memStats),
		},
	}

	// 存储指标
	mc.metrics.Set(metrics)

	// 广播指标到 WebSocket 客户端
	if mc.wsManager != nil {
		mc.wsManager.BroadcastMetrics(metrics)
	}

	return nil
}

// calculateCPUUsage 计算 CPU 使用率百分比
func (mc *metricsCollector) calculateCPUUsage() float64 {
	current := mc.getCPUStats()

	// 检测 CPU 统计是否可用（仅在首次调用时检测）
	mc.platformFeatures.mu.Lock()
	if !mc.platformFeatures.checkedOnce {
		mc.platformFeatures.checkedOnce = true
		// 如果 CPU 统计返回全零值，说明该平台不支持
		if current.userTime == 0 && current.systemTime == 0 && !mc.lastCPU.timestamp.IsZero() {
			mc.platformFeatures.cpuStatsAvailable = false
			log.Printf("[WARN] CPU statistics not available on this platform (%s/%s), CPU usage will be reported as 0%%",
				runtime.GOOS, runtime.GOARCH)
		} else {
			mc.platformFeatures.cpuStatsAvailable = true
		}
	}
	cpuAvailable := mc.platformFeatures.cpuStatsAvailable
	mc.platformFeatures.mu.Unlock()

	// 如果 CPU 统计不可用，直接返回 0
	if !cpuAvailable {
		return 0.0
	}

	// 计算时间差
	timeDelta := current.timestamp.Sub(mc.lastCPU.timestamp).Seconds()
	if timeDelta <= 0 {
		return 0.0
	}

	// 计算 CPU 时间差（纳秒）
	userDelta := current.userTime - mc.lastCPU.userTime
	systemDelta := current.systemTime - mc.lastCPU.systemTime
	totalCPUTime := float64(userDelta+systemDelta) / 1e9 // 转换为秒

	// 计算使用率百分比
	cpuUsage := (totalCPUTime / timeDelta) * 100.0

	// 限制在 0-100 范围内
	if cpuUsage < 0 {
		cpuUsage = 0
	}
	if cpuUsage > 100 {
		cpuUsage = 100
	}

	// 更新上次统计
	mc.lastCPU = current

	return cpuUsage
}

// calculateAvgGCPause 计算平均 GC 暂停时间
func (mc *metricsCollector) calculateAvgGCPause(memStats *runtime.MemStats) time.Duration {
	if memStats.NumGC == 0 {
		return 0
	}
	return time.Duration(memStats.PauseTotalNs / uint64(memStats.NumGC))
}

// GetMetrics 返回当前指标的副本
func (mc *metricsCollector) GetMetrics() *Metrics {
	return mc.metrics.Get()
}
