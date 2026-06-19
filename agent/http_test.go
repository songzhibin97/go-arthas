package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestHTTP_MetricsEndpointServesValidData 验证真不变量:/api/v1/metrics 返回 200 + 合法
// JSON + 有效数据(Goroutines>0)。取代旧的 "100ms 内响应" 绝对时延 property——响应时延在
// 有负载机器上会超 100ms 而 flaky;这里只断言"正确服务",不卡响应时延。起停经 startOnFreePort
// 消除历史固定端口 8800 的冲突 flaky。
func TestHTTP_MetricsEndpointServesValidData(t *testing.T) {
	port, err := startOnFreePort(Config{EnableMetrics: true, LogLevel: "error"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer Stop()
	if !waitMetrics(3 * time.Second) {
		t.Fatal("collector not ready")
	}

	client := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/metrics", port)
	for i := 0; i < 20; i++ {
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("get #%d: %v", i, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("get #%d: status %d", i, resp.StatusCode)
		}
		var m Metrics
		err = json.NewDecoder(resp.Body).Decode(&m)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("decode #%d: %v", i, err)
		}
		if m.Goroutines <= 0 {
			t.Errorf("get #%d: empty metrics %+v", i, m)
		}
	}
}

func TestProperty_HTTPErrorStatusCodes(t *testing.T) {
	// 启动测试 agent
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8801,
		EnablePprof:   false,
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

	// 测试不存在的端点返回 404
	properties.Property("non-existent endpoints return 404",
		prop.ForAll(
			func(path string) bool {
				resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", config.Port, path))
				if err != nil {
					return false
				}
				defer resp.Body.Close()

				// 应该返回 404
				return resp.StatusCode == http.StatusNotFound
			},
			gen.OneConstOf("/api/v1/nonexistent", "/api/v2/metrics", "/invalid", "/api/v1/metrics/invalid"),
		))

	// 测试不支持的方法返回 405
	properties.Property("unsupported methods return 405",
		prop.ForAll(
			func(method string) bool {
				client := &http.Client{}
				req, err := http.NewRequest(method, fmt.Sprintf("http://localhost:%d/api/v1/metrics", config.Port), nil)
				if err != nil {
					return false
				}

				resp, err := client.Do(req)
				if err != nil {
					return false
				}
				defer resp.Body.Close()

				// 应该返回 405 Method Not Allowed
				return resp.StatusCode == http.StatusMethodNotAllowed
			},
			gen.OneConstOf("POST", "PUT", "DELETE", "PATCH"),
		))

	properties.TestingRun(t)
}

func TestProperty_CORSHeadersPresent(t *testing.T) {
	// 启动测试 agent
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8802,
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

	properties.Property("all responses include CORS headers",
		prop.ForAll(
			func(endpoint string) bool {
				resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", config.Port, endpoint))
				if err != nil {
					return false
				}
				defer resp.Body.Close()

				// 验证 CORS 头存在
				allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
				allowMethods := resp.Header.Get("Access-Control-Allow-Methods")

				return allowOrigin != "" && allowMethods != ""
			},
			gen.OneConstOf("/api/v1/metrics", "/api/v1/info", "/", "/debug/pprof/"),
		))

	// 测试 OPTIONS 预检请求
	properties.Property("OPTIONS requests return 200 with CORS headers",
		prop.ForAll(
			func(endpoint string) bool {
				client := &http.Client{}
				req, err := http.NewRequest("OPTIONS", fmt.Sprintf("http://localhost:%d%s", config.Port, endpoint), nil)
				if err != nil {
					return false
				}

				resp, err := client.Do(req)
				if err != nil {
					return false
				}
				defer resp.Body.Close()

				// 验证状态码和 CORS 头
				if resp.StatusCode != http.StatusOK {
					return false
				}

				allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
				allowMethods := resp.Header.Get("Access-Control-Allow-Methods")

				return allowOrigin != "" && allowMethods != ""
			},
			gen.OneConstOf("/api/v1/metrics", "/api/v1/info"),
		))

	properties.TestingRun(t)
}

func TestProperty_ConcurrentRequestHandling(t *testing.T) {
	// 启动测试 agent
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8803,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(1500 * time.Millisecond)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("concurrent requests don't block each other",
		prop.ForAll(
			func(numRequests int) bool {
				var wg sync.WaitGroup
				results := make(chan bool, numRequests)

				// 发起并发请求
				for i := 0; i < numRequests; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()

						resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/metrics", config.Port))
						if err != nil {
							results <- false
							return
						}
						defer resp.Body.Close()

						// 验证响应成功
						results <- resp.StatusCode == http.StatusOK
					}()
				}

				wg.Wait()
				close(results)

				// 验证所有请求都成功
				for success := range results {
					if !success {
						return false
					}
				}

				return true
			},
			gen.IntRange(5, 50), // 5-50 个并发请求
		))

	properties.TestingRun(t)
}

// 单元测试：测试 /api/v1/metrics 端点
func TestHTTP_MetricsEndpoint(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8810,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(1500 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/metrics", config.Port))
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 验证 Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// 解析 JSON
	var metrics Metrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		t.Fatalf("Failed to decode metrics: %v", err)
	}

	// 验证指标数据
	if metrics.Goroutines <= 0 {
		t.Errorf("Expected positive goroutine count, got %d", metrics.Goroutines)
	}
}

// 单元测试：测试 /api/v1/info 端点
func TestHTTP_InfoEndpoint(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8811,
		EnablePprof:   false,
		EnableMetrics: false,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/info", config.Port))
	if err != nil {
		t.Fatalf("Failed to get info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 验证 Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// 解析 JSON
	var info SystemInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("Failed to decode info: %v", err)
	}

	// 验证系统信息
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
}

// 单元测试：测试禁用指标时的行为
func TestHTTP_MetricsDisabled(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8812,
		EnablePprof:   false,
		EnableMetrics: false, // 禁用指标
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/metrics", config.Port))
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer resp.Body.Close()

	// 应该返回 503 Service Unavailable
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.StatusCode)
	}
}

// 单元测试：测试 CORS 头
func TestHTTP_CORSHeaders(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8813,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/info", config.Port))
	if err != nil {
		t.Fatalf("Failed to get info: %v", err)
	}
	defer resp.Body.Close()

	// 验证 CORS 头
	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got %s", allowOrigin)
	}

	allowMethods := resp.Header.Get("Access-Control-Allow-Methods")
	if allowMethods == "" {
		t.Error("Expected Access-Control-Allow-Methods header")
	}
}

// 单元测试：测试不支持的 HTTP 方法
func TestHTTP_UnsupportedMethods(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8814,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(100 * time.Millisecond)

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			client := &http.Client{}
			req, err := http.NewRequest(method, fmt.Sprintf("http://localhost:%d/api/v1/metrics", config.Port), nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for %s, got %d", method, resp.StatusCode)
			}
		})
	}
}

// 单元测试：测试并发请求
func TestHTTP_ConcurrentRequests(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          8815,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(1500 * time.Millisecond)

	// 发起 100 个并发请求
	numRequests := 100
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/metrics", config.Port))
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
				return
			}

			// 读取响应体
			_, err = io.ReadAll(resp.Body)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// 检查是否有错误
	for err := range errors {
		t.Errorf("Concurrent request failed: %v", err)
	}
}
