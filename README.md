# Go-Arthas

> ⚠️ **重要说明**：Go-Arthas 是一个受 [Alibaba Arthas](https://github.com/alibaba/arthas) 启发的 Go 应用监控工具。由于 Go 语言特性限制，本项目**专注于运行时监控和性能分析**，无法实现 Java Arthas 的方法级诊断功能（如 watch、trace、monitor 等）。详见[功能对比](#功能对比)。

Go-Arthas 是一个用于 Go 应用程序的**运行时监控和性能分析工具**，灵感来自 Java Arthas。它提供运行时指标监控、性能分析和诊断能力，通过嵌入式 SDK 集成到目标应用程序中。

## 特性

- **轻量级集成**: 只需几行代码即可集成到现有应用程序
- **实时指标**: 监控 goroutine、内存、CPU 和 GC 统计信息
- **性能分析**: 集成 Go 的 pprof，支持 CPU、内存、goroutine 等 profile
- **多种接口**: HTTP API、WebSocket 实时更新、CLI 工具、Web Console
- **低开销**: CPU 开销 < 5%，内存开销 < 50MB
- **生产就绪**: 故障安全设计，Agent 失败不会影响应用程序
- **跨平台**: 支持 Linux、macOS、Windows（amd64、arm64）

## 快速开始

### 安装

```bash
go get github.com/songzhibin97/go-arthas
```

### 集成到应用程序

```go
package main

import (
    "log"
    "github.com/songzhibin97/go-arthas/agent"
)

func main() {
    // 配置并启动 Agent
    config := agent.Config{
        Port:          8563,
        EnablePprof:   true,
        EnableMetrics: true,
        LogLevel:      "info",
    }
    
    if err := agent.Start(config); err != nil {
        log.Printf("警告: Agent 启动失败: %v", err)
        // 应用程序继续运行
    }
    defer agent.Stop()
    
    // 你的应用程序代码
    // ...
}
```

### 访问诊断信息

启动应用程序后，可以通过以下方式访问诊断信息：

**HTTP API**:
```bash
# 查看运行时指标
curl http://localhost:8563/api/v1/metrics

# 查看系统信息
curl http://localhost:8563/api/v1/info

# 捕获 CPU profile（30 秒）
curl http://localhost:8563/debug/pprof/profile?seconds=30 > cpu.prof
```

**CLI 工具**:
```bash
# 安装 CLI
go install github.com/songzhibin97/go-arthas/cmd/go-arthas@latest

# 查看指标
go-arthas metrics --host localhost:8563

# 捕获 profile
go-arthas profile cpu --host localhost:8563 --duration 30
```

**Web Console**:
在浏览器中打开 `http://localhost:3000/?agent=localhost:8563` 查看实时仪表板。

## 文档

### Agent SDK

#### 配置选项

```go
type Config struct {
    Port          int    // HTTP 服务器端口（默认 8563）
    EnablePprof   bool   // 启用 pprof 端点（默认 false）
    EnableMetrics bool   // 启用指标收集（默认 false）
    LogLevel      string // 日志级别：debug, info, warn, error（默认 info）
}
```

#### API 函数

- `agent.Start(config Config) error`: 启动 Agent（非阻塞）
- `agent.Stop() error`: 停止 Agent（优雅关闭）
- `agent.GetMetrics() *Metrics`: 获取当前指标快照

### HTTP API 端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/metrics` | GET | 返回当前运行时指标（JSON） |
| `/api/v1/info` | GET | 返回系统信息（JSON） |
| `/debug/pprof/` | GET | Pprof 索引页 |
| `/debug/pprof/profile` | GET | CPU profile |
| `/debug/pprof/heap` | GET | 堆 profile |
| `/debug/pprof/goroutine` | GET | Goroutine profile |
| `/debug/pprof/block` | GET | 阻塞 profile |
| `/debug/pprof/mutex` | GET | 互斥锁 profile |
| `/ws/metrics` | WebSocket | 实时指标流（每秒更新） |

### CLI 工具

#### 安装

```bash
go install github.com/songzhibin97/go-arthas/cmd/go-arthas@latest
```

#### 命令

```bash
# 查看运行时指标
go-arthas metrics --host localhost:8563

# 查看系统信息
go-arthas info --host localhost:8563

# 捕获 CPU profile
go-arthas profile cpu --host localhost:8563 --duration 30

# 捕获堆 profile
go-arthas profile heap --host localhost:8563

# 捕获 goroutine profile
go-arthas profile goroutine --host localhost:8563
```

#### 选项

- `--host`: Agent 地址（格式: `host:port`，默认 `localhost:8563`）
- `--duration`: Profile 持续时间（仅用于 CPU profile，默认 30 秒）
- `--output`: 输出文件路径（默认自动生成）

### Web Console

Web Console 提供实时可视化仪表板，显示：

- **Goroutine 趋势图**: 过去 5 分钟的 goroutine 数量变化
- **内存面板**: 堆内存、栈内存使用情况
- **CPU 仪表盘**: 当前 CPU 使用率
- **GC 统计**: GC 次数、暂停时间
- **系统信息**: Go 版本、操作系统、架构、运行时长

#### 启动 Web Console

```bash
cd web
npm install
npm run dev
```

然后在浏览器中访问 `http://localhost:3000/?agent=localhost:8563`。

## 性能影响

Go-Arthas 设计为最小化性能开销：

| 场景 | CPU 开销 | 内存开销 |
|------|----------|----------|
| 启用指标收集 | < 5% | < 50MB |
| 空闲（无客户端连接） | < 1% | < 10MB |
| 捕获 profile | < 10% | 取决于 profile 类型 |

- Profile 捕获不会导致应用程序暂停超过 100ms
- Agent 创建的 goroutine ≤ 10 个
- 所有操作都是并发安全的

## 架构

```
┌─────────────────────────────────────┐
│     Target Application Process      │
│                                     │
│  ┌──────────────┐                  │
│  │ Application  │                  │
│  │    Code      │                  │
│  └──────┬───────┘                  │
│         │ agent.Start()            │
│         ▼                           │
│  ┌──────────────┐                  │
│  │  Agent SDK   │                  │
│  ├──────────────┤                  │
│  │ • Metrics    │                  │
│  │   Collector  │                  │
│  │ • HTTP       │                  │
│  │   Server     │                  │
│  │ • WebSocket  │                  │
│  │   Manager    │                  │
│  │ • Pprof      │                  │
│  │   Handler    │                  │
│  └──────┬───────┘                  │
│         │                           │
└─────────┼───────────────────────────┘
          │
          │ HTTP/WebSocket
          │
    ┌─────┴─────┬──────────┐
    │           │          │
┌───▼───┐  ┌───▼───┐  ┌──▼────┐
│  CLI  │  │  Web  │  │ Custom│
│ Tool  │  │Console│  │Client │
└───────┘  └───────┘  └───────┘
```

## 示例

查看 [examples/simple](examples/simple) 目录获取完整的示例应用程序。

## 故障排除

### Agent 启动失败

**问题**: `agent.Start()` 返回错误

**可能原因**:
- 端口已被占用
- 端口号无效（不在 1-65535 范围内）
- 权限不足（尝试绑定特权端口 < 1024）

**解决方案**:
```go
config := agent.Config{
    Port: 8563, // 使用非特权端口
}
if err := agent.Start(config); err != nil {
    log.Printf("Agent 启动失败: %v", err)
    // 检查错误消息并调整配置
}
```

### 无法连接到 Agent

**问题**: CLI 或 Web Console 无法连接

**检查清单**:
1. Agent 是否成功启动？检查应用程序日志
2. 防火墙是否阻止了端口？
3. 使用正确的 host:port 组合？
4. 应用程序是否在运行？

**测试连接**:
```bash
curl http://localhost:8563/api/v1/info
```

### 指标不可用

**问题**: `/api/v1/metrics` 返回 503 错误

**原因**: `EnableMetrics` 未设置为 `true`

**解决方案**:
```go
config := agent.Config{
    EnableMetrics: true, // 必须显式启用
}
```

### Pprof 端点不可用

**问题**: `/debug/pprof/` 返回 404 错误

**原因**: `EnablePprof` 未设置为 `true`

**解决方案**:
```go
config := agent.Config{
    EnablePprof: true, // 必须显式启用
}
```

### 性能开销过高

**问题**: Agent 导致应用程序性能下降

**检查**:
1. 是否有大量客户端连接？
2. 是否频繁捕获 profile？
3. 应用程序本身是否有性能问题？

**优化**:
- 限制并发客户端连接数
- 避免连续捕获 CPU profile
- 在不需要时禁用指标收集

### WebSocket 连接断开

**问题**: Web Console 频繁断开连接

**可能原因**:
- 网络不稳定
- 代理或负载均衡器超时
- Agent 重启

**解决方案**: Web Console 会自动重连（每 5 秒尝试一次）

## 系统要求

- Go 1.18 或更高版本
- 支持的操作系统:
  - Linux (amd64, arm64)
  - macOS (amd64, arm64)
  - Windows (amd64)

## 开发

### 构建

```bash
# 构建所有组件
make build

# 仅构建 CLI
make build-cli

# 仅构建 Web Console
make build-web
```

### 测试

```bash
# 运行所有测试
make test

# 运行单元测试
go test ./...

# 运行属性测试
go test -v -run TestProperty ./...

# 运行基准测试
go test -bench=. ./...

# 运行竞态检测
go test -race ./...
```

### 代码覆盖率

```bash
make coverage
```

## 贡献

欢迎贡献！请遵循以下步骤：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 功能对比

### 与 Alibaba Arthas (Java) 的对比

Go-Arthas 受 Alibaba Arthas 启发，但由于 Go 语言特性限制，功能范围有所不同。**详细对比请查看 [功能对比文档](docs/COMPARISON.md)**。

| 功能类别 | Alibaba Arthas (Java) | Go-Arthas | 说明 |
|---------|----------------------|-----------|------|
| **基础监控** | ✅ | ✅ | CPU、内存、线程/Goroutine、GC |
| **性能分析** | ✅ | ✅ | CPU/内存 Profile、火焰图 |
| **方法观察** | ✅ watch | ❌ | Go 反射能力有限，无法实现 |
| **调用追踪** | ✅ trace | ❌ | 需要字节码增强，Go 不支持 |
| **方法监控** | ✅ monitor | ❌ | 无法动态统计单个函数 |
| **反编译** | ✅ jad | ❌ | Go 编译成机器码，无法反编译 |
| **热更新** | ✅ redefine | ❌ | Go 是静态编译语言 |
| **时间隧道** | ✅ tt | ❌ | 无法拦截函数调用 |

**Go 生态的替代方案**：
- 调用链追踪：[OpenTelemetry](https://opentelemetry.io/)
- 方法监控：[Prometheus](https://prometheus.io/) + 手动埋点
- 分布式追踪：[Jaeger](https://www.jaegertracing.io/)

### Go-Arthas 的定位

Go-Arthas 是一个**运行时监控和性能分析工具**，提供：
- 实时指标监控（比原生 pprof 更友好）
- WebSocket 实时推送（pprof 不支持）
- CLI 工具和 Web Console（更好的用户体验）
- 低开销、生产就绪

**不提供**（受 Go 语言特性限制）：
- 方法级诊断（watch、trace、monitor）
- 代码操作（jad、redefine）
- 动态表达式（ognl）

这不是项目缺陷，而是 Go 语言的设计理念和技术限制。详见 [功能对比文档](docs/COMPARISON.md)。

## 文档

- [构建指南](docs/BUILD.md)
- [CLI 使用指南](docs/CLI_USAGE.md)
- [Web Console 使用指南](docs/WEB_CONSOLE.md)
- [功能对比：Go-Arthas vs Alibaba Arthas](docs/COMPARISON.md)
- [常见问题 (FAQ)](docs/FAQ.md)

## 致谢

本项目受 [Alibaba Arthas](https://github.com/alibaba/arthas) 启发。

## 联系方式

- 问题反馈: [GitHub Issues](https://github.com/songzhibin97/go-arthas/issues)
- 讨论: [GitHub Discussions](https://github.com/songzhibin97/go-arthas/discussions)
