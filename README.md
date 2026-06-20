# Go-Arthas

[![CI](https://github.com/songzhibin97/go-arthas/actions/workflows/ci.yml/badge.svg)](https://github.com/songzhibin97/go-arthas/actions/workflows/ci.yml)
[![Release](https://github.com/songzhibin97/go-arthas/actions/workflows/release.yml/badge.svg)](https://github.com/songzhibin97/go-arthas/actions/workflows/release.yml)
[![GitHub release](https://img.shields.io/github/v/release/songzhibin97/go-arthas?sort=semver)](https://github.com/songzhibin97/go-arthas/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/songzhibin97/go-arthas.svg)](https://pkg.go.dev/github.com/songzhibin97/go-arthas)
[![Go Version](https://img.shields.io/github/go-mod/go-version/songzhibin97/go-arthas)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> Go-Arthas 是一个受 [Alibaba Arthas](https://github.com/alibaba/arthas) 启发的 Go 应用**诊断工具**。除运行时指标与 pprof 外,它通过**三条互补路线**逼近 Arthas 的方法级诊断:① 只读诊断(`thread`/`flight`,跨平台、零依赖)② 编译期插桩 `watch`/`trace`/`tt`(可生产,需用 go-arthas 构建包装器重编译)③ eBPF 零重启 `attach`(Linux + root)。`jad`/`redefine`/`ognl` 因 Go 无字节码/类加载器/内置表达式引擎,不做对等。详见 [ROADMAP](docs/ROADMAP.md) 与下方[功能对比](#功能对比)。

Go-Arthas 提供运行时指标监控、性能分析与方法级诊断,通过嵌入式 SDK(只读/编译期路线)或 eBPF(零重启路线)集成到目标应用程序。

## 特性

- **轻量级集成**: 只需几行代码即可集成到现有应用程序
- **实时指标**: 监控 goroutine、内存、CPU 和 GC 统计信息
- **性能分析**: 集成 Go 的 pprof，支持 CPU、内存、goroutine 等 profile
- **在线诊断**: `thread`（goroutine 全量 dump + 状态聚合 + 长阻塞启发式）、`flight`（Go 1.25 Flight Recorder 执行轨迹回放）—— 跨平台、零依赖
- **方法级 watch/trace/tt**: 编译期插桩（路线 B，可生产、跨平台，需用 `go-arthas build` 重编译），捕获入参/返回值/panic/耗时/调用栈，运行时经控制面动态开关
- **eBPF 零重启 attach**: 对未经特殊编译的现网进程零重启注入方法级观察（路线 A1，Linux + root + 内核 BTF；uprobe-on-RET，绝不用会崩溃 Go 的 uretprobe）
- **多种接口**: HTTP API、WebSocket 实时更新、CLI 工具、Web Console
- **生产就绪**: 故障安全设计，Agent/诊断失败不影响目标应用
- **跨平台**: 只读诊断与编译期插桩支持 Linux、macOS、Windows（amd64、arm64）；eBPF attach 仅 Linux

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

- Go 1.25+（`flight` 执行轨迹依赖 Go 1.25 的 Flight Recorder；仓库 `go.mod` 为 1.25）
- 支持的操作系统:
  - Linux (amd64, arm64)
  - macOS (amd64, arm64)
  - Windows (amd64, arm64)
- eBPF `attach`（路线 A1）额外要求：Linux + root（或 CAP_BPF + CAP_PERFMON）+ 内核 ≥ 5.15 且启用 BTF（`/sys/kernel/btf/vmlinux`）

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

# 运行环境敏感的性能/资源基准（默认跳过；绝对时序/内存阈值,需在空闲机器上单独运行）
make test-perf            # 等价于 ARTHAS_PERF_TESTS=1 go test -run TestPerformance ./...
```

> eBPF 相关构建/测试需在 Linux 上先生成 bpf2go 产物：`cd ebpf && go generate ./...`（需 clang/libbpf/bpftool + 内核 BTF，详见 [ebpf/README.md](ebpf/README.md)）。CI 已自动处理(见 `.github/workflows/`)。

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

Go-Arthas 受 Alibaba Arthas 启发。Go 在语言层面注定无法复刻 Arthas「纯运行时 attach 任意方法、零重启、跨平台」的单一体验,但通过**编译期插桩 + eBPF + Go 原生能力**三者组合,可逼近其大部分核心命令。**详细对比请查看 [功能对比文档](docs/COMPARISON.md)、[ROADMAP](docs/ROADMAP.md)**。

| 功能类别 | Alibaba Arthas (Java) | Go-Arthas | 说明 |
|---------|----------------------|-----------|------|
| **基础监控** | ✅ | ✅ | CPU、内存、线程/Goroutine、GC |
| **性能分析** | ✅ | ✅ | CPU/内存/goroutine Profile（pprof） |
| **线程/协程栈** | ✅ thread | ✅ `thread` | `runtime.Stack` 全量 dump + 状态聚合 + 长阻塞启发式 |
| **执行轨迹** | ✅ | ✅ `flight` | Go 1.25 Flight Recorder 环形缓冲回放 |
| **方法观察** | ✅ watch | ✅ | 编译期插桩（可生产，需重编译）**或** eBPF `attach`（Linux+root，零重启） |
| **调用追踪** | ✅ trace | ✅ | 同上（入口 OnEnter / `defer` OnExit / `recover` 织入；或每个 RET 挂 uprobe） |
| **方法监控** | ✅ monitor | ✅（基础） | 织入计数（调用次数）；更丰富的成功率/RT 统计为后续 |
| **时间隧道** | ✅ tt | ✅ | 编译期插桩：环形缓冲记录每次调用的入参/返回/耗时快照 |
| **反编译** | ✅ jad | ❌ | Go 编译为机器码、无字节码，不可行 |
| **热更新** | ✅ redefine | ❌ | Go 无类加载器热替换（仅 monkey-patch 测试级），不做 |
| **表达式求值** | ✅ ognl | ❌ | Go 无内置表达式引擎，无法访问进程内运行时变量 |

> 注意:eBPF 路线下复合类型(结构体/接口/string)目前以原始寄存器值暴露,完整解释需结合 DWARF(后续);amd64 实机 attach 待在 x86 Linux 上验证(逻辑有交叉编译单测覆盖)。

### Go-Arthas 的定位

通过**三条互补路线**在 Go 上逼近 Arthas 的方法级诊断:

- **只读诊断**(跨平台、零依赖、零风险):`thread` / `flight` / 指标 / pprof —— 纯 Go 标准库,全平台可用。
- **编译期插桩**(路线 B,可生产、跨平台):`watch` / `trace` / `tt`,代价是目标用 `go-arthas build` 重编译;运行时经控制面动态开关,关闭时开销可忽略。
- **eBPF 零重启 attach**(路线 A1):attach 未经特殊编译的现网进程,代价是 Linux + root + 较新内核(BTF)。

**明确不做**(Go 无对应能力):`jad`(无字节码)、`redefine`(无类加载器热替换)、`ognl`(无内置表达式引擎)。生产级分布式追踪可结合 [OpenTelemetry](https://opentelemetry.io/) / [Jaeger](https://www.jaegertracing.io/)。

详见 [ROADMAP](docs/ROADMAP.md) 与[功能对比文档](docs/COMPARISON.md)。

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
