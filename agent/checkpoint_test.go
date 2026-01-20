package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// TestCheckpoint_HTTPAPIWorks 验证 HTTP API 的所有端点正常工作
func TestCheckpoint_HTTPAPIWorks(t *testing.T) {
	// 启动 agent
	config := Config{
		Port:          18563, // 使用测试端口避免冲突
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "info",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待服务器完全启动
	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%d", config.Port)

	t.Run("Metrics endpoint returns valid JSON", func(t *testing.T) {
		// 等待至少一次指标收集
		time.Sleep(1200 * time.Millisecond)

		resp, err := http.Get(baseURL + "/api/v1/metrics")
		if err != nil {
			t.Fatalf("Failed to GET /api/v1/metrics: %v", err)
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

		// 验证 CORS 头
		corsHeader := resp.Header.Get("Access-Control-Allow-Origin")
		if corsHeader != "*" {
			t.Errorf("Expected CORS header *, got %s", corsHeader)
		}

		// 解析 JSON
		var metrics Metrics
		if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
			t.Fatalf("Failed to decode metrics JSON: %v", err)
		}

		// 验证指标数据
		if metrics.Goroutines <= 0 {
			t.Errorf("Expected goroutines > 0, got %d", metrics.Goroutines)
		}

		if metrics.Memory.HeapAlloc == 0 {
			t.Errorf("Expected HeapAlloc > 0, got %d", metrics.Memory.HeapAlloc)
		}

		t.Logf("✓ Metrics endpoint returned valid JSON with %d goroutines", metrics.Goroutines)
	})

	t.Run("Info endpoint returns valid JSON", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/info")
		if err != nil {
			t.Fatalf("Failed to GET /api/v1/info: %v", err)
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
			t.Fatalf("Failed to decode info JSON: %v", err)
		}

		// 验证系统信息
		if info.GoVersion == "" {
			t.Error("Expected GoVersion to be set")
		}
		if info.GOOS == "" {
			t.Error("Expected GOOS to be set")
		}
		if info.NumCPU <= 0 {
			t.Errorf("Expected NumCPU > 0, got %d", info.NumCPU)
		}

		t.Logf("✓ Info endpoint returned valid JSON: Go %s on %s/%s", info.GoVersion, info.GOOS, info.GOARCH)
	})

	t.Run("Pprof index endpoint works", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/debug/pprof/")
		if err != nil {
			t.Fatalf("Failed to GET /debug/pprof/: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		// 验证响应包含 pprof 内容
		bodyStr := string(body)
		if len(bodyStr) == 0 {
			t.Error("Expected non-empty pprof index page")
		}

		t.Logf("✓ Pprof index endpoint returned %d bytes", len(body))
	})

	t.Run("Pprof heap profile works", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/debug/pprof/heap")
		if err != nil {
			t.Fatalf("Failed to GET /debug/pprof/heap: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		if len(body) == 0 {
			t.Error("Expected non-empty heap profile")
		}

		t.Logf("✓ Heap profile returned %d bytes", len(body))
	})

	t.Run("Pprof goroutine profile works", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/debug/pprof/goroutine")
		if err != nil {
			t.Fatalf("Failed to GET /debug/pprof/goroutine: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		if len(body) == 0 {
			t.Error("Expected non-empty goroutine profile")
		}

		t.Logf("✓ Goroutine profile returned %d bytes", len(body))
	})

	t.Run("Pprof CPU profile works", func(t *testing.T) {
		// CPU profile 需要 seconds 参数
		resp, err := http.Get(baseURL + "/debug/pprof/profile?seconds=1")
		if err != nil {
			t.Fatalf("Failed to GET /debug/pprof/profile: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		if len(body) == 0 {
			t.Error("Expected non-empty CPU profile")
		}

		t.Logf("✓ CPU profile returned %d bytes", len(body))
	})

	t.Run("Invalid endpoint returns 404", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/nonexistent")
		if err != nil {
			t.Fatalf("Failed to GET /api/v1/nonexistent: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}

		t.Logf("✓ Invalid endpoint correctly returned 404")
	})

	t.Run("Invalid method returns 405", func(t *testing.T) {
		resp, err := http.Post(baseURL+"/api/v1/metrics", "application/json", nil)
		if err != nil {
			t.Fatalf("Failed to POST /api/v1/metrics: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", resp.StatusCode)
		}

		t.Logf("✓ Invalid method correctly returned 405")
	})
}
