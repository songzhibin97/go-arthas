package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebSocket_ConcurrentWritesSerialized 是 wsClient 写串行化（writeMu）的回归守护。
//
// gorilla/websocket 禁止对同一连接并发写数据帧；ping goroutine、初始/广播 goroutine
// 都会写同一连接。本测试直接对一个真实连接的 wsClient 并发调用三种写方法，必须在
// `-race` 下保持干净。若有人移除 writeMu，并发数据写会触发 gorilla 的
// "concurrent write to websocket connection" panic 或被竞态检测器标记，从而让本测试失败。
//
// 关键：本测试**不**调用 skipEnvSensitive —— 它是功能正确性守护，必须随常规
// `go test`/`-race` 一起运行（而那 4 个 TestProperty_WebSocket* 性能测试默认跳过、
// 仅 ARTHAS_PERF_TESTS=1 时运行，等于并发写修复在竞态检测器下没有任何护栏）。
func TestWebSocket_ConcurrentWritesSerialized(t *testing.T) {
	upgrader := websocket.Upgrader{}
	serverConnCh := make(chan *websocket.Conn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnCh <- c
		// 持续 drain 读取，避免对端写阻塞
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close()
	// 客户端 drain，避免服务端写阻塞
	go func() {
		for {
			if _, _, err := clientConn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	serverConn := <-serverConnCh
	client := &wsClient{conn: serverConn}

	// 对同一连接并发走全部三条写路径
	const workers, iters = 24, 30
	deadline := time.Now().Add(5 * time.Second)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				switch j % 3 {
				case 0:
					_ = client.writeMessage(websocket.TextMessage, []byte("data"), deadline)
				case 1:
					_ = client.writeJSON(map[string]int{"i": i, "j": j}, deadline)
				case 2:
					_ = client.writeControl(websocket.PingMessage, []byte{}, deadline)
				}
			}
		}(i)
	}
	wg.Wait()
}
