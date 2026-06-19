package agent

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_ErrorResilience(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Agent continues after metrics collection errors
	properties.Property("agent continues after metrics collection errors",
		prop.ForAll(
			func(iterations int) bool {
				// 确保清理
				Stop()
				time.Sleep(10 * time.Millisecond)

				config := Config{
					Port:          8900,
					EnablePprof:   true,
					EnableMetrics: true,
					LogLevel:      "error", // 减少日志输出
				}

				err := Start(config)
				if err != nil {
					return false
				}
				defer Stop()

				// 等待 agent 启动
				time.Sleep(100 * time.Millisecond)

				// 收集多次指标，即使有错误也应该继续
				successCount := 0
				for i := 0; i < iterations; i++ {
					metrics := GetMetrics()
					if metrics != nil {
						successCount++
					}
					time.Sleep(50 * time.Millisecond)
				}

				// 应该至少成功收集一些指标
				return successCount > 0
			},
			gen.IntRange(3, 10), // 测试 3-10 次迭代
		))

	// Agent continues after HTTP handler errors
	properties.Property("agent continues after HTTP handler errors",
		prop.ForAll(
			func(invalidRequests int) bool {
				// 确保清理
				Stop()
				time.Sleep(10 * time.Millisecond)

				config := Config{
					Port:          8901,
					EnablePprof:   true,
					EnableMetrics: true,
					LogLevel:      "error",
				}

				err := Start(config)
				if err != nil {
					return false
				}
				defer Stop()

				// 等待 agent 启动
				time.Sleep(100 * time.Millisecond)

				// 发送一些无效请求
				client := &http.Client{Timeout: 1 * time.Second}
				for i := 0; i < invalidRequests; i++ {
					// 发送无效方法
					req, _ := http.NewRequest("POST", "http://localhost:8901/api/v1/metrics", nil)
					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
					}

					// 发送不存在的端点
					resp, err = client.Get("http://localhost:8901/api/v1/nonexistent")
					if err == nil {
						resp.Body.Close()
					}
				}

				// 验证 agent 仍然可以响应有效请求
				resp, err := client.Get("http://localhost:8901/api/v1/metrics")
				if err != nil {
					return false
				}
				defer resp.Body.Close()

				return resp.StatusCode == http.StatusOK
			},
			gen.IntRange(1, 5), // 发送 1-5 个无效请求
		))

	// Agent logs errors and continues
	properties.Property("agent logs errors and continues operation",
		prop.ForAll(
			func(duration int) bool {
				// 确保清理
				Stop()
				time.Sleep(10 * time.Millisecond)

				config := Config{
					Port:          8902,
					EnablePprof:   true,
					EnableMetrics: true,
					LogLevel:      "error",
				}

				err := Start(config)
				if err != nil {
					return false
				}
				defer Stop()

				// 运行一段时间
				time.Sleep(time.Duration(duration) * time.Millisecond)

				// 验证 agent 仍在运行
				if globalAgent == nil || !globalAgent.running {
					return false
				}

				// 验证可以获取指标
				metrics := GetMetrics()
				if metrics == nil {
					return false
				}

				// 验证指标是最近的
				age := time.Since(metrics.Timestamp)
				return age < 2*time.Second
			},
			gen.IntRange(100, 500), // 运行 100-500ms
		))

	// No crashes or panics during operation
	properties.Property("no crashes or panics during operation",
		prop.ForAll(
			func(workload int) bool {
				// 确保清理
				Stop()
				time.Sleep(10 * time.Millisecond)

				config := Config{
					Port:          8903,
					EnablePprof:   true,
					EnableMetrics: true,
					LogLevel:      "error",
				}

				err := Start(config)
				if err != nil {
					return false
				}
				defer Stop()

				// 创建一些工作负载
				done := make(chan struct{})
				for i := 0; i < workload; i++ {
					go func() {
						<-done
					}()
				}

				// 等待一段时间
				time.Sleep(200 * time.Millisecond)

				// 清理工作负载
				close(done)
				time.Sleep(50 * time.Millisecond)

				// 验证 agent 仍在运行且没有 panic
				if globalAgent == nil || !globalAgent.running {
					return false
				}

				// 验证可以获取指标
				metrics := GetMetrics()
				return metrics != nil
			},
			gen.IntRange(10, 100), // 创建 10-100 个 goroutine
		))

	properties.TestingRun(t)
}

// 单元测试：测试收集器在错误后继续运行
func TestErrorResilience_CollectorContinuesAfterError(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8904,
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

// 单元测试：测试 HTTP 服务器在错误后继续服务
func TestErrorResilience_HTTPServerContinuesAfterError(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8905,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 启动
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 1 * time.Second}

	// 发送一些无效请求
	for i := 0; i < 10; i++ {
		// 无效方法
		req, _ := http.NewRequest("POST", "http://localhost:8905/api/v1/metrics", nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}

		// 不存在的端点
		resp, err = client.Get("http://localhost:8905/api/v1/nonexistent")
		if err == nil {
			resp.Body.Close()
		}
	}

	// 验证服务器仍然可以响应有效请求
	resp, err := client.Get("http://localhost:8905/api/v1/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics after errors: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 验证响应体可以读取
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if len(body) == 0 {
		t.Error("Expected non-empty response body")
	}
}

// 单元测试：测试 pprof 端点错误处理
func TestErrorResilience_PprofContinuesAfterError(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8906,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 启动
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}

	// 发送一些 pprof 请求（可能会失败或成功）
	pprofEndpoints := []string{
		"/debug/pprof/",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/profile?seconds=1",
	}

	for _, endpoint := range pprofEndpoints {
		resp, err := client.Get(fmt.Sprintf("http://localhost:8906%s", endpoint))
		if err == nil {
			resp.Body.Close()
		}
	}

	// 验证 agent 仍然可以响应其他请求
	resp, err := client.Get("http://localhost:8906/api/v1/info")
	if err != nil {
		t.Fatalf("Failed to get info after pprof requests: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// 单元测试：测试并发请求下的错误恢复
func TestErrorResilience_ConcurrentRequests(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8907,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 启动
	time.Sleep(100 * time.Millisecond)

	// 并发发送大量请求
	concurrency := 50
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			client := &http.Client{Timeout: 2 * time.Second}

			// 发送多个请求
			for j := 0; j < 5; j++ {
				// 有效请求
				resp, err := client.Get("http://localhost:8907/api/v1/metrics")
				if err == nil {
					resp.Body.Close()
				}

				// 无效请求
				req, _ := http.NewRequest("DELETE", "http://localhost:8907/api/v1/metrics", nil)
				resp, err = client.Do(req)
				if err == nil {
					resp.Body.Close()
				}
			}

			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 验证 agent 仍在运行
	if globalAgent == nil || !globalAgent.running {
		t.Fatal("Agent should still be running after concurrent requests")
	}

	// 验证可以获取指标
	metrics := GetMetrics()
	if metrics == nil {
		t.Fatal("Should be able to get metrics after concurrent requests")
	}
}

// 单元测试：测试长时间运行的稳定性
func TestErrorResilience_LongRunningStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8908,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 运行 10 秒，持续发送请求
	deadline := time.Now().Add(10 * time.Second)
	client := &http.Client{Timeout: 1 * time.Second}

	requestCount := 0
	errorCount := 0

	for time.Now().Before(deadline) {
		resp, err := client.Get("http://localhost:8908/api/v1/metrics")
		requestCount++

		if err != nil {
			errorCount++
		} else {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errorCount++
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	// 验证 agent 仍在运行
	if globalAgent == nil || !globalAgent.running {
		t.Fatal("Agent should still be running after long operation")
	}

	// 验证大部分请求成功
	successRate := float64(requestCount-errorCount) / float64(requestCount)
	if successRate < 0.9 {
		t.Errorf("Success rate too low: %.2f%% (%d/%d)", successRate*100, requestCount-errorCount, requestCount)
	}

	t.Logf("Long-running test: %d requests, %d errors, %.2f%% success rate",
		requestCount, errorCount, successRate*100)
}

// 单元测试：测试内存泄漏检测
func TestErrorResilience_NoMemoryLeak(t *testing.T) {
	// 清理可能残留的实例后再取内存基线
	_ = Stop()
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// 在 OS 空闲端口启动,消除历史固定端口(8910 落在其它测试端口区间内)的冲突 flaky
	port, err := startOnFreePort(Config{
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "error",
	})
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	// 运行一段时间
	time.Sleep(2 * time.Second)

	// 发送一些请求
	client := &http.Client{Timeout: 1 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/metrics", port)
	for i := 0; i < 100; i++ {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
		}
	}

	// 停止 agent
	Stop()
	time.Sleep(100 * time.Millisecond)

	// 强制 GC
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// 记录最终内存
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// 计算内存增长。m2.Alloc 可能小于 m1.Alloc（agent 停止 + GC 后内存反而下降，
	// 正是"无泄漏"的健康情况）。两者都是 uint64，直接相减会下溢成 ~2^64，误把内存
	// 下降报成天文数字的"泄漏"。用有符号差值，内存下降即视为 0 增长。
	memGrowth := int64(m2.Alloc) - int64(m1.Alloc)

	// 允许一定的内存增长（小于 10MB）
	if memGrowth > 10*1024*1024 {
		t.Errorf("Potential memory leak: memory grew by %d bytes (%.2f MB)",
			memGrowth, float64(memGrowth)/(1024*1024))
	}

	t.Logf("Memory growth: %d bytes (%.2f MB)", memGrowth, float64(memGrowth)/(1024*1024))
}
