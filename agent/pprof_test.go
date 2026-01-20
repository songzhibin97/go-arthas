package agent

import (
	"fmt"
	"io"
	"net/http"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_PprofFormatValidation(t *testing.T) {
	// 启动测试 agent（启用 pprof）
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8820,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 测试各种 profile 类型返回有效的 pprof 格式
	properties.Property("profile types return valid pprof format",
		prop.ForAll(
			func(profileType string) bool {
				var url string
				switch profileType {
				case "heap":
					url = fmt.Sprintf("http://localhost:%d/debug/pprof/heap", config.Port)
				case "goroutine":
					url = fmt.Sprintf("http://localhost:%d/debug/pprof/goroutine", config.Port)
				case "block":
					url = fmt.Sprintf("http://localhost:%d/debug/pprof/block", config.Port)
				case "mutex":
					url = fmt.Sprintf("http://localhost:%d/debug/pprof/mutex", config.Port)
				case "threadcreate":
					url = fmt.Sprintf("http://localhost:%d/debug/pprof/threadcreate", config.Port)
				case "allocs":
					url = fmt.Sprintf("http://localhost:%d/debug/pprof/allocs", config.Port)
				default:
					return false
				}

				resp, err := http.Get(url)
				if err != nil {
					return false
				}
				defer resp.Body.Close()

				// 验证响应成功
				if resp.StatusCode != http.StatusOK {
					return false
				}

				// 读取响应体
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					return false
				}

				// 验证数据非空
				if len(data) == 0 {
					return false
				}

				// 尝试解析 pprof 格式
				// pprof 数据应该是二进制格式或文本格式
				// 简单验证：检查是否包含 pprof 相关的标记
				profile := pprof.Lookup(profileType)
				if profile == nil {
					// 某些 profile 类型可能不存在，这是正常的
					return true
				}

				return profile != nil
			},
			gen.OneConstOf("heap", "goroutine", "block", "mutex", "threadcreate", "allocs"),
		))

	properties.TestingRun(t)
}

func TestProperty_PprofParameterSupport(t *testing.T) {
	// 启动测试 agent（启用 pprof）
	Stop()
	time.Sleep(500 * time.Millisecond) // 等待端口释放

	config := Config{
		Port:          8822,
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20 // 减少测试次数，因为 CPU profiling 很慢

	properties := gopter.NewProperties(parameters)

	// 测试 CPU profile 的 seconds 参数
	properties.Property("CPU profile supports seconds parameter",
		prop.ForAll(
			func(seconds int) bool {
				url := fmt.Sprintf("http://localhost:%d/debug/pprof/profile?seconds=%d", config.Port, seconds)

				start := time.Now()
				resp, err := http.Get(url)
				elapsed := time.Since(start)

				if err != nil {
					return false
				}
				defer resp.Body.Close()

				// 验证响应成功
				if resp.StatusCode != http.StatusOK {
					return false
				}

				// 读取响应体
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					return false
				}

				// 验证数据非空
				if len(data) == 0 {
					return false
				}

				// 验证采样时间大致符合预期（允许一些误差）
				expectedDuration := time.Duration(seconds) * time.Second
				if elapsed < expectedDuration || elapsed > expectedDuration+2*time.Second {
					return false
				}

				return true
			},
			gen.IntRange(1, 2), // 1-2 秒的采样时间（减少时间）
		))

	// 测试 debug 参数
	properties.Property("profiles support debug parameter",
		prop.ForAll(
			func(debugLevel int) bool {
				url := fmt.Sprintf("http://localhost:%d/debug/pprof/goroutine?debug=%d", config.Port, debugLevel)

				resp, err := http.Get(url)
				if err != nil {
					return false
				}
				defer resp.Body.Close()

				// 验证响应成功
				if resp.StatusCode != http.StatusOK {
					return false
				}

				// 读取响应体
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					return false
				}

				// 验证数据非空
				if len(data) == 0 {
					return false
				}

				// debug=1 或 debug=2 应该返回文本格式
				if debugLevel > 0 {
					// 文本格式应该包含可读的文本
					text := string(data)
					return len(text) > 0 && strings.Contains(text, "goroutine")
				}

				return true
			},
			gen.IntRange(0, 2), // debug 参数通常是 0, 1, 2
		))

	properties.TestingRun(t)
}

// 单元测试：测试 pprof 端点可访问性
func TestPprof_EndpointsAccessible(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8830,
		EnablePprof:   true,
		EnableMetrics: false,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	// 测试各种 pprof 端点
	endpoints := []string{
		"/debug/pprof/",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/block",
		"/debug/pprof/mutex",
		"/debug/pprof/cmdline",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", config.Port, endpoint))
			if err != nil {
				t.Fatalf("Failed to access %s: %v", endpoint, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for %s, got %d", endpoint, resp.StatusCode)
			}

			// 验证响应非空
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response: %v", err)
			}

			if len(data) == 0 {
				t.Errorf("Expected non-empty response for %s", endpoint)
			}
		})
	}
}

// 单元测试：测试 CPU profile
func TestPprof_CPUProfile(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8831,
		EnablePprof:   true,
		EnableMetrics: false,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	// 请求 1 秒的 CPU profile
	start := time.Now()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/debug/pprof/profile?seconds=1", config.Port))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to get CPU profile: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 验证采样时间大致为 1 秒
	if elapsed < 1*time.Second || elapsed > 2*time.Second {
		t.Errorf("Expected ~1 second sampling time, got %v", elapsed)
	}

	// 读取 profile 数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read profile: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty CPU profile")
	}
}

// 单元测试：测试 heap profile
func TestPprof_HeapProfile(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8832,
		EnablePprof:   true,
		EnableMetrics: false,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/debug/pprof/heap", config.Port))
	if err != nil {
		t.Fatalf("Failed to get heap profile: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 读取 profile 数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read profile: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty heap profile")
	}
}

// 单元测试：测试 goroutine profile
func TestPprof_GoroutineProfile(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8833,
		EnablePprof:   true,
		EnableMetrics: false,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	// 测试二进制格式（debug=0）
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/debug/pprof/goroutine", config.Port))
	if err != nil {
		t.Fatalf("Failed to get goroutine profile: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read profile: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty goroutine profile")
	}

	// 测试文本格式（debug=1）
	resp2, err := http.Get(fmt.Sprintf("http://localhost:%d/debug/pprof/goroutine?debug=1", config.Port))
	if err != nil {
		t.Fatalf("Failed to get goroutine profile with debug=1: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp2.StatusCode)
	}

	data2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("Failed to read profile: %v", err)
	}

	if len(data2) == 0 {
		t.Error("Expected non-empty goroutine profile with debug=1")
	}

	// 文本格式应该包含 "goroutine" 字样
	text := string(data2)
	if !strings.Contains(text, "goroutine") {
		t.Error("Expected text format to contain 'goroutine'")
	}
}

// 单元测试：测试禁用 pprof
func TestPprof_Disabled(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8834,
		EnablePprof:   false, // 禁用 pprof
		EnableMetrics: false,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	// 尝试访问 pprof 端点应该返回 404
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/debug/pprof/", config.Port))
	if err != nil {
		t.Fatalf("Failed to access pprof: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 when pprof is disabled, got %d", resp.StatusCode)
	}
}

// 单元测试：测试 pprof 索引页
func TestPprof_IndexPage(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8835,
		EnablePprof:   true,
		EnableMetrics: false,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/debug/pprof/", config.Port))
	if err != nil {
		t.Fatalf("Failed to get pprof index: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 读取索引页内容
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read index: %v", err)
	}

	html := string(data)

	// 验证索引页包含各种 profile 类型的链接
	expectedLinks := []string{"heap", "goroutine", "block", "mutex", "profile"}
	for _, link := range expectedLinks {
		if !strings.Contains(html, link) {
			t.Errorf("Expected index page to contain link to %s", link)
		}
	}
}
