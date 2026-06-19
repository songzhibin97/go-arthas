package agent

import (
	"bytes"
	"log"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestGracefulDegradationWithUnavailableFeatures 测试平台特性不可用时的优雅降级
func TestGracefulDegradationWithUnavailableFeatures(t *testing.T) {
	// 创建一个收集器
	collector := newMetricsCollector(100 * time.Millisecond)

	// 捕获日志输出
	var logBuf bytes.Buffer
	originalLogOutput := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalLogOutput)

	// 模拟 CPU 统计不可用的情况
	// 通过设置 lastCPU 为非零时间戳，然后让 getCPUStats 返回零值
	collector.lastCPU = cpuStats{
		userTime:   0,
		systemTime: 0,
		timestamp:  time.Now().Add(-1 * time.Second),
	}

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

	// 验证即使 CPU 统计不可用，其他指标仍然正常收集
	if metrics.Goroutines <= 0 {
		t.Errorf("Expected positive goroutine count, got %d", metrics.Goroutines)
	}

	if metrics.Memory.Sys == 0 {
		t.Error("Expected non-zero system memory")
	}

	// CPU 使用率应该在有效范围内（即使是 0 也是有效的）
	if metrics.CPU.UsagePercent < 0 || metrics.CPU.UsagePercent > 100 {
		t.Errorf("Expected CPU usage in [0, 100], got %.2f", metrics.CPU.UsagePercent)
	}

	t.Logf("Graceful degradation test passed")
	t.Logf("  Goroutines: %d", metrics.Goroutines)
	t.Logf("  Memory Sys: %d bytes", metrics.Memory.Sys)
	t.Logf("  CPU Usage: %.2f%%", metrics.CPU.UsagePercent)
}

// TestPlatformFeatureDetection 测试平台特性检测
func TestPlatformFeatureDetection(t *testing.T) {
	collector := newMetricsCollector(1 * time.Second)

	// 捕获日志输出
	var logBuf bytes.Buffer
	originalLogOutput := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalLogOutput)

	// 初始化 lastCPU
	collector.lastCPU = collector.getCPUStats()
	time.Sleep(100 * time.Millisecond)

	// 调用 calculateCPUUsage 触发特性检测
	_ = collector.calculateCPUUsage()

	// 检查平台特性是否已检测
	collector.platformFeatures.mu.RLock()
	checked := collector.platformFeatures.checkedOnce
	cpuAvailable := collector.platformFeatures.cpuStatsAvailable
	collector.platformFeatures.mu.RUnlock()

	if !checked {
		t.Error("Expected platform features to be checked")
	}

	// 在支持的平台上（Unix/Darwin/Windows），CPU 统计应该可用
	expectedAvailable := runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "windows"

	t.Logf("Platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	t.Logf("CPU stats available: %v (expected: %v)", cpuAvailable, expectedAvailable)

	// 如果 CPU 统计不可用，应该有警告日志
	if !cpuAvailable {
		logOutput := logBuf.String()
		if !strings.Contains(logOutput, "CPU statistics not available") {
			t.Error("Expected warning log for unavailable CPU statistics")
		}
		t.Logf("Warning logged correctly: %s", logOutput)
	}
}

// TestCollectorContinuesAfterFeatureFailure 测试特性失败后收集器继续运行
func TestCollectorContinuesAfterFeatureFailure(t *testing.T) {
	collector := newMetricsCollector(50 * time.Millisecond)

	// 启动收集器
	collector.start()
	defer collector.stop()

	// 等待多次收集
	time.Sleep(200 * time.Millisecond)

	// 获取多次指标，验证收集器持续运行
	for i := 0; i < 3; i++ {
		metrics := collector.GetMetrics()
		if metrics == nil {
			t.Fatalf("Iteration %d: Expected non-nil metrics", i)
		}

		if metrics.Goroutines <= 0 {
			t.Errorf("Iteration %d: Expected positive goroutine count", i)
		}

		time.Sleep(60 * time.Millisecond)
	}

	t.Log("Collector continues running successfully after multiple collections")
}

// TestAgentStartsWithUnavailableFeatures 测试即使某些特性不可用，Agent 仍能启动
func TestAgentStartsWithUnavailableFeatures(t *testing.T) {
	config := Config{
		Port:          18564,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "info",
	}

	// 捕获日志输出
	var logBuf bytes.Buffer
	originalLogOutput := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalLogOutput)

	// 启动 Agent
	err := Start(config)
	if err != nil {
		t.Fatalf("Agent should start even with unavailable features: %v", err)
	}

	// 等待一小段时间
	time.Sleep(200 * time.Millisecond)

	// 停止 Agent
	err = Stop()
	if err != nil {
		t.Errorf("Failed to stop agent: %v", err)
	}

	t.Log("Agent started and stopped successfully despite platform limitations")

	// 检查是否有适当的警告日志（如果有不可用的特性）
	logOutput := logBuf.String()
	if strings.Contains(logOutput, "not available") {
		t.Logf("Platform limitation warnings logged: %s", logOutput)
	}
}
