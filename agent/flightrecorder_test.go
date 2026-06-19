package agent

import (
	"io"
	"net/http"
	"runtime"
	"testing"
	"time"
)

func TestFlightRecorder_Lifecycle(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{Port: 9710, EnableMetrics: false, LogLevel: "error"}
	if err := Start(config); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer Stop()
	time.Sleep(100 * time.Millisecond)

	base := "http://localhost:9710/api/v1/trace/flight"
	client := &http.Client{Timeout: 10 * time.Second}

	// snapshot before start → 409（两种构建一致：尚未运行）
	if resp, err := client.Get(base + "/snapshot"); err == nil {
		resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("snapshot before start: status=%d want 409", resp.StatusCode)
		}
	}

	// start：若返回 501 说明当前工具链 < Go 1.25（stub），跳过其余 real 断言
	resp, err := client.Post(base+"/start", "", nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	startStatus := resp.StatusCode
	resp.Body.Close()
	if startStatus == http.StatusNotImplemented {
		t.Skipf("flight recorder unsupported on %s (needs Go 1.25+)", runtime.Version())
	}
	if startStatus != http.StatusOK {
		t.Fatalf("start status=%d want 200", startStatus)
	}

	// 重复 start → 409
	if resp, err := client.Post(base+"/start", "", nil); err == nil {
		resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("double start: status=%d want 409", resp.StatusCode)
		}
	}

	// 产生一些可观测活动，让轨迹窗口有内容
	time.Sleep(200 * time.Millisecond)
	for i := 0; i < 2000; i++ {
		_ = make([]byte, 512)
	}
	runtime.GC()

	// snapshot → 200 且非空二进制 trace
	resp2, err := client.Get(base + "/snapshot")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("snapshot status=%d want 200", resp2.StatusCode)
	}
	body, _ := io.ReadAll(resp2.Body)
	if len(body) == 0 {
		t.Error("snapshot returned empty trace")
	}
	if ct := resp2.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("snapshot content-type=%q want application/octet-stream", ct)
	}

	// stop → 200
	resp3, err := client.Post(base+"/stop", "", nil)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("stop status=%d want 200", resp3.StatusCode)
	}

	// 重复 stop → 409
	if resp, err := client.Post(base+"/stop", "", nil); err == nil {
		resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("double stop: status=%d want 409", resp.StatusCode)
		}
	}

	// 方法校验：GET start → 405
	if resp, err := client.Get(base + "/start"); err == nil {
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET start: status=%d want 405", resp.StatusCode)
		}
	}
}
