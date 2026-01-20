# Go-Arthas 简单示例

这是一个展示如何在 Go 应用程序中集成 Go-Arthas Agent 的简单示例。

## 运行示例

```bash
cd examples/simple
go run main.go
```

## 配置说明

### 基本配置

```go
config := agent.Config{
    Port:          8563,  // HTTP 服务器端口（默认 8563）
    EnablePprof:   true,  // 启用 pprof 性能分析端点
    EnableMetrics: true,  // 启用运行时指标收集
    LogLevel:      "info", // 日志级别：debug, info, warn, error
}
```

### 配置选项详解

- **Port**: Agent HTTP 服务器监听的端口号
  - 默认值: 8563
  - 有效范围: 1-65535
  
- **EnablePprof**: 是否启用 Go 的 pprof 性能分析端点
  - 默认值: false
  - 启用后可以通过 `/debug/pprof/` 访问性能分析数据
  
- **EnableMetrics**: 是否启用运行时指标收集
  - 默认值: false
  - 启用后会每秒收集一次运行时指标（goroutine、内存、CPU、GC）
  
- **LogLevel**: Agent 的日志级别
  - 默认值: "info"
  - 可选值: "debug", "info", "warn", "error"

## 使用 Agent

### 1. 启动 Agent

```go
if err := agent.Start(config); err != nil {
    log.Printf("警告: Agent 启动失败: %v", err)
    // 应用程序继续运行
}
```

**重要**: 即使 Agent 启动失败，应用程序也会继续正常运行。这确保了诊断工具不会影响生产环境的稳定性。

### 2. 停止 Agent

```go
defer func() {
    if err := agent.Stop(); err != nil {
        log.Printf("停止 Agent 时出错: %v", err)
    }
}()
```

建议使用 `defer` 确保在程序退出时优雅停止 Agent。

## 访问诊断端点

启动应用程序后，可以通过以下端点访问诊断信息：

### HTTP API

- **运行时指标**: http://localhost:8563/api/v1/metrics
  - 返回当前的 goroutine 数量、内存使用、CPU 使用率、GC 统计等
  
- **系统信息**: http://localhost:8563/api/v1/info
  - 返回 Go 版本、操作系统、架构、进程 ID、运行时长等

### Pprof 性能分析

- **Pprof 索引**: http://localhost:8563/debug/pprof/
- **CPU Profile**: http://localhost:8563/debug/pprof/profile?seconds=30
- **Heap Profile**: http://localhost:8563/debug/pprof/heap
- **Goroutine Profile**: http://localhost:8563/debug/pprof/goroutine
- **Block Profile**: http://localhost:8563/debug/pprof/block
- **Mutex Profile**: http://localhost:8563/debug/pprof/mutex

### WebSocket 实时更新

- **实时指标流**: ws://localhost:8563/ws/metrics
  - 连接后会立即收到当前指标，之后每秒推送一次更新

## 使用 CLI 工具

```bash
# 查看运行时指标
go-arthas metrics --host localhost:8563

# 查看系统信息
go-arthas info --host localhost:8563

# 捕获 CPU profile（30 秒）
go-arthas profile cpu --host localhost:8563 --duration 30

# 捕获堆 profile
go-arthas profile heap --host localhost:8563
```

## 使用 Web Console

在浏览器中打开 Web Console（假设已部署）：

```
http://localhost:3000/?agent=localhost:8563
```

Web Console 提供实时仪表板，显示：
- Goroutine 数量趋势图
- 内存使用情况
- CPU 使用率
- GC 统计信息
- 系统信息

## 性能影响

Go-Arthas 设计为低开销：
- 启用指标收集时 CPU 开销 < 5%
- 内存开销 < 50MB
- 空闲时 CPU 开销 < 1%
- Agent 创建的 goroutine ≤ 10 个

## 最佳实践

1. **生产环境**: 可以安全地在生产环境中运行，但建议：
   - 使用防火墙限制对 Agent 端口的访问
   - 考虑在需要时动态启用/禁用 pprof
   
2. **开发环境**: 建议启用所有功能以便于调试

3. **错误处理**: 始终检查 `agent.Start()` 的返回值，但允许应用程序在 Agent 失败时继续运行

4. **优雅退出**: 使用 `defer agent.Stop()` 确保资源正确清理
