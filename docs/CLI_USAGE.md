# CLI 使用指南

Go-Arthas CLI 工具提供命令行界面来与运行中的 Agent 交互，查看运行时指标和捕获性能分析数据。

## ⚠️ 重要说明

**功能范围**：CLI 工具专注于**运行时指标查看和性能分析**，提供：
- 查看实时运行时指标（CPU、内存、Goroutine、GC）
- 查看系统信息
- 捕获性能分析数据（CPU、Heap、Goroutine Profile）
- 连接测试

**不提供**（受 Go 语言特性限制）：
- 方法级观察（如 Java Arthas 的 `watch`）
- 调用链追踪（如 Java Arthas 的 `trace`）
- 方法监控统计（如 Java Arthas 的 `monitor`）
- 代码反编译（如 Java Arthas 的 `jad`）
- 代码热更新（如 Java Arthas 的 `redefine`）

**替代方案**：
- 调用链追踪：使用 [OpenTelemetry](https://opentelemetry.io/) 提前埋点
- 方法监控：使用 [Prometheus](https://prometheus.io/) 手动添加 metrics
- 详细对比：查看 [README 功能对比](../README.md#功能对比)

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

- `--host <host:port>`: Agent 地址（默认: `localhost:8563`）
- `--help`: 显示帮助信息
- `--version`: 显示版本信息

## 命令

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

### profile cpu - 捕获 CPU Profile

捕获指定时长的 CPU profile，用于分析 CPU 热点。

**用法**:
```bash
go-arthas profile cpu [--host <host:port>] [--duration <seconds>] [--output <file>]
```

**选项**:
- `--duration <seconds>`: Profile 持续时间（默认: 30 秒）
- `--output <file>`: 输出文件路径（默认: `cpu-<timestamp>.prof`）

**示例**:
```bash
# 捕获 30 秒的 CPU profile
go-arthas profile cpu

# 捕获 60 秒的 CPU profile
go-arthas profile cpu --duration 60

# 指定输出文件
go-arthas profile cpu --output my-cpu-profile.prof
```

**分析 Profile**:
```bash
# 使用 go tool pprof 分析
go tool pprof cpu-20240115-103045.prof

# 生成火焰图
go tool pprof -http=:8080 cpu-20240115-103045.prof
```

### profile heap - 捕获堆 Profile

捕获当前的堆内存快照，用于分析内存分配。

**用法**:
```bash
go-arthas profile heap [--host <host:port>] [--output <file>]
```

**选项**:
- `--output <file>`: 输出文件路径（默认: `heap-<timestamp>.prof`）

**示例**:
```bash
# 捕获堆 profile
go-arthas profile heap

# 指定输出文件
go-arthas profile heap --output my-heap-profile.prof
```

**分析 Profile**:
```bash
# 查看 top 分配
go tool pprof -top heap-20240115-103045.prof

# 交互式分析
go tool pprof heap-20240115-103045.prof

# Web UI
go tool pprof -http=:8080 heap-20240115-103045.prof
```

### profile goroutine - 捕获 Goroutine Profile

捕获当前所有 goroutine 的堆栈跟踪，用于分析 goroutine 泄漏或阻塞。

**用法**:
```bash
go-arthas profile goroutine [--host <host:port>] [--output <file>]
```

**选项**:
- `--output <file>`: 输出文件路径（默认: `goroutine-<timestamp>.prof`）

**示例**:
```bash
# 捕获 goroutine profile
go-arthas profile goroutine

# 指定输出文件
go-arthas profile goroutine --output my-goroutine-profile.prof
```

**分析 Profile**:
```bash
# 查看所有 goroutine
go tool pprof -text goroutine-20240115-103045.prof

# 查看特定状态的 goroutine
go tool pprof -text -focus="runtime.gopark" goroutine-20240115-103045.prof
```

### profile block - 捕获阻塞 Profile

捕获阻塞操作的 profile，用于分析锁竞争和通道阻塞。

**用法**:
```bash
go-arthas profile block [--host <host:port>] [--output <file>]
```

**注意**: 需要在应用程序中启用阻塞 profile：
```go
runtime.SetBlockProfileRate(1)
```

**示例**:
```bash
go-arthas profile block
```

### profile mutex - 捕获互斥锁 Profile

捕获互斥锁竞争的 profile，用于分析锁竞争问题。

**用法**:
```bash
go-arthas profile mutex [--host <host:port>] [--output <file>]
```

**注意**: 需要在应用程序中启用互斥锁 profile：
```go
runtime.SetMutexProfileFraction(1)
```

**示例**:
```bash
go-arthas profile mutex
```

## 使用场景

### 场景 1: 诊断高 CPU 使用率

```bash
# 1. 查看当前 CPU 使用率
go-arthas metrics

# 2. 如果 CPU 使用率高，捕获 CPU profile
go-arthas profile cpu --duration 60

# 3. 分析 profile 找出热点
go tool pprof -top cpu-*.prof
go tool pprof -http=:8080 cpu-*.prof
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
go tool pprof -top heap-*.prof
go tool pprof -http=:8080 heap-*.prof
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
go tool pprof -text goroutine-*.prof
```

### 场景 4: 诊断锁竞争

```bash
# 1. 确保应用程序启用了 mutex profile
# runtime.SetMutexProfileFraction(1)

# 2. 捕获 mutex profile
go-arthas profile mutex

# 3. 分析 profile 找出竞争热点
go tool pprof -top mutex-*.prof
go tool pprof -http=:8080 mutex-*.prof
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

# 捕获 CPU profile
go-arthas profile cpu --duration 30 --output cpu.prof

# 生成报告
go tool pprof -text cpu.prof > cpu-report.txt
go tool pprof -pdf cpu.prof > cpu-report.pdf

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
