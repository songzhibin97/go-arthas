//go:build unix || darwin

package agent

import (
	"syscall"
	"time"
)

// getCPUStats 获取当前 CPU 统计（Unix/Darwin 平台）
func (mc *metricsCollector) getCPUStats() cpuStats {
	var rusage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err != nil {
		// 如果获取失败，返回零值
		return cpuStats{
			userTime:   0,
			systemTime: 0,
			timestamp:  time.Now(),
		}
	}

	// 将 Timeval 转换为纳秒
	userTime := rusage.Utime.Sec*1e9 + int64(rusage.Utime.Usec)*1e3
	systemTime := rusage.Stime.Sec*1e9 + int64(rusage.Stime.Usec)*1e3

	return cpuStats{
		userTime:   userTime,
		systemTime: systemTime,
		timestamp:  time.Now(),
	}
}
