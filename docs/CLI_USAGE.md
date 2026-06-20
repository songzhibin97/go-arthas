# CLI 使用指南

Go-Arthas CLI 工具提供命令行界面来与运行中的 Agent 交互，查看运行时指标、捕获性能分析数据，并对方法进行运行时观察。

## ⚠️ 重要说明

**功能范围**：CLI 工具提供以下能力：

无侵入、零代价的运行时观测（连接到正在运行的 Agent 即可使用）：
- 查看实时运行时指标（CPU、内存、Goroutine、GC）—— `metrics`
- 查看系统信息 —— `info`
- 捕获性能分析数据（CPU、Heap、Goroutine Profile）—— `profile`
- Goroutine 转储与长阻塞启发式诊断（对应 Arthas `thread`）—— `thread`
- 执行轨迹飞行记录器（Go 1.25+，对应 `go tool trace`）—— `flight`
- 连接测试 —— `connect`

方法级观察（对应 Java Arthas 的 `watch` / `tt` 时间隧道），通过以下两条路线之一启用，**有代价**：

- **路线 B（编译期插桩，跨平台）**：用 `go-arthas build --targets "pkg.Func,..."` 在编译期对目标函数织入观察点，需要**重新编译并重启**目标程序。运行后用 `methods` 列出可观察方法、`watch` 动态开关并查看时间隧道记录。
- **路线 A1（eBPF 零重启 attach，仅 Linux + root）**：用 `go-arthas attach <pid> --func <name>` 通过 eBPF uprobe 直接观察正在运行的进程，无需重启，但仅在 Linux 上以 root 运行。

**仍不提供**（受 Go 语言特性限制）：
- 方法级动态修改：代码反编译（Java Arthas 的 `jad`）、代码热更新（`redefine`）

**关于调用链追踪**：跨服务的调用链追踪建议使用 [OpenTelemetry](https://opentelemetry.io/) 提前埋点。详细对比见 [README 功能对比](../README.md#功能对比)。

方法级观察的原理与代价详见 [docs/BUILD.md](BUILD.md) 与 [ebpf/README.md](../ebpf/README.md)。

## 安装

### 从源码安装

```bash
go install github.com/songzhibin97/go-arthas/cmd/go-arthas@latest
```

### 从发布版本安装

下载适合你操作系统的预编译二进制文件：

```bash
# Linux (amd64)
wget https://github.com/songzhibin97/go-arthas/releases/latest/download/go-arthas-linux-amd64
chmod +x go-arthas-linux-amd64
sudo mv go-arthas-linux-amd64 /usr/local/bin/go-arthas

# macOS (amd64)
wget https://github.com/songzhibin97/go-arthas/releases/latest/download/go-arthas-darwin-amd64
chmod +x go-arthas-darwin-amd64
sudo mv go-arthas-darwin-amd64 /usr/local/bin/go-arthas

# Windows (amd64)
# 下载 go-arthas-windows-amd64.exe 并添加到 PATH
```

## 基本用法

### 命令格式

```bash
go-arthas <command> [options]
```

### 全局选项

- `--host <host:port>`: Agent 地址（默认: `localhost:8563`），多数与 Agent 通信的命令都支持
- `help` / `--help` / `-h`: 显示帮助信息
- `version` / `--version` / `-v`: 显示版本信息

## 命令

### connect - 连接测试

连接到指定的 Agent 并验证连通性。

**用法**:
```bash
go-arthas connect <host:port>
```

**示例**:
```bash
go-arthas connect localhost:8563
```

成功时输出 `Successfully connected to <host:port>`。

### metrics - 查看运行时指标

显示当前的运行时指标，包括 goroutine 数量、内存使用、CPU 使用率和 GC 统计。

**用法**:
```bash
go-arthas metrics [--host <host:port>]
```

**示例**:
```bash
# 连接到默认地址
go-arthas metrics

# 连接到指定地址
go-arthas metrics --host 192.168.1.100:8563
```

**输出示例**:
```
=== Runtime Metrics ===
Timestamp: 2024-01-15 10:30:45

Goroutines: 42

Memory:
  Heap Allocated:  15.2 MB
  Heap In Use:     18.5 MB
  Heap Idle:       3.8 MB
  Heap Released:   1.2 MB
  Stack In Use:    2.1 MB
  Total Allocated: 125.6 MB
  System Memory:   25.3 MB

CPU:
  Usage: 12.5%

GC:
  Total GC Count:     156
  Total Pause Time:   45.2ms
  Last Pause Time:    0.3ms
  Average Pause Time: 0.29ms
```

### info - 查看系统信息

显示系统信息，包括 Go 版本、操作系统、架构、进程 ID 和运行时长。

**用法**:
```bash
go-arthas info [--host <host:port>]
```

**示例**:
```bash
go-arthas info
```

**输出示例**:
```
=== System Information ===
Go Version:  go1.21.5
OS:          linux
Architecture: amd64
CPU Cores:   8
Process ID:  12345
Start Time:  2024-01-15 09:15:30
Uptime:      1h15m15s
```

> **注意**：`profile` 仅支持 `cpu`、`heap`、`goroutine` 三种类型。
> 输出文件名由工具自动生成（`<type>_profile_<timestamp>.pprof`，保存在当前目录），不支持自定义输出路径。

### profile cpu - 捕获 CPU Profile

捕获指定时长的 CPU profile，用于分析 CPU 热点。

**用法**:
```bash
go-arthas profile cpu [--host <host:port>] [--duration <seconds>]
```

**选项**:
- `--duration <seconds>`: Profile 持续时间（默认: 30 秒）

**示例**:
```bash
# 捕获 30 秒的 CPU profile
go-arthas profile cpu

# 捕获 60 秒的 CPU profile
go-arthas profile cpu --duration 60
```

**分析 Profile**:
```bash
# 使用 go tool pprof 分析（文件名形如 cpu_profile_20240115_103045.pprof）
go tool pprof cpu_profile_20240115_103045.pprof

# 生成火焰图
go tool pprof -http=:8080 cpu_profile_20240115_103045.pprof
```

### profile heap - 捕获堆 Profile

捕获当前的堆内存快照，用于分析内存分配。

**用法**:
```bash
go-arthas profile heap [--host <host:port>]
```

**示例**:
```bash
# 捕获堆 profile
go-arthas profile heap
```

**分析 Profile**:
```bash
# 查看 top 分配
go tool pprof -top heap_profile_20240115_103045.pprof

# 交互式分析
go tool pprof heap_profile_20240115_103045.pprof

# Web UI
go tool pprof -http=:8080 heap_profile_20240115_103045.pprof
```

### profile goroutine - 捕获 Goroutine Profile

捕获当前所有 goroutine 的堆栈跟踪，用于分析 goroutine 泄漏或阻塞。

**用法**:
```bash
go-arthas profile goroutine [--host <host:port>]
```

**示例**:
```bash
# 捕获 goroutine profile
go-arthas profile goroutine
```

**分析 Profile**:
```bash
# 查看所有 goroutine
go tool pprof -text goroutine_profile_20240115_103045.pprof

# 查看特定状态的 goroutine
go tool pprof -text -focus="runtime.gopark" goroutine_profile_20240115_103045.pprof
```

> 如需对 goroutine 做状态聚合与长阻塞快速诊断而无需 pprof，请使用下面的 `thread` 命令。

### thread - Goroutine 转储与阻塞诊断

对应 Java Arthas 的 `thread`。从 Agent 拉取 goroutine 转储，按状态聚合并用启发式标记疑似长时间阻塞的 goroutine。

**用法**:
```bash
go-arthas thread [--host <host:port>] [--full] [--stacks] [--min-wait <minutes>]
```

**选项**:
- `--full`: 打印所有 goroutine 的原始完整堆栈（等价于 `runtime.Stack` 全量文本）
- `--stacks`: 在结构化输出中包含每个 goroutine 的堆栈
- `--min-wait <minutes>`: 将阻塞时间 >= N 分钟的 goroutine 标记为疑似阻塞（默认: 1）

**示例**:
```bash
# 状态聚合 + 疑似阻塞概览
go-arthas thread

# 把阻塞超过 5 分钟的标记为疑似
go-arthas thread --min-wait 5

# 结构化输出中带上各 goroutine 堆栈
go-arthas thread --stacks

# 打印全量原始堆栈
go-arthas thread --full
```

### flight - 执行轨迹飞行记录器

基于 Go 1.25+ 的 Flight Recorder 录制执行轨迹（execution trace），产出可用 `go tool trace` 分析的 trace 文件。

**用法**:
```bash
go-arthas flight <start|snapshot|stop> [--host <host:port>]
```

**子命令**:
- `start`: 启动飞行记录器
- `snapshot`: 下载当前轨迹快照并保存为 `flight_<timestamp>.trace`（当前目录）
- `stop`: 停止飞行记录器

**示例**:
```bash
# 开始录制
go-arthas flight start

# 在问题发生时抓取一段快照
go-arthas flight snapshot

# 停止录制
go-arthas flight stop

# 分析轨迹（文件名形如 flight_20240115_103045.trace）
go tool trace flight_20240115_103045.trace
```

### methods - 列出可观察方法

列出在编译期通过 `go-arthas build` 织入并注册的可观察方法及其 id。`watch` 命令需要这些 id。

**用法**:
```bash
go-arthas methods [--host <host:port>]
```

**示例**:
```bash
go-arthas methods
```

> 该列表仅包含通过路线 B（编译期插桩）织入的方法；未经 `go-arthas build` 重新编译的程序，此处为空。

### watch - 动态开关方法观察 / 查看时间隧道

对应 Java Arthas 的 `watch` 与 `tt`（时间隧道）。在运行时动态开启/关闭某个已织入方法的观察，或查看其已记录的调用。

> **前提**：目标方法必须已通过 `go-arthas build --targets ...` 在编译期织入观察点（见下文 `build`）。方法 id 来自 `methods` 命令。

**用法**:
```bash
go-arthas watch <id> [--off] [--records] [--host <host:port>]
```

**选项**:
- `<id>`: 方法 id（必填，来自 `methods`）
- `--off`: 关闭该方法的观察（默认是开启）
- `--records`: 查看该方法已记录的调用（时间隧道），而不是开关观察

**示例**:
```bash
# 开启某方法的观察
go-arthas watch main.handler

# 关闭观察
go-arthas watch main.handler --off

# 查看已记录的调用（时间隧道）
go-arthas watch main.handler --records
```

### build - 编译期织入观察点构建（路线 B，跨平台）

用编译期插桩（`go build -toolexec`）对指定函数织入观察点后构建目标程序。这是跨平台的方法级观察路线，**代价是需要重新编译并重启目标程序**。

**用法**:
```bash
go-arthas build --targets "pkg.Func,..." [透传给 go build 的参数]
```

**说明**:
- `--targets "pkg.Func,..."`: 逗号分隔的目标函数列表（必填）
- 其余参数原样透传给底层 `go build`（如 `-o`、`-tags`、包路径 `./...` 等）
- 目标二进制必须导入 `arthastrace` 包（导入 go-arthas agent 时会自动引入）

**示例**:
```bash
# 织入两个函数并构建当前目录的程序，输出到 ./app
go-arthas build --targets "main.handler,main.process" -o ./app .

# 之后运行 ./app，再用 methods/watch 进行观察
```

详见 [docs/BUILD.md](BUILD.md)。

### attach - eBPF 零重启观察（路线 A1，仅 Linux + root）

通过 eBPF uprobe 直接挂载到**正在运行**的 Go 进程上观察函数调用，无需重启目标程序。

> **限制**：仅在 **Linux** 上、以 **root** 运行。其它平台不可用。

**用法**:
```bash
go-arthas attach <pid> --func <name> [--func <name> ...] [--list <substr>] [--bin <path>] [--duration <dur>]
```

**选项**:
- `<pid>`: 目标进程 PID（必填）
- `--func <name>`: 要观察的函数符号，可重复指定，例如 `main.handler`
- `--list <substr>`: 列出符号中包含该子串的函数，然后退出（用于查找函数名）
- `--bin <path>`: 目标二进制路径（默认 `/proc/<pid>/exe`）
- `--duration <dur>`: 观察时长，Go duration 格式（默认: `30s`）

**示例**:
```bash
# 先查找包含 handler 的函数符号
sudo go-arthas attach 12345 --list handler

# 观察 main.handler 60 秒
sudo go-arthas attach 12345 --func main.handler --duration 60s

# 同时观察多个函数
sudo go-arthas attach 12345 --func main.handler --func main.process
```

详见 [ebpf/README.md](../ebpf/README.md)。

### version - 查看版本信息

**用法**:
```bash
go-arthas version
```

输出版本号、构建时间和 git commit（源码直接运行时显示默认值 `dev`）。

## 使用场景

### 场景 1: 诊断高 CPU 使用率

```bash
# 1. 查看当前 CPU 使用率
go-arthas metrics

# 2. 如果 CPU 使用率高，捕获 CPU profile
go-arthas profile cpu --duration 60

# 3. 分析 profile 找出热点
go tool pprof -top cpu_profile_*.pprof
go tool pprof -http=:8080 cpu_profile_*.pprof
```

### 场景 2: 诊断内存泄漏

```bash
# 1. 查看当前内存使用
go-arthas metrics

# 2. 等待一段时间后再次查看
sleep 300
go-arthas metrics

# 3. 如果内存持续增长，捕获堆 profile
go-arthas profile heap

# 4. 分析 profile 找出内存分配热点
go tool pprof -top heap_profile_*.pprof
go tool pprof -http=:8080 heap_profile_*.pprof
```

### 场景 3: 诊断 Goroutine 泄漏

```bash
# 1. 查看当前 goroutine 数量
go-arthas metrics

# 2. 等待一段时间后再次查看
sleep 300
go-arthas metrics

# 3. 如果 goroutine 数量持续增长，捕获 goroutine profile
go-arthas profile goroutine

# 4. 分析 profile 找出泄漏的 goroutine
go tool pprof -text goroutine_profile_*.pprof

# 也可以直接用 thread 看状态聚合与疑似长阻塞
go-arthas thread --stacks
```

### 场景 4: 诊断阻塞 / 卡死

```bash
# 1. 转储 goroutine，按状态聚合并标记疑似长阻塞
go-arthas thread

# 2. 把阻塞超过 5 分钟的标记为疑似，并打印其堆栈
go-arthas thread --min-wait 5 --stacks

# 3. 若需细粒度的执行轨迹（调度、系统调用、GC 等），用飞行记录器
go-arthas flight start
# 复现问题后抓取快照
go-arthas flight snapshot
go-arthas flight stop
go tool trace flight_*.trace
```

### 场景 5: 方法级观察（watch / 时间隧道）

```bash
# 路线 B（跨平台，需重新编译并重启目标程序）
go-arthas build --targets "main.handler" -o ./app .
./app &
go-arthas methods                 # 列出可观察方法及 id
go-arthas watch main.handler      # 开启观察
go-arthas watch main.handler --records  # 查看已记录调用（时间隧道）

# 路线 A1（仅 Linux + root，零重启）
sudo go-arthas attach <pid> --func main.handler --duration 60s
```

## 错误处理

### 连接失败

**错误**: `Failed to connect to agent at localhost:8563: connection refused`

**原因**:
- Agent 未启动
- 地址或端口错误
- 防火墙阻止连接

**解决方案**:
```bash
# 检查 Agent 是否运行
curl http://localhost:8563/api/v1/info

# 使用正确的地址
go-arthas metrics --host <correct-host:port>
```

### 指标不可用

**错误**: `Metrics collection is disabled`

**原因**: Agent 配置中 `EnableMetrics` 未设置为 `true`

**解决方案**: 在应用程序中启用指标收集：
```go
config := agent.Config{
    EnableMetrics: true,
}
```

### Profile 不可用

**错误**: `Pprof endpoints are disabled`

**原因**: Agent 配置中 `EnablePprof` 未设置为 `true`

**解决方案**: 在应用程序中启用 pprof：
```go
config := agent.Config{
    EnablePprof: true,
}
```

## 高级用法

### 批量操作

使用 shell 脚本批量执行命令：

```bash
#!/bin/bash
# monitor.sh - 持续监控应用程序

while true; do
    echo "=== $(date) ==="
    go-arthas metrics --host localhost:8563
    echo ""
    sleep 60
done
```

### 远程诊断

通过 SSH 隧道连接到远程 Agent：

```bash
# 建立 SSH 隧道
ssh -L 8563:localhost:8563 user@remote-host

# 在本地连接
go-arthas metrics --host localhost:8563
```

### 自动化分析

结合其他工具自动化分析：

```bash
#!/bin/bash
# auto-profile.sh - 自动捕获和分析 profile

# 捕获 CPU profile（文件自动命名为 cpu_profile_<timestamp>.pprof，保存在当前目录）
go-arthas profile cpu --duration 30

# 取最新生成的 CPU profile 文件
PROF=$(ls -t cpu_profile_*.pprof | head -n1)

# 生成报告
go tool pprof -text "$PROF" > cpu-report.txt
go tool pprof -pdf  "$PROF" > cpu-report.pdf

# 发送报告
mail -s "CPU Profile Report" admin@example.com < cpu-report.txt
```

## 最佳实践

1. **定期监控**: 使用 `metrics` 命令定期检查应用程序健康状况
2. **基线对比**: 记录正常情况下的指标作为基线，便于异常检测
3. **长时间 Profile**: 对于间歇性问题，使用较长的 profile 持续时间（60-120 秒）
4. **多次采样**: 捕获多个 profile 样本以确认问题的一致性
5. **保存 Profile**: 保存 profile 文件以便后续分析和对比
6. **安全访问**: 在生产环境中限制对 Agent 端口的访问

## 参考

- [Go pprof 文档](https://pkg.go.dev/runtime/pprof)
- [Go tool pprof 使用指南](https://github.com/google/pprof/blob/master/doc/README.md)
- [性能分析最佳实践](https://go.dev/blog/pprof)
