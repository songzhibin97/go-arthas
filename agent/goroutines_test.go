package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestParseGoroutineHeader(t *testing.T) {
	cases := []struct {
		line  string
		id    int
		state string
		wait  int
		ok    bool
	}{
		{"goroutine 1 [running]:", 1, "running", 0, true},
		{"goroutine 42 [chan receive, 5 minutes]:", 42, "chan receive", 5, true},
		{"goroutine 7 [select]:", 7, "select", 0, true},
		{"goroutine 9 [semacquire, 1 minutes]:", 9, "semacquire", 1, true},
		{"goroutine 3 [IO wait]:", 3, "IO wait", 0, true},
		{"not a goroutine line", 0, "", 0, false},
		{"goroutine x [running]:", 0, "", 0, false},
		{"goroutine 5 running:", 0, "", 0, false}, // 缺少方括号
	}
	for _, c := range cases {
		gi, ok := parseGoroutineHeader(c.line)
		if ok != c.ok {
			t.Errorf("%q: ok=%v want %v", c.line, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if gi.ID != c.id || gi.State != c.state || gi.WaitMinutes != c.wait {
			t.Errorf("%q: got {id:%d state:%q wait:%d} want {id:%d state:%q wait:%d}",
				c.line, gi.ID, gi.State, gi.WaitMinutes, c.id, c.state, c.wait)
		}
	}
}

func TestParseGoroutineDump(t *testing.T) {
	raw := `goroutine 1 [running]:
main.main()
	/app/main.go:10 +0x20

goroutine 2 [chan receive, 5 minutes]:
main.worker()
	/app/main.go:20 +0x40

goroutine 3 [select]:
main.loop()
	/app/main.go:30 +0x60

goroutine 4 [chan receive, 8 minutes]:
main.worker2()
	/app/main.go:40 +0x80
`
	dump := parseGoroutineDump([]byte(raw), false, 1, time.Now())
	if dump.Total != 4 {
		t.Errorf("Total=%d want 4", dump.Total)
	}
	if dump.StateCounts["chan receive"] != 2 {
		t.Errorf("chan receive count=%d want 2", dump.StateCounts["chan receive"])
	}
	if dump.StateCounts["running"] != 1 || dump.StateCounts["select"] != 1 {
		t.Errorf("unexpected state counts: %+v", dump.StateCounts)
	}
	if len(dump.Suspected) != 2 {
		t.Fatalf("suspected=%d want 2", len(dump.Suspected))
	}
	// 按时长降序：8min 在前
	if dump.Suspected[0].WaitMinutes != 8 || dump.Suspected[1].WaitMinutes != 5 {
		t.Errorf("suspected order wrong: %d,%d", dump.Suspected[0].WaitMinutes, dump.Suspected[1].WaitMinutes)
	}
	// suspected 始终带栈
	if !strings.Contains(dump.Suspected[0].Stack, "worker2") {
		t.Errorf("suspected stack missing worker2: %q", dump.Suspected[0].Stack)
	}
	// 未请求 stacks 时 Goroutines 为空
	if len(dump.Goroutines) != 0 {
		t.Errorf("Goroutines should be empty when includeStacks=false, got %d", len(dump.Goroutines))
	}
}

func TestParseGoroutineDump_SuspectDisabledAndStacks(t *testing.T) {
	raw := `goroutine 2 [chan receive, 5 minutes]:
main.worker()
	/app/main.go:20 +0x40
`
	dump := parseGoroutineDump([]byte(raw), true, 0, time.Now())
	if len(dump.Suspected) != 0 {
		t.Errorf("suspected should be empty when suspectMinWait=0, got %d", len(dump.Suspected))
	}
	if len(dump.Goroutines) != 1 || dump.Goroutines[0].Stack == "" {
		t.Errorf("includeStacks should populate Goroutines with stack, got %+v", dump.Goroutines)
	}
}

func TestCaptureGoroutineDump(t *testing.T) {
	dump := captureGoroutineDump(false, 1)
	if dump.Total <= 0 {
		t.Errorf("Total=%d want >0", dump.Total)
	}
	if len(dump.StateCounts) == 0 {
		t.Error("expected non-empty StateCounts")
	}
}

func TestHandleGoroutines(t *testing.T) {
	Stop()
	time.Sleep(10 * time.Millisecond)

	// 注意：故意禁用指标，验证 goroutine 诊断独立于 collector
	config := Config{Port: 9700, EnableMetrics: false, LogLevel: "error"}
	if err := Start(config); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer Stop()
	time.Sleep(100 * time.Millisecond)

	// JSON 响应
	resp, err := http.Get("http://localhost:9700/api/v1/goroutines")
	if err != nil {
		t.Fatalf("GET json: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.StatusCode)
	}
	var dump GoroutineDump
	if err := json.NewDecoder(resp.Body).Decode(&dump); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dump.Total <= 0 {
		t.Errorf("Total=%d want >0", dump.Total)
	}
	if len(dump.StateCounts) == 0 {
		t.Error("expected non-empty StateCounts in response")
	}

	// text 响应
	resp2, err := http.Get("http://localhost:9700/api/v1/goroutines?format=text")
	if err != nil {
		t.Fatalf("GET text: %v", err)
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body), "goroutine ") {
		t.Errorf("text output missing 'goroutine ' prefix")
	}

	// 非法方法
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:9700/api/v1/goroutines", nil)
	resp3, err := http.DefaultClient.Do(req)
	if err == nil {
		defer resp3.Body.Close()
		if resp3.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST status=%d want 405", resp3.StatusCode)
		}
	}
}
