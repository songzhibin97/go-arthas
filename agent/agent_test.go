package agent

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty_AgentStartsAndServes 验证一个**真不变量**:对任意有效配置(pprof/metrics
// 各种开关组合)agent 都能干净启动并提供服务。
//
// 取代旧的 "启动 <1s" 墙钟阈值断言——那是环境敏感的绝对时序,在有负载机器上会偶发 flaky,
// 且不反映任何真实契约(实测启动 ~200µs,1s 阈值纯属噪声)。这里改为:启动成功 +
// /api/v1/info(与 metrics/pprof 开关无关、恒可用)返回 200,即"确实在服务"。
// 起停经 startOnFreePort(OS 空闲端口 + 重试),消除历史的端口冲突 flaky。
func TestProperty_AgentStartsAndServes(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 24 // 覆盖 pprof×metrics 四种组合多次;真不变量与迭代次数无关

	properties := gopter.NewProperties(parameters)

	properties.Property("agent starts cleanly and serves /api/v1/info for any valid pprof/metrics combo",
		prop.ForAll(
			func(enablePprof, enableMetrics bool) bool {
				port, err := startOnFreePort(Config{
					EnablePprof:   enablePprof,
					EnableMetrics: enableMetrics,
					LogLevel:      "error",
				})
				if err != nil {
					return false
				}
				defer Stop()

				client := &http.Client{Timeout: 3 * time.Second}
				resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/info", port))
				if err != nil {
					return false
				}
				defer resp.Body.Close()
				return resp.StatusCode == http.StatusOK
			},
			gen.Bool(), // enablePprof
			gen.Bool(), // enableMetrics
		))

	properties.TestingRun(t)
}

func TestProperty_ConfigurationValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 测试无效端口
	properties.Property("invalid port returns descriptive error",
		prop.ForAll(
			func(port int) bool {
				// 确保之前的 agent 已停止
				Stop()
				time.Sleep(10 * time.Millisecond)

				config := Config{
					Port:          port,
					EnablePprof:   true,
					EnableMetrics: true,
					LogLevel:      "info",
				}

				err := Start(config)

				// 清理（如果意外启动）
				defer Stop()

				// 应该返回错误且错误消息非空
				return err != nil && err.Error() != ""
			},
			gen.IntRange(-1000, -1).WithLabel("negative ports"),
		))

	// 测试超出范围的端口
	properties.Property("out of range port returns descriptive error",
		prop.ForAll(
			func(port int) bool {
				// 确保之前的 agent 已停止
				Stop()
				time.Sleep(10 * time.Millisecond)

				config := Config{
					Port:          port,
					EnablePprof:   true,
					EnableMetrics: true,
					LogLevel:      "info",
				}

				err := Start(config)

				// 清理（如果意外启动）
				defer Stop()

				// 应该返回错误且错误消息非空
				return err != nil && err.Error() != ""
			},
			gen.IntRange(65536, 70000).WithLabel("out of range ports"),
		))

	properties.TestingRun(t)
}

func TestProperty_GracefulStartupFailure(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("application continues on startup failure",
		prop.ForAll(
			func(port int) bool {
				// 确保之前的 agent 已停止
				Stop()
				time.Sleep(10 * time.Millisecond)

				// 首先启动一个 agent 占用端口
				config1 := Config{
					Port:          port,
					EnablePprof:   true,
					EnableMetrics: true,
					LogLevel:      "info",
				}

				err := Start(config1)
				if err != nil {
					// 如果第一个启动失败，跳过这个测试用例
					return true
				}

				// 尝试在同一端口启动第二个 agent（应该失败）
				config2 := Config{
					Port:          port,
					EnablePprof:   true,
					EnableMetrics: true,
					LogLevel:      "info",
				}

				err2 := Start(config2)

				// 清理第一个 agent
				defer Stop()

				// 第二个启动应该失败且有描述性错误消息
				// 应用程序应该继续运行（不会 panic）
				return err2 != nil && err2.Error() != ""
			},
			gen.IntRange(8100, 8200),
		))

	properties.TestingRun(t)
}

// 单元测试：测试基本的启动和停止
func TestAgent_StartStop(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8765,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "info",
	}

	// 启动 agent
	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	// 验证 agent 正在运行
	if globalAgent == nil || !globalAgent.running {
		t.Fatal("Agent should be running")
	}

	// 停止 agent
	err = Stop()
	if err != nil {
		t.Fatalf("Failed to stop agent: %v", err)
	}

	// 验证 agent 已停止
	if globalAgent != nil && globalAgent.running {
		t.Fatal("Agent should be stopped")
	}
}

// 单元测试：测试重复启动
func TestAgent_DoubleStart(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8766,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "info",
	}

	// 第一次启动
	err := Start(config)
	if err != nil {
		t.Fatalf("First start failed: %v", err)
	}
	defer Stop()

	// 第二次启动应该失败
	err = Start(config)
	if err == nil {
		t.Fatal("Expected error on double start, got nil")
	}
}

// 单元测试：测试无效配置
func TestAgent_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "invalid port - negative",
			config: Config{
				Port:          -1,
				EnablePprof:   true,
				EnableMetrics: true,
				LogLevel:      "info",
			},
		},
		{
			name: "invalid port - too large",
			config: Config{
				Port:          70000,
				EnablePprof:   true,
				EnableMetrics: true,
				LogLevel:      "info",
			},
		},
		{
			name: "invalid log level",
			config: Config{
				Port:          8767,
				EnablePprof:   true,
				EnableMetrics: true,
				LogLevel:      "invalid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 确保清理
			Stop()
			time.Sleep(10 * time.Millisecond)

			err := Start(tt.config)
			if err == nil {
				Stop() // 清理
				t.Fatal("Expected error, got nil")
			}
		})
	}
}

// 单元测试：测试默认配置
func TestAgent_DefaultConfig(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		// 不设置任何值，测试默认值
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start with default config: %v", err)
	}
	defer Stop()

	// 验证默认值
	if globalAgent.config.Port != 8563 {
		t.Errorf("Expected default port 8563, got %d", globalAgent.config.Port)
	}
	if globalAgent.config.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %q", globalAgent.config.LogLevel)
	}
}

// 单元测试：测试 GetMetrics
func TestAgent_GetMetrics(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8768,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "info",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待收集器收集一次指标
	time.Sleep(1500 * time.Millisecond)

	metrics := GetMetrics()
	if metrics == nil {
		t.Fatal("Expected metrics, got nil")
	}

	// 验证指标包含有效数据
	if metrics.Goroutines <= 0 {
		t.Errorf("Expected positive goroutine count, got %d", metrics.Goroutines)
	}
}

// 单元测试：测试禁用指标收集
func TestAgent_DisabledMetrics(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8769,
		EnablePprof:   true,
		EnableMetrics: false, // 禁用指标收集
		LogLevel:      "info",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// GetMetrics 应该返回 nil
	metrics := GetMetrics()
	if metrics != nil {
		t.Fatal("Expected nil metrics when collection is disabled")
	}
}

// 基准测试：测试启动性能
func BenchmarkAgent_Start(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		Stop()
		time.Sleep(10 * time.Millisecond)

		config := Config{
			Port:          8770 + (i % 100),
			EnablePprof:   true,
			EnableMetrics: true,
			LogLevel:      "error",
		}

		b.StartTimer()
		err := Start(config)
		b.StopTimer()

		if err != nil {
			b.Fatalf("Failed to start: %v", err)
		}

		Stop()
	}
}

// 测试辅助函数：等待端口可用
func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// 尝试连接端口
		addr := fmt.Sprintf("localhost:%d", port)
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("port %d not available after %v", port, timeout)
}

// 单元测试：测试 panic 恢复
func TestAgent_PanicRecovery(t *testing.T) {
	// 这个测试验证 agent 在遇到 panic 时能够恢复并继续运行
	// 由于我们的 goroutines 都有 panic recovery，agent 应该能够继续服务

	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8771,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "error", // 减少日志输出
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 完全启动
	time.Sleep(100 * time.Millisecond)

	// 验证 agent 仍在运行
	if globalAgent == nil || !globalAgent.running {
		t.Fatal("Agent should still be running after potential panics")
	}

	// 验证可以获取指标（说明 collector 仍在工作）
	time.Sleep(1500 * time.Millisecond)
	metrics := GetMetrics()
	if metrics == nil {
		t.Fatal("Should be able to get metrics after panic recovery")
	}
}

// 单元测试：测试 collector 在 panic 后继续运行
func TestCollector_ContinuesAfterError(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8772,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待收集器收集几次指标
	time.Sleep(2500 * time.Millisecond)

	// 验证可以获取指标
	metrics := GetMetrics()
	if metrics == nil {
		t.Fatal("Expected metrics, got nil")
	}

	// 验证指标是最近的（说明 collector 一直在运行）
	age := time.Since(metrics.Timestamp)
	if age > 2*time.Second {
		t.Errorf("Metrics are too old (%v), collector may not be running", age)
	}
}
