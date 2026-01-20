package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/songzhibin97/go-arthas/agent"
)

func main() {
	// 配置 Agent
	// 所有配置选项都是可选的，这里展示了所有可用选项
	config := agent.Config{
		Port:          8563,   // HTTP 服务器端口（默认 8563）
		EnablePprof:   true,   // 启用 pprof 性能分析端点
		EnableMetrics: true,   // 启用运行时指标收集
		LogLevel:      "info", // 日志级别：debug, info, warn, error
	}

	// 启动 Agent
	// Agent 在后台运行，不会阻塞主程序
	if err := agent.Start(config); err != nil {
		log.Printf("警告: Agent 启动失败: %v", err)
		log.Printf("应用程序将继续运行，但诊断功能不可用")
		// 注意：即使 Agent 启动失败，应用程序也会继续运行
	} else {
		log.Printf("Agent 已启动，监听端口 %d", config.Port)
		log.Printf("访问 http://localhost:%d/api/v1/metrics 查看指标", config.Port)
		log.Printf("访问 http://localhost:%d/api/v1/info 查看系统信息", config.Port)
		log.Printf("访问 http://localhost:%d/debug/pprof/ 查看性能分析", config.Port)
	}

	// 确保在程序退出时优雅停止 Agent
	defer func() {
		if err := agent.Stop(); err != nil {
			log.Printf("停止 Agent 时出错: %v", err)
		}
	}()

	// 设置信号处理，支持优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 启动一些示例工作负载来演示指标收集
	go simulateWorkload()

	log.Println("应用程序正在运行...")
	log.Println("按 Ctrl+C 退出")

	// 等待退出信号
	<-sigCh
	log.Println("\n收到退出信号，正在关闭...")
}

// simulateWorkload 模拟一些工作负载以生成有趣的指标
func simulateWorkload() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 创建一些 goroutine
		for i := 0; i < 5; i++ {
			go func() {
				// 模拟一些工作
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

				// 分配一些内存
				data := make([]byte, rand.Intn(1024*1024)) // 最多 1MB
				_ = data
			}()
		}

		// 执行一些 CPU 密集型操作
		go func() {
			sum := 0
			for i := 0; i < 1000000; i++ {
				sum += i
			}
			_ = sum
		}()

		fmt.Print(".")
	}
}
