package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCheckpoint9_CLIEndToEnd 验证 CLI 端到端功能
// 这是一个集成测试，启动真实的 agent 并测试所有 CLI 命令
func TestCheckpoint9_CLIEndToEnd(t *testing.T) {
	// 启动 agent
	config := Config{
		Port:          18563, // 使用不同的端口避免冲突
		EnablePprof:   true,
		EnableMetrics: true,
		LogLevel:      "info",
	}

	if err := Start(config); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 完全启动
	time.Sleep(500 * time.Millisecond)

	// 构建 CLI 二进制文件
	cliBinary := buildCLI(t)
	defer os.Remove(cliBinary)

	host := fmt.Sprintf("localhost:%d", config.Port)

	t.Run("Connect Command Success", func(t *testing.T) {
		output, exitCode := runCLI(t, cliBinary, "connect", host)
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}
		if !strings.Contains(output, "Successfully connected") {
			t.Errorf("Expected success message, got: %s", output)
		}
	})

	t.Run("Metrics Command Success", func(t *testing.T) {
		output, exitCode := runCLI(t, cliBinary, "metrics", "--host", host)
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d. Output: %s", exitCode, output)
		}
		// 验证输出包含关键指标
		if !strings.Contains(output, "Goroutines") {
			t.Errorf("Expected metrics output to contain 'Goroutines', got: %s", output)
		}
		if !strings.Contains(output, "Memory") {
			t.Errorf("Expected metrics output to contain 'Memory', got: %s", output)
		}
	})

	t.Run("Info Command Success", func(t *testing.T) {
		output, exitCode := runCLI(t, cliBinary, "info", "--host", host)
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d. Output: %s", exitCode, output)
		}
		// 验证输出包含系统信息
		if !strings.Contains(output, "Go Version") {
			t.Errorf("Expected info output to contain 'Go Version', got: %s", output)
		}
		if !strings.Contains(output, "Operating System") {
			t.Errorf("Expected info output to contain 'Operating System', got: %s", output)
		}
	})

	t.Run("Profile CPU Command Success", func(t *testing.T) {
		// CPU profile 需要较长时间，使用较短的 duration
		output, exitCode := runCLI(t, cliBinary, "profile", "cpu", "--host", host, "--duration", "1")
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d. Output: %s", exitCode, output)
		}
		if !strings.Contains(output, "Profile saved to") {
			t.Errorf("Expected profile save message, got: %s", output)
		}
		if !strings.Contains(output, "cpu_profile_") {
			t.Errorf("Expected CPU profile filename, got: %s", output)
		}

		// 清理生成的 profile 文件
		cleanupProfileFiles(t, "cpu_profile_")
	})

	t.Run("Profile Heap Command Success", func(t *testing.T) {
		output, exitCode := runCLI(t, cliBinary, "profile", "heap", "--host", host)
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d. Output: %s", exitCode, output)
		}
		if !strings.Contains(output, "Profile saved to") {
			t.Errorf("Expected profile save message, got: %s", output)
		}
		if !strings.Contains(output, "heap_profile_") {
			t.Errorf("Expected heap profile filename, got: %s", output)
		}

		// 清理生成的 profile 文件
		cleanupProfileFiles(t, "heap_profile_")
	})

	t.Run("Profile Goroutine Command Success", func(t *testing.T) {
		output, exitCode := runCLI(t, cliBinary, "profile", "goroutine", "--host", host)
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d. Output: %s", exitCode, output)
		}
		if !strings.Contains(output, "Profile saved to") {
			t.Errorf("Expected profile save message, got: %s", output)
		}
		if !strings.Contains(output, "goroutine_profile_") {
			t.Errorf("Expected goroutine profile filename, got: %s", output)
		}

		// 清理生成的 profile 文件
		cleanupProfileFiles(t, "goroutine_profile_")
	})

	// 测试错误情况
	t.Run("Connect Command - Agent Not Running", func(t *testing.T) {
		wrongHost := "localhost:19999" // 不存在的端口
		output, exitCode := runCLI(t, cliBinary, "connect", wrongHost)
		if exitCode == 0 {
			t.Errorf("Expected non-zero exit code for failed connection, got 0")
		}
		if !strings.Contains(output, "failed to connect") && !strings.Contains(output, "Error") {
			t.Errorf("Expected error message, got: %s", output)
		}
		if !strings.Contains(output, wrongHost) {
			t.Errorf("Expected error to include connection details (%s), got: %s", wrongHost, output)
		}
	})

	t.Run("Metrics Command - Wrong Port", func(t *testing.T) {
		wrongHost := "localhost:19999"
		output, exitCode := runCLI(t, cliBinary, "metrics", "--host", wrongHost)
		if exitCode == 0 {
			t.Errorf("Expected non-zero exit code for failed connection, got 0")
		}
		if !strings.Contains(output, "Error") {
			t.Errorf("Expected error message, got: %s", output)
		}
	})

	t.Run("Info Command - Wrong Port", func(t *testing.T) {
		wrongHost := "localhost:19999"
		output, exitCode := runCLI(t, cliBinary, "info", "--host", wrongHost)
		if exitCode == 0 {
			t.Errorf("Expected non-zero exit code for failed connection, got 0")
		}
		if !strings.Contains(output, "Error") {
			t.Errorf("Expected error message, got: %s", output)
		}
	})

	t.Run("Profile Command - Wrong Port", func(t *testing.T) {
		wrongHost := "localhost:19999"
		output, exitCode := runCLI(t, cliBinary, "profile", "heap", "--host", wrongHost)
		if exitCode == 0 {
			t.Errorf("Expected non-zero exit code for failed connection, got 0")
		}
		if !strings.Contains(output, "Error") {
			t.Errorf("Expected error message, got: %s", output)
		}
	})

	t.Run("Invalid Command", func(t *testing.T) {
		output, exitCode := runCLI(t, cliBinary, "invalid-command")
		if exitCode == 0 {
			t.Errorf("Expected non-zero exit code for invalid command, got 0")
		}
		if !strings.Contains(output, "Unknown command") {
			t.Errorf("Expected 'Unknown command' message, got: %s", output)
		}
	})

	t.Run("Profile Invalid Type", func(t *testing.T) {
		output, exitCode := runCLI(t, cliBinary, "profile", "invalid-type", "--host", host)
		if exitCode == 0 {
			t.Errorf("Expected non-zero exit code for invalid profile type, got 0")
		}
		if !strings.Contains(output, "invalid profile type") {
			t.Errorf("Expected 'invalid profile type' message, got: %s", output)
		}
	})
}

// buildCLI 构建 CLI 二进制文件用于测试
func buildCLI(t *testing.T) string {
	t.Helper()

	// 创建临时目录
	tmpDir := t.TempDir()
	cliBinary := filepath.Join(tmpDir, "go-arthas-test")

	// 构建 CLI
	cmd := exec.Command("go", "build", "-o", cliBinary, "../cmd/go-arthas")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, string(output))
	}

	return cliBinary
}

// runCLI 运行 CLI 命令并返回输出和退出码
func runCLI(t *testing.T, binary string, args ...string) (string, int) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Logf("Command execution error: %v", err)
			exitCode = 1
		}
	}

	return string(output), exitCode
}

// cleanupProfileFiles 清理测试生成的 profile 文件
func cleanupProfileFiles(t *testing.T, prefix string) {
	t.Helper()

	files, err := filepath.Glob(prefix + "*.pprof")
	if err != nil {
		t.Logf("Warning: failed to glob profile files: %v", err)
		return
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			t.Logf("Warning: failed to remove profile file %s: %v", file, err)
		}
	}
}
