package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// CLI 客户端结构
type CLI struct {
	host   string
	client *http.Client
}

// NewCLI 创建新的 CLI 客户端
func NewCLI(host string) *CLI {
	return &CLI{
		host: host,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Connect 测试与 Agent 的连接
func (c *CLI) Connect() error {
	url := fmt.Sprintf("http://%s/", c.host)
	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent returned status %d", resp.StatusCode)
	}

	return nil
}

// GetMetrics 获取运行时指标
func (c *CLI) GetMetrics() (*Metrics, error) {
	url := fmt.Sprintf("http://%s/api/v1/metrics", c.host)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics from %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
	}

	var metrics Metrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return nil, fmt.Errorf("failed to decode metrics: %w", err)
	}

	return &metrics, nil
}

// GetInfo 获取系统信息
func (c *CLI) GetInfo() (*SystemInfo, error) {
	url := fmt.Sprintf("http://%s/api/v1/info", c.host)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch info from %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
	}

	var info SystemInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode info: %w", err)
	}

	return &info, nil
}

// GetProfile 获取性能分析数据
func (c *CLI) GetProfile(profileType string, duration int) ([]byte, error) {
	var url string
	if profileType == "cpu" {
		url = fmt.Sprintf("http://%s/debug/pprof/profile?seconds=%d", c.host, duration)
	} else {
		url = fmt.Sprintf("http://%s/debug/pprof/%s", c.host, profileType)
	}

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s profile from %s: %w", profileType, c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile data: %w", err)
	}

	return data, nil
}

// SaveProfile 保存性能分析数据到文件
func (c *CLI) SaveProfile(profileType string, data []byte) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_profile_%s.pprof", profileType, timestamp)

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save profile to %s: %w", filename, err)
	}

	return filename, nil
}
