# 常见问题 (FAQ)

## 关于功能

### Q: Go-Arthas 能像 Java Arthas 一样观察方法的入参和返回值吗？

**A**: 不能。Go 语言的反射能力有限，无法在运行时拦截函数调用。

**替代方案**：
- 手动添加日志记录入参和返回值
- 使用 OpenTelemetry 进行分布式追踪

### Q: Go-Arthas 能追踪方法调用链路吗（类似 Java Arthas 的 trace）？

**A**: 不能。这需要字节码增强技术，而 Go 编译成机器码后无法动态修改。

**替代方案**：
- 使用 [OpenTelemetry](https://opentelemetry.io/) 提前埋点
- 使用 [Jaeger](https://www.jaegertracing.io/) 进行分布式追踪

### Q: Go-Arthas 能监控单个方法的 QPS 和响应时间吗（类似 Java Arthas 的 monitor）？

**A**: 不能。无法动态统计单个函数的调用情况。

**替代方案**：
- 使用 [Prometheus](https://prometheus.io/) 手动添加 metrics
- 在函数中添加计数器和直方图

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

### Q: 为什么 Go-Arthas 缺少这么多功能？

**A**: 这不是项目缺陷，而是 Go 语言的设计理念和技术限制：

| 特性 | Java | Go |
|------|------|-----|
| 运行方式 | JVM 虚拟机（可动态操作） | 编译成机器码（不可修改） |
| 反射能力 | 非常强大 | 有限 |
| 字节码 | 可动态修改 | 无字节码 |

Go 追求简单和性能，牺牲了动态性。

### Q: 那 Go-Arthas 到底能做什么？

**A**: Go-Arthas 专注于**运行时监控和性能分析**：

**能做的**：
- 实时监控 CPU、内存、Goroutine、GC
- 捕获性能分析数据（CPU、Heap、Goroutine Profile）
- WebSocket 实时推送指标
- 友好的 CLI 和 Web Console

**不能做的**：
- 方法级诊断（watch、trace、monitor）
- 代码操作（jad、redefine）
- 动态表达式（ognl）

详见 [功能对比文档](COMPARISON.md)。

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

**A**: 约 30%。

**已实现**（30%）：
- 基础监控
- 性能分析
- 实时推送
- CLI 和 Web Console

**无法实现**（70%）：
- 方法级诊断
- 代码操作
- 动态表达式

详见 [功能对比文档](COMPARISON.md)。

## 关于贡献

### Q: 我可以贡献代码吗？

**A**: 当然可以！欢迎贡献：
- Bug 修复
- 文档改进
- 新功能（在 Go 语言能力范围内）
- 测试用例

### Q: 能否添加 watch/trace/monitor 功能？

**A**: 很遗憾，不能。这些功能需要：
- 字节码增强（Go 没有字节码）
- 强大的反射能力（Go 反射有限）
- 动态代码修改（Go 是静态编译）

这是 Go 语言的根本限制，不是实现问题。

### Q: 有计划支持更多功能吗？

**A**: 在 Go 语言能力范围内，可能的改进：
- 更多的性能分析类型
- 更好的可视化
- 更多的导出格式
- 更好的用户体验

但方法级诊断功能由于语言限制，无法实现。

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

Go-Arthas 是一个**运行时监控和性能分析工具**，不是 Alibaba Arthas 的完整移植。

**正确的期望**：
- 比原生 pprof 更好用
- 实时监控运行时指标
- 方便的性能分析
- 友好的用户界面

**不要期望**：
- 方法级诊断
- 代码热更新
- 反编译

这不是缺陷，而是 Go 语言的特性。
