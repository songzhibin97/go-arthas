package agent

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// TestCrossPlatformCompilation 测试跨平台编译兼容性
func TestCrossPlatformCompilation(t *testing.T) {
	// 此测试验证代码在当前平台上可以编译和运行
	t.Logf("Testing on platform: GOOS=%s, GOARCH=%s", runtime.GOOS, runtime.GOARCH)

	// 验证 Go 版本
	version := runtime.Version()
	t.Logf("Go version: %s", version)

	// 基本的运行时功能测试
	goroutines := runtime.NumGoroutine()
	if goroutines <= 0 {
		t.Errorf("Expected positive goroutine count, got %d", goroutines)
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	if memStats.Sys == 0 {
		t.Error("Expected non-zero system memory")
	}

	t.Logf("Platform test passed: %d goroutines, %d bytes system memory",
		goroutines, memStats.Sys)
}

// TestPlatformSpecificCPUStats 测试平台特定的 CPU 统计功能
func TestPlatformSpecificCPUStats(t *testing.T) {
	collector := newMetricsCollector(1 * time.Second)

	// 获取 CPU 统计
	stats := collector.getCPUStats()

	// 验证时间戳
	if stats.timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	// 在某些平台上，CPU 统计可能不可用（返回零值）
	// 这是可以接受的，只要不崩溃
	t.Logf("Platform: %s/%s, CPU stats: user=%d ns, system=%d ns",
		runtime.GOOS, runtime.GOARCH, stats.userTime, stats.systemTime)

	// 如果 CPU 统计可用，验证它们是非负的
	if stats.userTime < 0 {
		t.Errorf("Expected non-negative user time, got %d", stats.userTime)
	}
	if stats.systemTime < 0 {
		t.Errorf("Expected non-negative system time, got %d", stats.systemTime)
	}
}

// TestMetricsCollectionOnCurrentPlatform 测试当前平台上的指标收集
func TestMetricsCollectionOnCurrentPlatform(t *testing.T) {
	collector := newMetricsCollector(100 * time.Millisecond)

	// 启动收集器
	collector.start()
	defer collector.stop()

	// 等待几次收集
	time.Sleep(300 * time.Millisecond)

	// 获取指标
	metrics := collector.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}

	// 验证基本指标
	if metrics.Goroutines <= 0 {
		t.Errorf("Expected positive goroutine count, got %d", metrics.Goroutines)
	}

	if metrics.Memory.Sys == 0 {
		t.Error("Expected non-zero system memory")
	}

	// CPU 使用率应该在 0-100 范围内
	if metrics.CPU.UsagePercent < 0 || metrics.CPU.UsagePercent > 100 {
		t.Errorf("Expected CPU usage in [0, 100], got %.2f", metrics.CPU.UsagePercent)
	}

	t.Logf("Platform %s/%s metrics collection successful", runtime.GOOS, runtime.GOARCH)
	t.Logf("  Goroutines: %d", metrics.Goroutines)
	t.Logf("  Memory Sys: %d bytes", metrics.Memory.Sys)
	t.Logf("  CPU Usage: %.2f%%", metrics.CPU.UsagePercent)
}

// TestAgentStartStopOnCurrentPlatform 测试 Agent 在当前平台上的启动和停止
func TestAgentStartStopOnCurrentPlatform(t *testing.T) {
	config := Config{
		Port:          18565, // 使用独立端口避免与其他测试冲突
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "info",
	}

	// 启动 Agent
	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent on %s/%s: %v", runtime.GOOS, runtime.GOARCH, err)
	}

	// 等待一小段时间确保 Agent 完全启动
	time.Sleep(200 * time.Millisecond)

	// 停止 Agent
	err = Stop()
	if err != nil {
		t.Errorf("Failed to stop agent on %s/%s: %v", runtime.GOOS, runtime.GOARCH, err)
	}

	t.Logf("Agent lifecycle test passed on %s/%s", runtime.GOOS, runtime.GOARCH)
}

// TestPlatformInfo 测试系统信息收集
func TestPlatformInfo(t *testing.T) {
	// 创建系统信息（模拟 agent 中的逻辑）
	startTime := time.Now()
	info := SystemInfo{
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
		ProcessID: os.Getpid(),
		StartTime: startTime,
		Uptime:    time.Since(startTime).String(),
	}

	// 验证基本字段
	if info.GoVersion == "" {
		t.Error("Expected non-empty Go version")
	}
	if info.GOOS == "" {
		t.Error("Expected non-empty GOOS")
	}
	if info.GOARCH == "" {
		t.Error("Expected non-empty GOARCH")
	}
	if info.NumCPU <= 0 {
		t.Errorf("Expected positive CPU count, got %d", info.NumCPU)
	}
	if info.ProcessID <= 0 {
		t.Errorf("Expected positive process ID, got %d", info.ProcessID)
	}

	t.Logf("Platform info collected successfully:")
	t.Logf("  Go Version: %s", info.GoVersion)
	t.Logf("  OS: %s", info.GOOS)
	t.Logf("  Arch: %s", info.GOARCH)
	t.Logf("  CPUs: %d", info.NumCPU)
	t.Logf("  PID: %d", info.ProcessID)
}
