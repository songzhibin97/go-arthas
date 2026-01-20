//go:build !unix && !darwin && !windows

package agent

import (
	"time"
)

// getCPUStats 获取当前 CPU 统计（不支持的平台）
// 对于不支持的平台，返回零值，让 collector 优雅降级
func (mc *metricsCollector) getCPUStats() cpuStats {
	return cpuStats{
		userTime:   0,
		systemTime: 0,
		timestamp:  time.Now(),
	}
}
