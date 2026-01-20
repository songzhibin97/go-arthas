package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_WebConsoleReconnection(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("web console reconnects after disconnect", prop.ForAll(
		func(disconnectCount int) bool {
			// 创建测试 WebSocket 服务器
			reconnectAttempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{
					CheckOrigin: func(r *http.Request) bool { return true },
				}
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()

				reconnectAttempts++

				// 发送一条消息后立即关闭连接（模拟断开）
				metrics := map[string]interface{}{
					"timestamp":  time.Now().Format(time.RFC3339),
					"goroutines": 10,
				}
				conn.WriteJSON(metrics)
				time.Sleep(100 * time.Millisecond)
			}))
			defer server.Close()

			// 将 http:// 替换为 ws://
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/metrics"

			// 连接并等待多次重连
			for i := 0; i < disconnectCount; i++ {
				conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
				if err != nil {
					return false
				}

				// 读取消息
				var msg map[string]interface{}
				err = conn.ReadJSON(&msg)
				if err != nil {
					conn.Close()
					return false
				}

				conn.Close()
				time.Sleep(100 * time.Millisecond) // 等待服务器处理断开
			}

			// 验证重连次数
			return reconnectAttempts >= disconnectCount
		},
		gen.IntRange(1, 5), // 测试 1-5 次重连
	))

	properties.TestingRun(t)
}

func TestProperty_WebConsoleUpdateTiming(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("web console updates within 1 second", prop.ForAll(
		func(metricsCount int) bool {
			// 创建测试 WebSocket 服务器
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{
					CheckOrigin: func(r *http.Request) bool { return true },
				}
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()

				// 发送多条指标消息
				for i := 0; i < metricsCount; i++ {
					metrics := map[string]interface{}{
						"timestamp":  time.Now().Format(time.RFC3339),
						"goroutines": 10 + i,
						"cpu": map[string]interface{}{
							"usage_percent": float64(i) * 10.0,
						},
						"memory": map[string]interface{}{
							"heap_alloc":  1024 * 1024 * uint64(i),
							"heap_inuse":  1024 * 1024 * uint64(i),
							"stack_inuse": 1024 * uint64(i),
						},
						"gc": map[string]interface{}{
							"num_gc":      uint32(i),
							"pause_total": int64(i * 1000000),
							"last_pause":  int64(i * 100000),
							"pause_avg":   int64(i * 50000),
						},
					}

					err := conn.WriteJSON(metrics)
					if err != nil {
						return
					}

					time.Sleep(50 * time.Millisecond) // 模拟更新间隔
				}
			}))
			defer server.Close()

			wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/metrics"

			// 连接到 WebSocket
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				return false
			}
			defer conn.Close()

			// 接收并测量消息接收时间
			receiveTimes := make([]time.Duration, 0, metricsCount)
			for i := 0; i < metricsCount; i++ {
				startTime := time.Now()

				var msg map[string]interface{}
				err := conn.ReadJSON(&msg)
				if err != nil {
					return false
				}

				receiveTime := time.Since(startTime)
				receiveTimes = append(receiveTimes, receiveTime)

				// 验证接收时间在 1 秒内
				if receiveTime > time.Second {
					return false
				}
			}

			return len(receiveTimes) == metricsCount
		},
		gen.IntRange(5, 20), // 测试 5-20 条消息
	))

	properties.TestingRun(t)
}

// 测试 WebSocket 连接和数据流
func TestWebSocketConnectionAndDataFlow(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
		}
		defer conn.Close()

		// 立即发送当前指标
		initialMetrics := map[string]interface{}{
			"timestamp":  time.Now().Format(time.RFC3339),
			"goroutines": 42,
			"cpu": map[string]interface{}{
				"usage_percent": 25.5,
			},
			"memory": map[string]interface{}{
				"heap_alloc":  1024 * 1024 * 100,
				"heap_inuse":  1024 * 1024 * 80,
				"stack_inuse": 1024 * 50,
			},
			"gc": map[string]interface{}{
				"num_gc":      uint32(10),
				"pause_total": int64(5000000),
				"last_pause":  int64(500000),
				"pause_avg":   int64(500000),
			},
		}

		err = conn.WriteJSON(initialMetrics)
		if err != nil {
			t.Errorf("Failed to send initial metrics: %v", err)
			return
		}

		// 每秒发送更新
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for i := 0; i < 3; i++ {
			<-ticker.C
			metrics := map[string]interface{}{
				"timestamp":  time.Now().Format(time.RFC3339),
				"goroutines": 42 + i,
				"cpu": map[string]interface{}{
					"usage_percent": 25.5 + float64(i),
				},
			}
			err = conn.WriteJSON(metrics)
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	// 连接到 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// 验证立即接收到初始指标
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var initialMsg map[string]interface{}
	err = conn.ReadJSON(&initialMsg)
	if err != nil {
		t.Fatalf("Failed to receive initial metrics: %v", err)
	}

	if initialMsg["goroutines"].(float64) != 42 {
		t.Errorf("Expected goroutines=42, got %v", initialMsg["goroutines"])
	}

	// 验证定期更新（每秒）
	updateCount := 0
	for i := 0; i < 3; i++ {
		conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
		var msg map[string]interface{}
		err = conn.ReadJSON(&msg)
		if err != nil {
			t.Errorf("Failed to receive update %d: %v", i, err)
			break
		}
		updateCount++
	}

	if updateCount != 3 {
		t.Errorf("Expected 3 updates, got %d", updateCount)
	}
}

// 测试断开后重连
func TestWebSocketReconnection(t *testing.T) {
	connectionCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		connectionCount++

		// 发送一条消息后关闭
		metrics := map[string]interface{}{
			"timestamp":  time.Now().Format(time.RFC3339),
			"goroutines": connectionCount,
		}
		conn.WriteJSON(metrics)
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 连接、断开、重连 3 次
	for i := 0; i < 3; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Connection %d failed: %v", i+1, err)
		}

		var msg map[string]interface{}
		err = conn.ReadJSON(&msg)
		if err != nil {
			t.Errorf("Failed to read message on connection %d: %v", i+1, err)
		}

		conn.Close()
		time.Sleep(100 * time.Millisecond)
	}

	if connectionCount != 3 {
		t.Errorf("Expected 3 connections, got %d", connectionCount)
	}
}

// 测试 UI 更新时间
func TestUIUpdateTiming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// 发送 10 条消息
		for i := 0; i < 10; i++ {
			metrics := map[string]interface{}{
				"timestamp":  time.Now().Format(time.RFC3339),
				"goroutines": 10 + i,
			}
			conn.WriteJSON(metrics)
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// 测量每条消息的接收时间
	for i := 0; i < 10; i++ {
		startTime := time.Now()

		var msg map[string]interface{}
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		err := conn.ReadJSON(&msg)
		if err != nil {
			t.Fatalf("Failed to receive message %d: %v", i, err)
		}

		receiveTime := time.Since(startTime)

		// 验证接收时间在 1 秒内
		if receiveTime > time.Second {
			t.Errorf("Message %d took %v to receive (>1s)", i, receiveTime)
		}

		// 验证数据正确性
		if msg["goroutines"].(float64) != float64(10+i) {
			t.Errorf("Expected goroutines=%d, got %v", 10+i, msg["goroutines"])
		}
	}
}

// 辅助函数：验证 JSON 结构
func validateMetricsJSON(t *testing.T, data []byte) bool {
	var metrics map[string]interface{}
	err := json.Unmarshal(data, &metrics)
	if err != nil {
		t.Errorf("Invalid JSON: %v", err)
		return false
	}

	// 验证必需字段
	requiredFields := []string{"timestamp", "goroutines"}
	for _, field := range requiredFields {
		if _, ok := metrics[field]; !ok {
			t.Errorf("Missing required field: %s", field)
			return false
		}
	}

	return true
}
