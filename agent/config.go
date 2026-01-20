package agent

import (
	"fmt"
)

// Validate 验证配置的有效性
func (c *Config) Validate() error {
	// 验证端口范围
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", c.Port)
	}

	// 验证日志级别
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if c.LogLevel != "" && !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level %q: must be one of debug, info, warn, error", c.LogLevel)
	}

	return nil
}

// SetDefaults 设置默认配置值
func (c *Config) SetDefaults() {
	if c.Port == 0 {
		c.Port = 8563
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	// EnablePprof 和 EnableMetrics 默认为 false，用户需要显式启用
}
