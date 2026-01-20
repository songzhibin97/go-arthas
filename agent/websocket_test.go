package agent

import (
	"encoding/json"
	"fmt"
	"net/url"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_WebSocketImmediateMetricsSend(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("client receives metrics immediately on connection",
		prop.ForAll(
			func(port int) bool {
				// 确保清理
				Stop()
				time.Sleep(10 * time.Millisecond)

				// 启动 agent
				config := Config{
					Port:          port,
					EnablePprof:   false,
					EnableMetrics: true,
					LogLevel:      "error",
				}

				err := Start(config)
				if err != nil {
					return false
				}
				defer Stop()

				// 等待 agent 启动和收集器收集一次指标
				time.Sleep(200 * time.Millisecond)

				// 连接 WebSocket
				wsURL := fmt.Sprintf("ws://localhost:%d/ws/metrics", port)
				u, _ := url.Parse(wsURL)
				conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
				if err != nil {
					return false
				}
				defer conn.Close()

				// 设置读取超时
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

				// 读取第一条消息（应该立即收到）
				start := time.Now()
				var metrics Metrics
				err = conn.ReadJSON(&metrics)
				elapsed := time.Since(start)

				if err != nil {
					return false
				}

				// 验证在 100ms 内收到
				return elapsed <= 100*time.Millisecond && metrics.Goroutines > 0
			},
			gen.IntRange(9000, 9100),
		))

	properties.TestingRun(t)
}

func TestProperty_WebSocketPeriodicUpdates(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20 // 减少测试次数以加快测试速度

	properties := gopter.NewProperties(parameters)

	properties.Property("client receives updates every 1 second",
		prop.ForAll(
			func(port int) bool {
				// 确保清理
				Stop()
				time.Sleep(10 * time.Millisecond)

				// 启动 agent
				config := Config{
					Port:          port,
					EnablePprof:   false,
					EnableMetrics: true,
					LogLevel:      "error",
				}

				err := Start(config)
				if err != nil {
					return false
				}
				defer Stop()

				// 等待 agent 启动
				time.Sleep(200 * time.Millisecond)

				// 连接 WebSocket
				wsURL := fmt.Sprintf("ws://localhost:%d/ws/metrics", port)
				u, _ := url.Parse(wsURL)
				conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
				if err != nil {
					return false
				}
				defer conn.Close()

				// 跳过第一条消息（立即发送的）
				conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				var firstMetrics Metrics
				err = conn.ReadJSON(&firstMetrics)
				if err != nil {
					return false
				}

				// 读取后续2条消息并测量间隔（减少到2条以加快测试）
				var timestamps []time.Time
				for i := 0; i < 2; i++ {
					conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
					var metrics Metrics
					err = conn.ReadJSON(&metrics)
					if err != nil {
						return false
					}
					timestamps = append(timestamps, time.Now())
				}

				// 验证间隔约为 1 秒（±200ms 容差，考虑系统调度延迟）
				if len(timestamps) >= 2 {
					interval := timestamps[1].Sub(timestamps[0])
					if interval < 800*time.Millisecond || interval > 1200*time.Millisecond {
						return false
					}
				}

				return true
			},
			gen.IntRange(9100, 9200),
		))

	properties.TestingRun(t)
}

func TestProperty_WebSocketBroadcastTiming(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("all clients receive broadcast within 100ms",
		prop.ForAll(
			func(port int, numClients int) bool {
				// 确保清理
				Stop()
				time.Sleep(10 * time.Millisecond)

				// 启动 agent
				config := Config{
					Port:          port,
					EnablePprof:   false,
					EnableMetrics: true,
					LogLevel:      "error",
				}

				err := Start(config)
				if err != nil {
					return false
				}
				defer Stop()

				// 等待 agent 启动（减少等待时间）
				time.Sleep(200 * time.Millisecond)

				// 连接多个 WebSocket 客户端
				var conns []*websocket.Conn
				wsURL := fmt.Sprintf("ws://localhost:%d/ws/metrics", port)
				u, _ := url.Parse(wsURL)

				for i := 0; i < numClients; i++ {
					conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
					if err != nil {
						// 清理已连接的客户端
						for _, c := range conns {
							c.Close()
						}
						return false
					}
					conns = append(conns, conn)
				}

				// 清理所有连接
				defer func() {
					for _, conn := range conns {
						conn.Close()
					}
				}()

				// 跳过初始消息
				for _, conn := range conns {
					conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
					var metrics Metrics
					conn.ReadJSON(&metrics)
				}

				// 等待下一次广播并测量所有客户端接收时间
				var receiveTimes []time.Time
				var mu sync.Mutex
				var wg sync.WaitGroup

				for _, conn := range conns {
					wg.Add(1)
					go func(c *websocket.Conn) {
						defer wg.Done()
						c.SetReadDeadline(time.Now().Add(2 * time.Second))
						var metrics Metrics
						err := c.ReadJSON(&metrics)
						if err == nil {
							mu.Lock()
							receiveTimes = append(receiveTimes, time.Now())
							mu.Unlock()
						}
					}(conn)
				}

				wg.Wait()

				// 验证所有客户端都收到了消息
				if len(receiveTimes) != numClients {
					return false
				}

				// 验证所有接收时间在 100ms 内
				if len(receiveTimes) > 1 {
					minTime := receiveTimes[0]
					maxTime := receiveTimes[0]
					for _, t := range receiveTimes {
						if t.Before(minTime) {
							minTime = t
						}
						if t.After(maxTime) {
							maxTime = t
						}
					}
					spread := maxTime.Sub(minTime)
					if spread > 100*time.Millisecond {
						return false
					}
				}

				return true
			},
			gen.IntRange(9200, 9300),
			gen.IntRange(2, 5), // 2-5 个客户端
		))

	properties.TestingRun(t)
}

func TestProperty_ResourceLeakPrevention(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20 // 减少测试次数以加快测试速度

	properties := gopter.NewProperties(parameters)

	properties.Property("no goroutine or memory leaks on repeated connect/disconnect",
		prop.ForAll(
			func(port int, iterations int) bool {
				// 确保清理
				Stop()
				time.Sleep(10 * time.Millisecond)

				// 启动 agent
				config := Config{
					Port:          port,
					EnablePprof:   false,
					EnableMetrics: true,
					LogLevel:      "error",
				}

				err := Start(config)
				if err != nil {
					return false
				}
				defer Stop()

				// 等待 agent 启动
				time.Sleep(200 * time.Millisecond)

				// 记录初始 goroutine 数量
				runtime.GC()
				time.Sleep(50 * time.Millisecond) // 减少等待时间
				initialGoroutines := runtime.NumGoroutine()

				// 重复连接和断开
				wsURL := fmt.Sprintf("ws://localhost:%d/ws/metrics", port)
				u, _ := url.Parse(wsURL)

				for i := 0; i < iterations; i++ {
					conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
					if err != nil {
						return false
					}

					// 读取一条消息
					conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
					var metrics Metrics
					conn.ReadJSON(&metrics)

					// 关闭连接
					conn.Close()

					// 等待清理（减少等待时间）
					time.Sleep(20 * time.Millisecond)
				}

				// 等待清理完成（减少等待时间）
				time.Sleep(200 * time.Millisecond)
				runtime.GC()
				time.Sleep(100 * time.Millisecond)

				// 检查 goroutine 数量
				finalGoroutines := runtime.NumGoroutine()

				// 允许少量增长（±10 个 goroutine，考虑到系统后台任务）
				goroutineDiff := finalGoroutines - initialGoroutines
				if goroutineDiff > 10 {
					return false
				}

				return true
			},
			gen.IntRange(9300, 9400),
			gen.IntRange(3, 5), // 减少迭代次数：3-5 次迭代
		))

	properties.TestingRun(t)
}

// 单元测试：测试 WebSocket 基本连接
func TestWebSocket_BasicConnection(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          9500,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 启动
	time.Sleep(200 * time.Millisecond)

	// 连接 WebSocket
	wsURL := "ws://localhost:9500/ws/metrics"
	u, _ := url.Parse(wsURL)
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// 读取消息
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var metrics Metrics
	err = conn.ReadJSON(&metrics)
	if err != nil {
		t.Fatalf("Failed to read metrics: %v", err)
	}

	// 验证指标
	if metrics.Goroutines <= 0 {
		t.Errorf("Expected positive goroutine count, got %d", metrics.Goroutines)
	}
}

// 单元测试：测试多个客户端
func TestWebSocket_MultipleClients(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          9501,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 启动
	time.Sleep(200 * time.Millisecond)

	// 连接多个客户端
	numClients := 3
	var conns []*websocket.Conn
	wsURL := "ws://localhost:9501/ws/metrics"
	u, _ := url.Parse(wsURL)

	for i := 0; i < numClients; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		conns = append(conns, conn)
	}

	// 清理
	defer func() {
		for _, conn := range conns {
			conn.Close()
		}
	}()

	// 验证所有客户端都能接收消息
	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var metrics Metrics
		err = conn.ReadJSON(&metrics)
		if err != nil {
			t.Errorf("Client %d failed to read metrics: %v", i, err)
		}
	}
}

// 单元测试：测试客户端断开
func TestWebSocket_ClientDisconnect(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          9502,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 启动
	time.Sleep(200 * time.Millisecond)

	// 连接客户端
	wsURL := "ws://localhost:9502/ws/metrics"
	u, _ := url.Parse(wsURL)
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// 读取一条消息
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var metrics Metrics
	err = conn.ReadJSON(&metrics)
	if err != nil {
		t.Fatalf("Failed to read metrics: %v", err)
	}

	// 关闭连接
	conn.Close()

	// 等待清理
	time.Sleep(200 * time.Millisecond)

	// Agent 应该仍在运行
	if globalAgent == nil || !globalAgent.running {
		t.Fatal("Agent should still be running after client disconnect")
	}
}

// 单元测试：测试禁用指标时的 WebSocket
func TestWebSocket_DisabledMetrics(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          9503,
		EnablePprof:   false,
		EnableMetrics: false, // 禁用指标
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 启动
	time.Sleep(100 * time.Millisecond)

	// 尝试连接 WebSocket（应该失败或返回 503）
	wsURL := "ws://localhost:9503/ws/metrics"
	u, _ := url.Parse(wsURL)
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)

	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail when metrics are disabled")
	}

	if resp != nil && resp.StatusCode != 503 {
		t.Errorf("Expected status 503, got %d", resp.StatusCode)
	}
}

// 单元测试：测试 JSON 格式
func TestWebSocket_JSONFormat(t *testing.T) {
	// 确保清理
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          9504,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	// 等待 agent 启动
	time.Sleep(200 * time.Millisecond)

	// 连接 WebSocket
	wsURL := "ws://localhost:9504/ws/metrics"
	u, _ := url.Parse(wsURL)
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// 读取原始消息
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	// 验证是有效的 JSON
	var metrics Metrics
	err = json.Unmarshal(message, &metrics)
	if err != nil {
		t.Fatalf("Invalid JSON format: %v", err)
	}

	// 验证必需字段
	if metrics.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if metrics.Goroutines <= 0 {
		t.Error("Goroutines should be positive")
	}
}

// 基准测试：WebSocket 连接性能
func BenchmarkWebSocket_Connect(b *testing.B) {
	// 启动 agent
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          9600,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		b.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(200 * time.Millisecond)

	wsURL := "ws://localhost:9600/ws/metrics"
	u, _ := url.Parse(wsURL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}
		conn.Close()
	}
}

// 基准测试：WebSocket 消息接收性能
func BenchmarkWebSocket_ReceiveMetrics(b *testing.B) {
	// 启动 agent
	Stop()
	time.Sleep(10 * time.Millisecond)

	config := Config{
		Port:          9601,
		EnablePprof:   false,
		EnableMetrics: true,
		LogLevel:      "error",
	}

	err := Start(config)
	if err != nil {
		b.Fatalf("Failed to start agent: %v", err)
	}
	defer Stop()

	time.Sleep(200 * time.Millisecond)

	// 连接 WebSocket
	wsURL := "ws://localhost:9601/ws/metrics"
	u, _ := url.Parse(wsURL)
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var metrics Metrics
		err = conn.ReadJSON(&metrics)
		if err != nil {
			b.Fatalf("Failed to read metrics: %v", err)
		}
	}
}
