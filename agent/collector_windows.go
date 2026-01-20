//go:build windows

package agent

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetProcessTimes = kernel32.NewProc("GetProcessTimes")
)

// getCPUStats 获取当前 CPU 统计（Windows 平台）
func (mc *metricsCollector) getCPUStats() cpuStats {
	handle, err := syscall.GetCurrentProcess()
	if err != nil {
		return cpuStats{
			userTime:   0,
			systemTime: 0,
			timestamp:  time.Now(),
		}
	}

	var creationTime, exitTime, kernelTime, userTime syscall.Filetime
	ret, _, _ := procGetProcessTimes.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&creationTime)),
		uintptr(unsafe.Pointer(&exitTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)

	if ret == 0 {
		return cpuStats{
			userTime:   0,
			systemTime: 0,
			timestamp:  time.Now(),
		}
	}

	// FILETIME 是 100 纳秒为单位，转换为纳秒
	userNs := int64(userTime.HighDateTime)<<32 | int64(userTime.LowDateTime)
	kernelNs := int64(kernelTime.HighDateTime)<<32 | int64(kernelTime.LowDateTime)

	return cpuStats{
		userTime:   userNs * 100, // 转换为纳秒
		systemTime: kernelNs * 100,
		timestamp:  time.Now(),
	}
}
