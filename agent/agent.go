package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// agent 代理实例，管理 HTTP 服务器、指标收集器和状态
type agent struct {
	config    Config
	server    *http.Server
	collector *metricsCollector
	wsManager *wsManager
	mu        sync.Mutex
	running   bool
	startTime time.Time
}

var (
	globalAgent *agent
	agentMu     sync.Mutex
)

// Start 启动 Agent，返回错误如果启动失败
// 此函数是非阻塞的，Agent 在后台运行
func Start(config Config) error {
	agentMu.Lock()
	defer agentMu.Unlock()

	// 如果已经有运行中的 agent，返回错误
	if globalAgent != nil && globalAgent.running {
		return fmt.Errorf("agent is already running")
	}

	// 设置默认值
	config.SetDefaults()

	// 验证配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// 创建新的 agent 实例
	a := &agent{
		config:    config,
		startTime: time.Now(),
	}

	// 初始化 WebSocket 管理器
	a.wsManager = newWSManager()
	a.wsManager.run()

	// 初始化指标收集器（如果启用）
	if config.EnableMetrics {
		a.collector = newMetricsCollector(1 * time.Second)
		a.collector.wsManager = a.wsManager
	}

	// 创建 HTTP 服务器
	mux := http.NewServeMux()

	// 注册 API 路由
	mux.HandleFunc("/api/v1/metrics", a.corsMiddleware(a.handleMetrics))
	mux.HandleFunc("/api/v1/info", a.corsMiddleware(a.handleInfo))

	// 注册 WebSocket 路由
	mux.HandleFunc("/ws/metrics", a.handleWebSocket)

	// 注册 pprof 路由（如果启用）
	if config.EnablePprof {
		mux.HandleFunc("/debug/pprof/", a.corsMiddleware(pprof.Index))
		mux.HandleFunc("/debug/pprof/cmdline", a.corsMiddleware(pprof.Cmdline))
		mux.HandleFunc("/debug/pprof/profile", a.corsMiddleware(pprof.Profile))
		mux.HandleFunc("/debug/pprof/symbol", a.corsMiddleware(pprof.Symbol))
		mux.HandleFunc("/debug/pprof/trace", a.corsMiddleware(pprof.Trace))
		// 其他 pprof 端点（heap, goroutine, block, mutex 等）通过 Index 处理
	}

	// 注册根路由（仅匹配 "/"）
	mux.HandleFunc("/", a.corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Go-Arthas Agent is running")
	}))

	a.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: mux,
	}

	// 尝试启动 HTTP 服务器
	listener, err := net.Listen("tcp", a.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// 在后台启动服务器
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] HTTP server panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
			}
		}()

		if err := a.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[ERROR] HTTP server error: %v", err)
		}
	}()

	// 启动指标收集器（如果启用）
	if a.collector != nil {
		a.collector.start()
	}

	a.running = true
	globalAgent = a

	log.Printf("[INFO] Agent started on port %d", config.Port)

	return nil
}

// Stop 优雅停止 Agent，关闭所有连接和后台任务
func Stop() error {
	agentMu.Lock()
	defer agentMu.Unlock()

	if globalAgent == nil || !globalAgent.running {
		return fmt.Errorf("agent is not running")
	}

	a := globalAgent

	// 停止 WebSocket 管理器
	if a.wsManager != nil {
		a.wsManager.stop()
	}

	// 停止指标收集器
	if a.collector != nil {
		a.collector.stop()
	}

	// 停止 HTTP 服务器
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
	}

	a.running = false
	globalAgent = nil

	log.Printf("[INFO] Agent stopped")

	return nil
}

// GetMetrics 返回当前的运行时指标快照
func GetMetrics() *Metrics {
	agentMu.Lock()
	defer agentMu.Unlock()

	if globalAgent == nil || !globalAgent.running || globalAgent.collector == nil {
		return nil
	}

	return globalAgent.collector.GetMetrics()
}

// corsMiddleware 添加 CORS 头到所有响应
func (a *agent) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 添加 CORS 头
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// 处理 OPTIONS 预检请求
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 调用下一个处理器
		next(w, r)
	}
}

// handleMetrics 处理 /api/v1/metrics 请求
func (a *agent) handleMetrics(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] Metrics handler panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}()

	// 只接受 GET 请求
	if r.Method != http.MethodGet {
		log.Printf("[WARN] Invalid method for /api/v1/metrics: %s from %s", r.Method, r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查 collector 是否存在
	if a.collector == nil {
		log.Printf("[WARN] Metrics collection is disabled, request from %s", r.RemoteAddr)
		http.Error(w, "Metrics collection is disabled", http.StatusServiceUnavailable)
		return
	}

	// 获取当前指标
	metrics := a.collector.GetMetrics()
	if metrics == nil {
		log.Printf("[WARN] Metrics not available, request from %s", r.RemoteAddr)
		http.Error(w, "Metrics not available", http.StatusServiceUnavailable)
		return
	}

	// 返回 JSON 响应
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf("[ERROR] Failed to encode metrics for %s: %v", r.RemoteAddr, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleInfo 处理 /api/v1/info 请求
func (a *agent) handleInfo(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] Info handler panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}()

	// 只接受 GET 请求
	if r.Method != http.MethodGet {
		log.Printf("[WARN] Invalid method for /api/v1/info: %s from %s", r.Method, r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 构建系统信息
	uptime := time.Since(a.startTime)
	info := SystemInfo{
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
		ProcessID: os.Getpid(),
		StartTime: a.startTime,
		Uptime:    uptime.String(),
	}

	// 返回 JSON 响应
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Printf("[ERROR] Failed to encode system info for %s: %v", r.RemoteAddr, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源（CORS）
		return true
	},
}

// handleWebSocket 处理 WebSocket 连接请求
func (a *agent) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] WebSocket handler panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
		}
	}()

	// 只接受 GET 请求
	if r.Method != http.MethodGet {
		log.Printf("[WARN] Invalid method for /ws/metrics: %s from %s", r.Method, r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查 collector 是否存在
	if a.collector == nil {
		log.Printf("[WARN] Metrics collection is disabled, WebSocket request from %s", r.RemoteAddr)
		http.Error(w, "Metrics collection is disabled", http.StatusServiceUnavailable)
		return
	}

	// 升级 HTTP 连接为 WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to upgrade WebSocket connection from %s: %v", r.RemoteAddr, err)
		return
	}

	// 获取当前指标
	initialMetrics := a.collector.GetMetrics()

	// 处理连接
	go a.wsManager.handleConnection(conn, initialMetrics)
}
