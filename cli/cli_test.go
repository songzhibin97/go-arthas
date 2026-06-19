package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_CLIErrorMessages(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// 生成无效的主机地址
	properties.Property("connection failures produce clear error messages with connection details",
		prop.ForAll(
			func(port int) bool {
				// 使用一个不存在的端口
				host := fmt.Sprintf("localhost:%d", port)
				cli := NewCLI(host)

				err := cli.Connect()
				if err == nil {
					// 如果连接成功，可能是端口碰巧被占用，跳过此测试
					return true
				}

				// 验证错误消息包含主机信息
				errMsg := err.Error()
				if !strings.Contains(errMsg, host) {
					t.Logf("Error message missing host details: %s", errMsg)
					return false
				}

				// 验证错误消息是描述性的
				if len(errMsg) < 10 {
					t.Logf("Error message too short: %s", errMsg)
					return false
				}

				return true
			},
			gen.IntRange(10000, 65000), // 生成随机端口号
		))

	properties.TestingRun(t)
}

func TestProperty_CLIExitCodes(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Go-Arthas Agent is running")
		case "/api/v1/metrics":
			metrics := Metrics{
				Timestamp:  time.Now(),
				Goroutines: 10,
				Memory: MemoryMetrics{
					HeapAlloc:  1024 * 1024,
					HeapInuse:  2048 * 1024,
					StackInuse: 512 * 1024,
				},
				CPU: CPUMetrics{
					UsagePercent: 5.5,
				},
				GC: GCMetrics{
					NumGC:     100,
					LastPause: 1 * time.Millisecond,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(metrics)
		case "/api/v1/info":
			info := SystemInfo{
				GoVersion: "go1.21.0",
				GOOS:      "linux",
				GOARCH:    "amd64",
				NumCPU:    4,
				ProcessID: 12345,
				StartTime: time.Now().Add(-1 * time.Hour),
				Uptime:    "1h0m0s",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(info)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// 从服务器 URL 中提取主机地址
	host := strings.TrimPrefix(server.URL, "http://")

	// 测试成功命令返回 0
	properties.Property("successful commands exit with code 0",
		prop.ForAll(
			func(cmd string) bool {
				var exitCode int

				switch cmd {
				case "connect":
					exitCode = runConnect([]string{host})
				case "metrics":
					exitCode = runMetrics([]string{"--host", host})
				case "info":
					exitCode = runInfo([]string{"--host", host})
				default:
					return true // 跳过未知命令
				}

				if exitCode != 0 {
					t.Logf("Command %s returned non-zero exit code: %d", cmd, exitCode)
					return false
				}

				return true
			},
			gen.OneConstOf("connect", "metrics", "info"),
		))

	// 测试失败命令返回非零退出码
	properties.Property("failed commands exit with non-zero code",
		prop.ForAll(
			func(cmd string) bool {
				invalidHost := "localhost:99999" // 无效端口
				var exitCode int

				switch cmd {
				case "connect":
					exitCode = runConnect([]string{invalidHost})
				case "metrics":
					exitCode = runMetrics([]string{"--host", invalidHost})
				case "info":
					exitCode = runInfo([]string{"--host", invalidHost})
				default:
					return true // 跳过未知命令
				}

				if exitCode == 0 {
					t.Logf("Command %s with invalid host returned zero exit code", cmd)
					return false
				}

				return true
			},
			gen.OneConstOf("connect", "metrics", "info"),
		))

	properties.TestingRun(t)
}

// 测试 CLI 基本功能
func TestCLI_Connect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Go-Arthas Agent is running")
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	cli := NewCLI(host)

	if err := cli.Connect(); err != nil {
		t.Errorf("Connect() failed: %v", err)
	}
}

func TestCLI_GetMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics := Metrics{
			Timestamp:  time.Now(),
			Goroutines: 10,
			Memory: MemoryMetrics{
				HeapAlloc: 1024 * 1024,
			},
			CPU: CPUMetrics{
				UsagePercent: 5.5,
			},
			GC: GCMetrics{
				NumGC: 100,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	cli := NewCLI(host)

	metrics, err := cli.GetMetrics()
	if err != nil {
		t.Fatalf("GetMetrics() failed: %v", err)
	}

	if metrics.Goroutines != 10 {
		t.Errorf("Expected 10 goroutines, got %d", metrics.Goroutines)
	}
}

func TestCLI_GetInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := SystemInfo{
			GoVersion: "go1.21.0",
			GOOS:      "linux",
			GOARCH:    "amd64",
			NumCPU:    4,
			ProcessID: 12345,
			StartTime: time.Now(),
			Uptime:    "1h0m0s",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	cli := NewCLI(host)

	info, err := cli.GetInfo()
	if err != nil {
		t.Fatalf("GetInfo() failed: %v", err)
	}

	if info.GoVersion != "go1.21.0" {
		t.Errorf("Expected go1.21.0, got %s", info.GoVersion)
	}
}

func TestCLI_GetProfile(t *testing.T) {
	profileData := []byte("fake profile data")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(profileData)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	cli := NewCLI(host)

	data, err := cli.GetProfile("heap", 0)
	if err != nil {
		t.Fatalf("GetProfile() failed: %v", err)
	}

	if string(data) != string(profileData) {
		t.Errorf("Expected profile data to match")
	}
}

func TestCLI_SaveProfile(t *testing.T) {
	cli := NewCLI("localhost:8563")
	profileData := []byte("test profile data")

	filename, err := cli.SaveProfile("heap", profileData)
	if err != nil {
		t.Fatalf("SaveProfile() failed: %v", err)
	}
	defer os.Remove(filename) // 清理测试文件

	// 验证文件存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Errorf("Profile file was not created: %s", filename)
	}

	// 验证文件内容
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read profile file: %v", err)
	}

	if string(content) != string(profileData) {
		t.Errorf("Profile file content mismatch")
	}
}

func TestCLI_ErrorHandling(t *testing.T) {
	// 测试连接到不存在的服务器
	cli := NewCLI("localhost:99999")

	if err := cli.Connect(); err == nil {
		t.Error("Expected error when connecting to non-existent server")
	}

	if _, err := cli.GetMetrics(); err == nil {
		t.Error("Expected error when getting metrics from non-existent server")
	}

	if _, err := cli.GetInfo(); err == nil {
		t.Error("Expected error when getting info from non-existent server")
	}
}

func TestRun_InvalidCommand(t *testing.T) {
	exitCode := Run([]string{"invalid-command"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid command")
	}
}

func TestRun_Help(t *testing.T) {
	exitCode := Run([]string{"help"})
	if exitCode != 0 {
		t.Error("Expected zero exit code for help command")
	}
}

func TestCLI_GetGoroutines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("min_wait") != "1" {
			t.Errorf("expected min_wait=1, got %q", r.URL.Query().Get("min_wait"))
		}
		dump := GoroutineDump{
			Timestamp:   time.Now(),
			Total:       3,
			StateCounts: map[string]int{"running": 1, "chan receive": 2},
			Suspected: []GoroutineInfo{
				{ID: 5, State: "chan receive", WaitMinutes: 7, Stack: "goroutine 5 [chan receive, 7 minutes]:"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dump)
	}))
	defer server.Close()

	cli := NewCLI(strings.TrimPrefix(server.URL, "http://"))
	dump, err := cli.GetGoroutines(false, 1)
	if err != nil {
		t.Fatalf("GetGoroutines: %v", err)
	}
	if dump.Total != 3 {
		t.Errorf("Total=%d want 3", dump.Total)
	}
	if len(dump.Suspected) != 1 || dump.Suspected[0].WaitMinutes != 7 {
		t.Errorf("suspected mismatch: %+v", dump.Suspected)
	}
	// 验证不会 panic 地格式化
	FormatGoroutineDump(dump, false)
}

func TestCLI_GetGoroutinesText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "text" {
			t.Errorf("expected format=text, got %q", r.URL.Query().Get("format"))
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "goroutine 1 [running]:\nmain.main()")
	}))
	defer server.Close()

	cli := NewCLI(strings.TrimPrefix(server.URL, "http://"))
	text, err := cli.GetGoroutinesText()
	if err != nil {
		t.Fatalf("GetGoroutinesText: %v", err)
	}
	if !strings.Contains(text, "goroutine 1") {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestCLI_FlightLifecycle(t *testing.T) {
	var started, stopped bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/trace/flight/start":
			started = true
			fmt.Fprint(w, `{"status":"started"}`)
		case "/api/v1/trace/flight/snapshot":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write([]byte("trace-bytes"))
		case "/api/v1/trace/flight/stop":
			stopped = true
			fmt.Fprint(w, `{"status":"stopped"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cli := NewCLI(strings.TrimPrefix(server.URL, "http://"))

	if err := cli.FlightStart(); err != nil {
		t.Fatalf("FlightStart: %v", err)
	}
	if !started {
		t.Error("start endpoint not called")
	}

	data, err := cli.FlightSnapshot()
	if err != nil {
		t.Fatalf("FlightSnapshot: %v", err)
	}
	if string(data) != "trace-bytes" {
		t.Errorf("snapshot data=%q", string(data))
	}

	fn, err := cli.SaveTrace(data)
	if err != nil {
		t.Fatalf("SaveTrace: %v", err)
	}
	defer os.Remove(fn)

	if err := cli.FlightStop(); err != nil {
		t.Fatalf("FlightStop: %v", err)
	}
	if !stopped {
		t.Error("stop endpoint not called")
	}
}

func TestCLI_FlightUnsupported(t *testing.T) {
	// 模拟 stub（Go < 1.25）返回 501
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "flight recorder requires Go 1.25+", http.StatusNotImplemented)
	}))
	defer server.Close()

	cli := NewCLI(strings.TrimPrefix(server.URL, "http://"))
	if err := cli.FlightStart(); err == nil {
		t.Error("expected error when agent returns 501")
	}
}
