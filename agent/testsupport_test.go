package agent

import (
	"fmt"
	"net"
	"time"
)

// 测试可靠性基础设施。
//
// agent 通过全局单例 + 包级 Start/Stop 运作,且 Config.Port 不支持传 0(SetDefaults
// 会把 0 改写成 8563)。历史测试用硬编码/区间随机端口 + 固定 time.Sleep 等就绪,在共享/
// 有负载机器上会因端口冲突或就绪竞争而 flaky。以下 helper 提供可靠的起停:
//   - 向 OS 要真正空闲的端口,并对 bind 竞争重试;
//   - 轮询直到 collector 采到首批指标(collector 首次采集是异步的),取代固定 sleep。
//
// 这些 helper 都返回 error/bool(不调用 t.Fatal),因此既能用于普通测试,也能安全地用在
// gopter property 闭包里(失败时 return false)。

// freePort 向 OS 要一个当前空闲的 TCP 端口号。关闭监听后该端口短时间内极可能仍空闲;
// 配合 startOnFreePort 的重试可消除关闭与重新 bind 之间的 TOCTOU 竞争。
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

// startOnFreePort 在一个 OS 分配的空闲端口上启动 agent,对端口竞争重试。返回实际端口。
// 调用前会先 Stop() 清理可能残留的实例。调用方负责 defer Stop()。
func startOnFreePort(cfg Config) (int, error) {
	_ = Stop() // 清理上一个测试可能残留的实例(无实例时返回的 error 可忽略)
	var lastErr error
	for i := 0; i < 8; i++ {
		port, err := freePort()
		if err != nil {
			lastErr = err
			continue
		}
		cfg.Port = port
		if err := Start(cfg); err == nil {
			return port, nil
		} else {
			// 端口在 freePort 关闭后被他人抢占等竞争 → 换一个端口重试
			lastErr = err
			time.Sleep(5 * time.Millisecond)
		}
	}
	return 0, fmt.Errorf("startOnFreePort: 重试后仍无法启动: %w", lastErr)
}

// waitMetrics 轮询 GetMetrics 直到 collector 采到首批有效指标(见 collector.start 的异步
// "立即收集一次")。返回是否在超时内就绪。取代脆弱的固定 sleep。
func waitMetrics(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m := GetMetrics(); m != nil && m.Goroutines > 0 {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}
