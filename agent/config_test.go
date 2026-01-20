package agent

import (
	"strings"
	"testing"
)

// TestConfig_Validate_ValidConfig 测试有效配置的验证
func TestConfig_Validate_ValidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "valid config with all fields",
			config: Config{
				Port:          8563,
				EnablePprof:   true,
				EnableMetrics: true,
				LogLevel:      "info",
			},
		},
		{
			name: "valid config with debug log level",
			config: Config{
				Port:     9000,
				LogLevel: "debug",
			},
		},
		{
			name: "valid config with warn log level",
			config: Config{
				Port:     8080,
				LogLevel: "warn",
			},
		},
		{
			name: "valid config with error log level",
			config: Config{
				Port:     3000,
				LogLevel: "error",
			},
		},
		{
			name: "valid config with empty log level",
			config: Config{
				Port:     8563,
				LogLevel: "",
			},
		},
		{
			name: "valid config with minimum port",
			config: Config{
				Port:     1,
				LogLevel: "info",
			},
		},
		{
			name: "valid config with maximum port",
			config: Config{
				Port:     65535,
				LogLevel: "info",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != nil {
				t.Errorf("Expected valid config, got error: %v", err)
			}
		})
	}
}

// TestConfig_Validate_InvalidPort 测试无效端口的验证
func TestConfig_Validate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port too low", 0},
		{"port negative", -1},
		{"port too high", 65536},
		{"port way too high", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Port:     tt.port,
				LogLevel: "info",
			}
			err := config.Validate()
			if err == nil {
				t.Errorf("Expected error for port %d, got nil", tt.port)
			}
			if !strings.Contains(err.Error(), "invalid port") {
				t.Errorf("Expected 'invalid port' error, got: %v", err)
			}
		})
	}
}

// TestConfig_Validate_InvalidLogLevel 测试无效日志级别的验证
func TestConfig_Validate_InvalidLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{"invalid level", "invalid"},
		{"uppercase", "INFO"},
		{"mixed case", "Debug"},
		{"typo", "infoo"},
		{"empty string is valid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Port:     8563,
				LogLevel: tt.logLevel,
			}
			err := config.Validate()

			// 空字符串是有效的（会使用默认值）
			if tt.logLevel == "" {
				if err != nil {
					t.Errorf("Expected no error for empty log level, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("Expected error for log level %q, got nil", tt.logLevel)
			}
			if !strings.Contains(err.Error(), "invalid log level") {
				t.Errorf("Expected 'invalid log level' error, got: %v", err)
			}
		})
	}
}

// TestConfig_SetDefaults 测试设置默认配置值
func TestConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name            string
		input           Config
		expectedPort    int
		expectedLevel   string
		expectedPprof   bool
		expectedMetrics bool
	}{
		{
			name:            "all defaults",
			input:           Config{},
			expectedPort:    8563,
			expectedLevel:   "info",
			expectedPprof:   false,
			expectedMetrics: false,
		},
		{
			name: "custom port, default others",
			input: Config{
				Port: 9000,
			},
			expectedPort:    9000,
			expectedLevel:   "info",
			expectedPprof:   false,
			expectedMetrics: false,
		},
		{
			name: "custom log level, default others",
			input: Config{
				LogLevel: "debug",
			},
			expectedPort:    8563,
			expectedLevel:   "debug",
			expectedPprof:   false,
			expectedMetrics: false,
		},
		{
			name: "all custom values",
			input: Config{
				Port:          3000,
				EnablePprof:   true,
				EnableMetrics: true,
				LogLevel:      "error",
			},
			expectedPort:    3000,
			expectedLevel:   "error",
			expectedPprof:   true,
			expectedMetrics: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.input
			config.SetDefaults()

			if config.Port != tt.expectedPort {
				t.Errorf("Expected port=%d, got %d", tt.expectedPort, config.Port)
			}
			if config.LogLevel != tt.expectedLevel {
				t.Errorf("Expected log level=%q, got %q", tt.expectedLevel, config.LogLevel)
			}
			if config.EnablePprof != tt.expectedPprof {
				t.Errorf("Expected EnablePprof=%v, got %v", tt.expectedPprof, config.EnablePprof)
			}
			if config.EnableMetrics != tt.expectedMetrics {
				t.Errorf("Expected EnableMetrics=%v, got %v", tt.expectedMetrics, config.EnableMetrics)
			}
		})
	}
}

// TestConfig_ValidateAndSetDefaults 测试验证和设置默认值的组合
func TestConfig_ValidateAndSetDefaults(t *testing.T) {
	config := Config{}

	// 设置默认值
	config.SetDefaults()

	// 验证配置
	err := config.Validate()
	if err != nil {
		t.Errorf("Default config should be valid, got error: %v", err)
	}

	// 验证默认值已设置
	if config.Port != 8563 {
		t.Errorf("Expected default port=8563, got %d", config.Port)
	}
	if config.LogLevel != "info" {
		t.Errorf("Expected default log level=info, got %q", config.LogLevel)
	}
}

// TestConfig_Validate_MultipleErrors 测试配置有多个错误时的行为
func TestConfig_Validate_MultipleErrors(t *testing.T) {
	// 端口和日志级别都无效
	config := Config{
		Port:     0,
		LogLevel: "invalid",
	}

	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid config, got nil")
	}

	// 验证返回第一个错误（端口错误）
	if !strings.Contains(err.Error(), "invalid port") {
		t.Errorf("Expected port error first, got: %v", err)
	}
}
