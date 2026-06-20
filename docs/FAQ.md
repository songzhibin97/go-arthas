# 常见问题 (FAQ)

## 关于功能

### Q: Go-Arthas 能像 Java Arthas 一样观察方法的入参和返回值吗？

**A**: 能。v0.1.0 通过两条路线实现了方法级 `watch`：

1. **编译期插桩（路线 B，可生产、跨平台）**：用 `go-arthas build --targets "pkg.Func,..."` 重新编译目标程序，织入器在函数入口/出口注入钩子，捕获入参、返回值、panic、耗时与调用栈，运行时经控制面（`go-arthas watch <id>`）动态开关。代价是目标需重编译。
2. **eBPF 零重启 attach（路线 A1，仅 Linux）**：`go-arthas attach <pid> --func <name>` 对未经特殊编译的现网进程零重启注入观察。要求 Linux + root（或 CAP_BPF + CAP_PERFMON）+ 内核 ≥ 5.15 且启用 BTF。

**已知限制**：eBPF 路线下复合类型（结构体/接口/string）目前以原始寄存器值暴露，完整解释需结合 DWARF（后续）；栈传参（Go < 1.17）与 strip 二进制暂不支持；amd64 实机 attach 待在 x86 Linux 上验证。详见 [BUILD](BUILD.md) 与 [ebpf/README](../ebpf/README.md)。

### Q: Go-Arthas 能追踪方法调用链路吗（类似 Java Arthas 的 trace）？

**A**: 能。`trace`（调用耗时/调用栈）与 `watch` 走同一套编译期插桩或 eBPF 路线——出口处的 `defer` 钩子记录耗时，`runtime.Callers` 记录调用来源。用法同上：`go-arthas build` 重编译 + `go-arthas watch <id>` 动态开关，或在 Linux 上 `go-arthas attach`。

生产级**跨进程分布式**追踪仍建议结合 [OpenTelemetry](https://opentelemetry.io/) / [Jaeger](https://www.jaegertracing.io/)，两者与 Go-Arthas 的进程内方法级诊断互补。

### Q: Go-Arthas 能监控单个方法的 QPS 和响应时间吗（类似 Java Arthas 的 monitor）？

**A**: 部分能。编译期插桩会记录每个被观察方法的**调用次数**（`go-arthas methods` 可查），出口钩子记录每次调用的**耗时**（`go-arthas watch <id> --records` 查看快照）。

更丰富的聚合统计（成功率、RT 分位数等）尚未内置，属后续计划。需要长期时序统计时，仍可结合 [Prometheus](https://prometheus.io/) 自行埋点。

### Q: Go-Arthas 能热更新代码吗（类似 Java Arthas 的 redefine）？

**A**: 不能。Go 是静态编译语言，编译后的二进制文件无法修改。

**替代方案**：
- 重新编译和部署
- 使用蓝绿部署或滚动更新减少停机时间

### Q: Go-Arthas 能反编译代码吗（类似 Java Arthas 的 jad）？

**A**: 不能。Go 编译成机器码，无法反编译成可读的源代码。

**替代方案**：
- 查看源代码
- 使用版本控制系统确认代码版本

### Q: Go-Arthas 还有哪些 Java Arthas 的功能不做？

**A**: 只有三个命令因 Go 语言没有对应能力而**明确不做对等**，这不是项目缺陷：

| 命令 | 不做原因 |
|------|---------|
| `jad`（反编译） | Go 编译为机器码、无字节码 |
| `redefine`（热替换） | Go 无类加载器热替换（仅 monkey-patch，测试级） |
| `ognl`（表达式求值） | Go 无内置表达式引擎，无法访问进程内运行时变量 |

其余核心命令（`thread`/`watch`/`trace`/`tt` 等）均已通过三条路线逼近，详见下一问。

### Q: 那 Go-Arthas 到底能做什么？

**A**: Go-Arthas 通过**三条互补路线**在运行时监控之外逼近 Arthas 的方法级诊断：

**只读诊断（跨平台、零依赖、零风险）**：
- 实时监控 CPU、内存、Goroutine、GC，WebSocket 实时推送
- 捕获性能分析数据（CPU、Heap、Goroutine Profile）
- `thread`：goroutine 全量 dump + 状态聚合 + 长阻塞启发式
- `flight`：Go 1.25 Flight Recorder 执行轨迹回放

**编译期插桩（路线 B，可生产、跨平台，需 `go-arthas build` 重编译）**：
- `watch`/`trace`/`tt`：捕获入参/返回值/panic/耗时/调用栈，运行时经控制面动态开关

**eBPF 零重启 attach（路线 A1，仅 Linux + root + BTF）**：
- `attach`：对未经特殊编译的现网进程零重启注入方法级观察

**明确不做**：`jad`、`redefine`、`ognl`（Go 无对应能力）。

还有：友好的 CLI 和 Web Console。详见 [功能对比文档](COMPARISON.md) 与 [ROADMAP](ROADMAP.md)。

## 关于使用

### Q: 如何集成 Go-Arthas 到我的应用？

**A**: 只需几行代码：

```go
import "github.com/songzhibin97/go-arthas/agent"

func main() {
    agent.Start(agent.Config{
        Port: 8563,
        EnablePprof: true,
        EnableMetrics: true,
    })
    defer agent.Stop()
    
    // 你的应用代码
}
```

### Q: Go-Arthas 对性能有影响吗？

**A**: 影响非常小：
- CPU 开销 < 0.2%（实测）
- 内存开销 < 14 MB（实测）
- GC 暂停时间 < 100 µs

远低于设计目标（CPU < 5%，内存 < 50 MB）。

### Q: Go-Arthas 可以在生产环境使用吗？

**A**: 可以。Go-Arthas 设计为生产就绪：
- 故障隔离：Agent 崩溃不影响主程序
- 低开销：性能影响可忽略
- 优雅关闭：支持平滑停止
- 完善的错误处理和 Panic 恢复

### Q: 如何排查 Goroutine 泄漏？

**A**: 使用 Go-Arthas 的步骤：

1. 打开 Web Console，观察 Goroutine 趋势图
2. 如果发现持续增长，捕获 goroutine profile：
   ```bash
   go-arthas profile goroutine --host localhost:8563
   ```
3. 分析 profile：
   ```bash
   go tool pprof goroutine_profile.pprof
   top 10  # 查看 goroutine 最多的函数
   ```

### Q: 如何排查内存泄漏？

**A**: 使用 Go-Arthas 的步骤：

1. 打开 Web Console，观察内存面板
2. 如果发现内存持续增长，捕获两次 heap profile：
   ```bash
   go-arthas profile heap --host localhost:8563
   # 等待 5 分钟
   go-arthas profile heap --host localhost:8563
   ```
3. 对比分析：
   ```bash
   go tool pprof -base heap1.pprof heap2.pprof
   top 10  # 查看内存增长最多的函数
   ```

### Q: 如何用 watch 观察某个方法的入参/返回值（编译期插桩路线）？

**A**: 这条路线可生产、跨平台，代价是目标需用 go-arthas 的构建包装器重编译：

1. 用 `go-arthas build` 替代 `go build`，用 `--targets` 声明要织入的函数（其余参数原样透传给 `go build`）：
   ```bash
   go-arthas build --targets "myapp/svc.(*Server).Handle,myapp/svc.Compute" -o myapp ./cmd/myapp
   ```
   （目标程序需已导入 go-arthas agent，会自动引入 `arthastrace` 运行时包。）
2. 运行重编译后的程序，列出可观察方法：
   ```bash
   go-arthas methods --host localhost:8563
   ```
3. 动态开启/关闭某方法的观察（关闭时开销可忽略）：
   ```bash
   go-arthas watch "myapp/svc.Compute" --host localhost:8563
   go-arthas watch "myapp/svc.Compute" --off
   ```
4. 查看最近若干次调用的入参/返回/耗时快照（对应 Arthas 的 tt 时间隧道）：
   ```bash
   go-arthas watch "myapp/svc.Compute" --records
   ```

详见 [构建指南](BUILD.md)。

### Q: 如何在不重编译的情况下 attach 到现网进程（eBPF 路线）？

**A**: 这条路线零重启，但**仅限 Linux + root + 较新内核（BTF）**：

```bash
# 列出含 "main." 的函数符号
go-arthas attach <pid> --list main.

# 观察指定函数 30 秒（uprobe-on-RET，绝不用会崩溃 Go 的 uretprobe）
sudo go-arthas attach <pid> --func main.handler --duration 30s
```

要求：Linux，内核 ≥ 5.15 并启用 BTF（`/sys/kernel/btf/vmlinux`），root 或 CAP_BPF + CAP_PERFMON，目标二进制未 strip（有符号表）。复合类型目前以原始寄存器值暴露；栈传参（Go < 1.17）暂不支持。详见 [ebpf/README](../ebpf/README.md)。

### Q: WebSocket 连接失败怎么办？

**A**: 检查清单：
1. Agent 是否启动？`curl http://localhost:8563/api/v1/info`
2. 端口是否正确？
3. 防火墙是否阻止？
4. 浏览器控制台是否有错误？

### Q: 指标不更新怎么办？

**A**: 可能原因：
1. `EnableMetrics` 未设置为 `true`
2. WebSocket 连接断开
3. Agent 重启

解决方案：
- 检查 Agent 配置
- 刷新页面重新连接
- 查看浏览器控制台错误

## 关于对比

### Q: Go-Arthas 和原生 pprof 有什么区别？

**A**: Go-Arthas 是 pprof 的增强版：

| 功能 | 原生 pprof | Go-Arthas |
|------|-----------|-----------|
| Profile 分析 | ✅ | ✅ |
| 实时指标监控 | ❌ | ✅ |
| WebSocket 推送 | ❌ | ✅ |
| CLI 工具 | ❌ | ✅ |
| Web Console | 简陋 | 完善 |
| CPU 使用率 | ❌ | ✅ |
| 系统信息 | ❌ | ✅ |

### Q: Go-Arthas 和 Prometheus 有什么区别？

**A**: 两者定位不同，可以互补：

| 特性 | Go-Arthas | Prometheus |
|------|-----------|-----------|
| 定位 | 实时诊断工具 | 长期监控系统 |
| 数据存储 | 不存储 | 时序数据库 |
| 告警 | 不支持 | 支持 |
| 性能分析 | 支持（pprof） | 不支持 |
| 部署 | 嵌入应用 | 独立部署 |
| 学习曲线 | 低 | 中等 |

**推荐组合**：
- Go-Arthas：实时诊断和性能分析
- Prometheus：长期监控和告警

### Q: Go-Arthas 实现了 Alibaba Arthas 多少功能？

**A**: 覆盖了大部分核心命令，仅三个命令明确不做对等。

**已实现**：
- 基础监控、性能分析、实时推送、CLI 和 Web Console
- 只读诊断：`thread`、`flight`
- 方法级诊断：`watch`/`trace`/`tt`（编译期插桩，可生产）+ `attach`（eBPF 零重启，Linux）
- `monitor`：基础调用计数（更丰富的成功率/RT 统计为后续）

**不做对等**（Go 无对应能力）：
- `jad`（无字节码）、`redefine`（无类加载器热替换）、`ognl`（无内置表达式引擎）

详见 [功能对比文档](COMPARISON.md) 与 [ROADMAP](ROADMAP.md)。

## 关于贡献

### Q: 我可以贡献代码吗？

**A**: 当然可以！欢迎贡献：
- Bug 修复
- 文档改进
- 新功能（在 Go 语言能力范围内）
- 测试用例

### Q: watch/trace/monitor 已经支持了吗？

**A**: 是的，v0.1.0 已支持：
- `watch`/`trace`/`tt`：编译期插桩（路线 B，可生产、跨平台，需 `go-arthas build` 重编译），或 eBPF `attach`（路线 A1，Linux + root，零重启）
- `monitor`：编译期插桩记录基础调用计数；更丰富的成功率/RT 统计为后续

实现细节见 `arthastrace/`、`cmd/arthas-toolexec/`、`ebpf/` 与 [ROADMAP](ROADMAP.md)。欢迎贡献相关增强（如 monitor 聚合统计、eBPF 复合类型 DWARF 解析）。

### Q: 有计划支持更多功能吗？

**A**: 见 [ROADMAP](ROADMAP.md)。已落地的可继续增强的方向：
- `monitor` 的成功率/RT 分位数等聚合统计
- eBPF 路线下结合 DWARF 还原复合类型（结构体/接口/string）
- amd64 实机 attach 的验证、strip 二进制（`.gopclntab` 兜底）、栈传参（Go < 1.17）
- 更多的性能分析类型、更好的可视化与导出

`jad`/`redefine`/`ognl` 因 Go 无字节码/类加载器/内置表达式引擎，明确不做对等。

## 其他问题

### Q: Go-Arthas 是否收费？

**A**: 完全免费，采用 MIT 许可证。

### Q: Go-Arthas 是否稳定？

**A**: 是的。项目包含：
- 17 个测试文件
- 单元测试、属性测试、E2E 测试
- 测试覆盖率 > 90%
- 在生产环境中使用

### Q: 如何报告 Bug？

**A**: 在 GitHub 上提交 Issue：
https://github.com/songzhibin97/go-arthas/issues

### Q: 如何获取帮助？

**A**: 
1. 查看文档：[README](../README.md)、[功能对比](COMPARISON.md)
2. 查看示例：[examples/simple](../examples/simple)
3. 提交 Issue：[GitHub Issues](https://github.com/songzhibin97/go-arthas/issues)
4. 参与讨论：[GitHub Discussions](https://github.com/songzhibin97/go-arthas/discussions)

### Q: 项目名字会改吗？

**A**: 目前没有改名计划。虽然功能范围与 Alibaba Arthas 不同，但：
- "Arthas" 体现了灵感来源
- 通过文档明确说明功能范围
- 避免用户产生不切实际的期望

### Q: 为什么不叫 "go-pprof-plus" 或其他名字？

**A**: 
- "Arthas" 在 Java 圈很有名，能吸引关注
- 名字体现了设计灵感和传承关系
- 通过文档说明可以避免误导

## 总结

Go-Arthas 是一个受 Alibaba Arthas 启发的 Go 应用**诊断工具**，通过三条互补路线逼近其方法级诊断，但不追求 1:1 对等。

**正确的期望**：
- 比原生 pprof 更好用，实时监控运行时指标，方便的性能分析，友好的用户界面
- 只读诊断：`thread`、`flight`（跨平台、零依赖）
- 方法级诊断：`watch`/`trace`/`tt`（编译期插桩，需 `go-arthas build` 重编译）+ `attach`（eBPF 零重启，仅 Linux + root）

**不要期望**（Go 无对应能力，明确不做）：
- 反编译（`jad`）、代码热更新（`redefine`）、动态表达式求值（`ognl`）

这不是缺陷，而是 Go 语言的特性。
