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
