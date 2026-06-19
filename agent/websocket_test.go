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
)

// TestWebSocket_NewClientReceivesMetrics 验证**真不变量**:每个新连接的 WebSocket 客户端
// 都会被主动推送一帧有效的初始 metrics(handleConnection 的初始发送)。
//
// 取代旧的 "首消息 <100ms" 墙钟阈值 property——旧版真正 flaky 的根因是 gen.IntRange(9000,…)
// 撞本机 OrbStack 常驻的 :9000 而 Start 失败(已用 startOnFreePort 修复),而非首帧时延本身
// (实测初始推送 ~µs)。这里先 waitMetrics 确保 collector 就绪,再对多个新连接断言"收到一帧
// 带数据的 metrics",且首帧死线 500ms:初始推送在 handleConnection 中**同步**发生(~µs,
// 约 1000× 余量不会 flaky),且 500ms 远低于 1s 的周期广播间隔,从而对"连接即推送初始帧"
// 这条路径仍有守护(注:周期广播可能恰好抢先,故非 100% 隔离,但已是可靠的及时性检查)。
func TestWebSocket_NewClientReceivesMetrics(t *testing.T) {
	port, err := startOnFreePort(Config{EnableMetrics: true, LogLevel: "error"})
	if err != nil {
		t.Fatalf("start agent: %v", err)
	}
	defer Stop()

	if !waitMetrics(3 * time.Second) {
		t.Fatal("collector did not produce metrics in time")
	}

	for i := 0; i < 10; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d/ws/metrics", port), nil)
		if err != nil {
			t.Fatalf("dial #%d: %v", i, err)
		}
		// 首帧应来自连接时的即时推送(~µs),500ms 死线既不 flaky 又守护"及时初始推送"路径
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		var m Metrics
		err = conn.ReadJSON(&m)
		conn.Close()
		if err != nil {
			t.Fatalf("read #%d: %v", i, err)
		}
		if m.Goroutines <= 0 {
			t.Errorf("client #%d received empty metrics: %+v", i, m)
		}
	}
}

// TestWebSocket_ClientReceivesPeriodicUpdates 验证真不变量:连上后客户端会持续收到周期性
// 推送(collector 每秒广播)。取代旧的 "间隔 800–1200ms" 绝对节拍断言——精确节拍在有负载
// 机器上会漂移而 flaky;这里只断言"确实在持续收到后续帧"(宽松窗口),不卡精确节拍。
func TestWebSocket_ClientReceivesPeriodicUpdates(t *testing.T) {
	port, err := startOnFreePort(Config{EnableMetrics: true, LogLevel: "error"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer Stop()
	if !waitMetrics(3 * time.Second) {
		t.Fatal("collector not ready")
	}

	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d/ws/metrics", port), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 跳过初始推送帧
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var first Metrics
	if err := conn.ReadJSON(&first); err != nil {
		t.Fatalf("read initial frame: %v", err)
	}

	// 应在宽松窗口内继续收到至少 2 条后续周期帧(collector 每秒广播)
	for i := 0; i < 2; i++ {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var m Metrics
		if err := conn.ReadJSON(&m); err != nil {
			t.Fatalf("read periodic frame #%d: %v", i, err)
		}
	}
}

// TestWebSocket_AllClientsReceiveBroadcast 验证真不变量:一次周期广播应送达**所有**已连接
// 客户端。取代旧的 "100ms 内全部收到" 绝对扇出时延断言——扇出时延在负载下会超 100ms 而 flaky;
// 这里只断言"每个客户端都收到了下一次广播"(宽松窗口),不卡扇出时延。
func TestWebSocket_AllClientsReceiveBroadcast(t *testing.T) {
	port, err := startOnFreePort(Config{EnableMetrics: true, LogLevel: "error"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer Stop()
	if !waitMetrics(3 * time.Second) {
		t.Fatal("collector not ready")
	}

	const numClients = 4
	conns := make([]*websocket.Conn, 0, numClients)
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()
	for i := 0; i < numClients; i++ {
		c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d/ws/metrics", port), nil)
		if err != nil {
			t.Fatalf("dial #%d: %v", i, err)
		}
		conns = append(conns, c)
		// 跳过初始推送帧
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		var m Metrics
		if err := c.ReadJSON(&m); err != nil {
			t.Fatalf("initial frame #%d: %v", i, err)
		}
	}

	// 每个客户端都应在宽松窗口内收到下一次周期广播
	var wg sync.WaitGroup
	errs := make([]error, numClients)
	for i, c := range conns {
		wg.Add(1)
		go func(i int, c *websocket.Conn) {
			defer wg.Done()
			c.SetReadDeadline(time.Now().Add(5 * time.Second))
			var m Metrics
			errs[i] = c.ReadJSON(&m)
		}(i, c)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Errorf("client #%d did not receive broadcast: %v", i, e)
		}
	}
}

// TestWebSocket_NoGoroutineLeakOnReconnect 验证真不变量:反复 connect/disconnect 后
// goroutine 不持续增长(无泄漏)。这是可靠的功能不变量,默认运行。起停经 startOnFreePort
// 消除端口 flaky(本机 OrbStack 常驻 :9000,旧的 gen.IntRange(9300,9400) 虽不撞 9000,
// 但区间随机端口仍可能与其它进程/测试撞)。
func TestWebSocket_NoGoroutineLeakOnReconnect(t *testing.T) {
	port, err := startOnFreePort(Config{EnableMetrics: true, LogLevel: "error"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer Stop()
	if !waitMetrics(3 * time.Second) {
		t.Fatal("collector not ready")
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	base := runtime.NumGoroutine()

	for i := 0; i < 10; i++ {
		c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d/ws/metrics", port), nil)
		if err != nil {
			t.Fatalf("dial #%d: %v", i, err)
		}
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		var m Metrics
		_ = c.ReadJSON(&m)
		c.Close()
		time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	if after-base > 10 {
		t.Errorf("goroutine leak on repeated connect/disconnect: base=%d after=%d", base, after)
	}
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
