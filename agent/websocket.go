package agent

import (
	"encoding/json"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsManager 管理 WebSocket 连接和消息广播
type wsManager struct {
	clients    map[*websocket.Conn]bool // 活跃连接集合
	broadcast  chan *Metrics            // 广播通道
	register   chan *websocket.Conn     // 注册新连接
	unregister chan *websocket.Conn     // 注销连接
	mu         sync.RWMutex             // 保护 clients map
	stopCh     chan struct{}            // 停止信号通道
	doneCh     chan struct{}            // 完成信号通道
	stopped    bool                     // 是否已停止
	stopMu     sync.Mutex               // 保护 stopped 字段
}

// newWSManager 创建新的 WebSocket 管理器
func newWSManager() *wsManager {
	return &wsManager{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan *Metrics, 10),
		register:   make(chan *websocket.Conn, 10),
		unregister: make(chan *websocket.Conn, 10),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// run 运行 WebSocket 管理器主循环
func (wm *wsManager) run() {
	go func() {
		defer close(wm.doneCh)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] WebSocket manager panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
			}
		}()

		for {
			select {
			case conn := <-wm.register:
				wm.mu.Lock()
				wm.clients[conn] = true
				wm.mu.Unlock()
				log.Printf("[INFO] WebSocket client connected, total clients: %d", len(wm.clients))

			case conn := <-wm.unregister:
				wm.mu.Lock()
				if _, ok := wm.clients[conn]; ok {
					delete(wm.clients, conn)
					conn.Close()
					log.Printf("[INFO] WebSocket client disconnected, total clients: %d", len(wm.clients))
				}
				wm.mu.Unlock()

			case metrics := <-wm.broadcast:
				wm.broadcastMetrics(metrics)

			case <-wm.stopCh:
				// 关闭所有连接
				wm.mu.Lock()
				for conn := range wm.clients {
					conn.Close()
				}
				wm.clients = make(map[*websocket.Conn]bool)
				wm.mu.Unlock()
				return
			}
		}
	}()
}

// stop 停止 WebSocket 管理器
func (wm *wsManager) stop() {
	wm.stopMu.Lock()
	defer wm.stopMu.Unlock()

	if wm.stopped {
		return
	}

	close(wm.stopCh)
	<-wm.doneCh
	wm.stopped = true
}

// handleConnection 处理单个 WebSocket 连接
func (wm *wsManager) handleConnection(conn *websocket.Conn, initialMetrics *Metrics) {
	// 创建连接专用的停止通道
	connStopCh := make(chan struct{})

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] WebSocket connection handler panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
		}
		close(connStopCh) // 通知 ping goroutine 停止
		wm.unregister <- conn
	}()

	// 注册连接
	wm.register <- conn

	// 立即发送当前指标
	if initialMetrics != nil {
		if err := conn.WriteJSON(initialMetrics); err != nil {
			log.Printf("[WARN] Failed to send initial metrics to WebSocket client: %v", err)
			return
		}
	}

	// 设置读取超时和 pong 处理
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 启动 ping 发送器
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Ping goroutine panic recovered: %v", r)
			}
		}()

		for {
			select {
			case <-pingTicker.C:
				if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			case <-connStopCh:
				return
			case <-wm.stopCh:
				return
			}
		}
	}()

	// 读取循环（主要用于检测连接断开）
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WARN] WebSocket unexpected close: %v", err)
			}
			break
		}
	}
}

// broadcastMetrics 向所有连接的客户端广播指标
func (wm *wsManager) broadcastMetrics(metrics *Metrics) {
	if metrics == nil {
		return
	}

	wm.mu.RLock()
	defer wm.mu.RUnlock()

	// 序列化一次，发送给所有客户端
	data, err := json.Marshal(metrics)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal metrics for broadcast: %v", err)
		return
	}

	// 广播给所有客户端
	for conn := range wm.clients {
		// 使用 goroutine 避免阻塞
		go func(c *websocket.Conn) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ERROR] Broadcast to client panic recovered: %v", r)
				}
			}()

			c.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("[WARN] Failed to broadcast to WebSocket client: %v", err)
				wm.unregister <- c
			}
		}(conn)
	}
}

// BroadcastMetrics 向所有连接的客户端广播指标（公开方法）
func (wm *wsManager) BroadcastMetrics(metrics *Metrics) {
	select {
	case wm.broadcast <- metrics:
	case <-time.After(100 * time.Millisecond):
		log.Printf("[WARN] Broadcast channel full, dropping metrics update")
	}
}
