package agent

import (
	"encoding/json"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsClient 包装单个 WebSocket 连接，并用 writeMu 串行化所有写操作。
// gorilla/websocket 不允许对同一连接并发写：ping 控制帧（来自 ping goroutine）、
// 初始/广播数据帧（来自连接处理 goroutine 与广播 goroutine）可能并发发生，
// 若不加锁会破坏帧、触发数据竞争。所有写都必须经由本类型的方法。
type wsClient struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

// writeMessage 加锁写入一条数据帧
func (c *wsClient) writeMessage(messageType int, data []byte, deadline time.Time) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return c.conn.WriteMessage(messageType, data)
}

// writeJSON 加锁写入一条 JSON 数据帧
func (c *wsClient) writeJSON(v interface{}, deadline time.Time) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return c.conn.WriteJSON(v)
}

// writeControl 加锁写入一条控制帧（如 ping）
func (c *wsClient) writeControl(messageType int, data []byte, deadline time.Time) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteControl(messageType, data, deadline)
}

// wsManager 管理 WebSocket 连接和消息广播
type wsManager struct {
	clients    map[*wsClient]bool // 活跃连接集合
	broadcast  chan *Metrics      // 广播通道
	register   chan *wsClient     // 注册新连接
	unregister chan *wsClient     // 注销连接
	mu         sync.RWMutex       // 保护 clients map
	stopCh     chan struct{}      // 停止信号通道
	doneCh     chan struct{}      // 完成信号通道
	stopped    bool               // 是否已停止
	stopMu     sync.Mutex         // 保护 stopped 字段
}

// newWSManager 创建新的 WebSocket 管理器
func newWSManager() *wsManager {
	return &wsManager{
		clients:    make(map[*wsClient]bool),
		broadcast:  make(chan *Metrics, 10),
		register:   make(chan *wsClient, 10),
		unregister: make(chan *wsClient, 10),
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
			case client := <-wm.register:
				wm.mu.Lock()
				wm.clients[client] = true
				count := len(wm.clients)
				wm.mu.Unlock()
				log.Printf("[INFO] WebSocket client connected, total clients: %d", count)

			case client := <-wm.unregister:
				wm.mu.Lock()
				if _, ok := wm.clients[client]; ok {
					delete(wm.clients, client)
					client.conn.Close()
					count := len(wm.clients)
					wm.mu.Unlock()
					log.Printf("[INFO] WebSocket client disconnected, total clients: %d", count)
				} else {
					wm.mu.Unlock()
				}

			case metrics := <-wm.broadcast:
				wm.broadcastMetrics(metrics)

			case <-wm.stopCh:
				// 关闭所有连接
				wm.mu.Lock()
				for client := range wm.clients {
					client.conn.Close()
				}
				wm.clients = make(map[*wsClient]bool)
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
	client := &wsClient{conn: conn}

	// 创建连接专用的停止通道
	connStopCh := make(chan struct{})

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] WebSocket connection handler panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
		}
		close(connStopCh) // 通知 ping goroutine 停止
		wm.unregister <- client
	}()

	// 注册连接
	wm.register <- client

	// 立即发送当前指标
	if initialMetrics != nil {
		if err := client.writeJSON(initialMetrics, time.Now().Add(10*time.Second)); err != nil {
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
				if err := client.writeControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
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

	// 序列化一次，发送给所有客户端
	data, err := json.Marshal(metrics)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal metrics for broadcast: %v", err)
		return
	}

	// 在锁内快照客户端列表，随后释放锁再写，避免持锁期间阻塞
	wm.mu.RLock()
	clients := make([]*wsClient, 0, len(wm.clients))
	for client := range wm.clients {
		clients = append(clients, client)
	}
	wm.mu.RUnlock()

	// 广播给所有客户端（每个客户端的写经 writeMu 串行化）
	for _, client := range clients {
		go func(c *wsClient) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ERROR] Broadcast to client panic recovered: %v", r)
				}
			}()

			if err := c.writeMessage(websocket.TextMessage, data, time.Now().Add(10*time.Second)); err != nil {
				log.Printf("[WARN] Failed to broadcast to WebSocket client: %v", err)
				wm.unregister <- c
			}
		}(client)
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
