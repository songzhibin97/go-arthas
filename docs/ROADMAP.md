# Go-Arthas Roadmap：逼近 Java Arthas 的方法级诊断

> 本文档基于一次多源、对抗式验证的技术调研（127 条 claim 抽取 → 25 条核心验证 → 23 确认 / 2 否决）整理而成，给出 go-arthas 从「指标面板」演进为「真正诊断工具」的分阶段路线。

## 1. 背景与核心结论

**一句话结论**：Go 在语言层面**注定无法复刻** Arthas「零重启、纯运行时 attach 任意方法」的体验，且 Go 官方明确不会支持；但通过**编译期插桩**（可生产）+ **eBPF**（Linux 零重启）+ **Go 原生能力**三者组合，可以逼近 Arthas 大部分核心命令。`jad / redefine / ognl` 三个命令在 Go 上无对应能力，**放弃对等**。

### 1.1 两个根因（均经 3-0 验证）

1. **可移动 goroutine 栈**：Go 栈初始仅 ~2KB，`morestack` 时整栈复制并重写指针。eBPF 的 `uretprobe`（返回探针）在函数入口劫持栈上返回地址，栈一移动地址即失效，触发 `runtime: unexpected return pc` / `fatal error: unknown caller pc` 崩溃。这是 JVM（固定栈 + 字节码注入）与 Go 的本质差异。
2. **寄存器调用约定**：Go 1.17 起参数走寄存器（amd64: RAX/RBX/RCX…，arm64 在 1.18、riscv64 在 1.19 跟进），1.17 前走栈。导致 Frida 等标准 C-ABI hook 工具开箱不可用，读参必须按 Go 版本 + 架构区分。

### 1.2 Go 官方态度

Ian Lance Taylor 在 [golang/go#22008](https://github.com/golang/go/issues/22008) 明确：Go 承诺可移动栈，看不出如何支持引用栈上地址的动态探针。该 issue 自 2017 年 open 至今未解。**不要指望改语言**，现实路径只能走 eBPF 生态与编译期插桩。

## 2. 设计原则

- **故障安全优先**：沿用现有「Agent 失败不拖垮宿主」契约——每条新路线都必须在 panic/不支持时优雅降级，绝不影响目标应用。
- **跨平台分级降级**：跨平台能力（Go 原生）作为基线全平台可用；Linux 专属能力（eBPF）作为增强；不可移植方案（monkey-patch）仅限测试。
- **不追求 1:1 对等**：明确放弃 Go 无法干净实现的命令，避免引入测试级技术到生产。

## 3. 能力地图（Arthas 命令 → Go 对应 → 现状 → 落地阶段）

| Arthas 命令 | Go 对应 | go-arthas 现状 | 落地阶段 |
|---|---|---|:---:|
| `dashboard` | runtime metrics | ✅ 已有 | — |
| profiler / `profile` | pprof | ✅ 已有（/debug/pprof） | — |
| `thread`（线程/协程栈） | `runtime.Stack(all=true)` | ❌ 缺，Go 原生支持 | **阶段 1** |
| `thread -b`（死锁检测） | goroutine dump 分析 | ❌ 缺 | **阶段 1** |
| 执行轨迹回放 | `runtime/trace` Flight Recorder（Go 1.25） | ❌ 缺 | **阶段 1** |
| `watch`（入参/返回/异常） | OnEnter/OnExit/recover 织入 | ❌ 缺 | **阶段 2**（B）或 阶段 3（A1） |
| `trace`（调用链耗时） | defer 计时织入 | ❌ 缺 | **阶段 2** 或 阶段 3 |
| `monitor`（调用统计） | 计数器织入 | ❌ 缺 | **阶段 2** 或 阶段 3 |
| `stack`（调用来源） | 织入点 + `runtime.Callers` | ❌ 缺 | **阶段 2** 或 阶段 3 |
| `tt`（时间隧道） | 每次调用快照记录 | ❌ 缺 | **阶段 2** 或 阶段 3 |
| `jad`（反编译） | 无字节码 | ❌ 不可行 | 放弃 |
| `redefine`（热替换） | 无类加载器；仅 monkey-patch（测试级） | ❌ 不可行 | 放弃 |
| `ognl`（表达式求值） | 无内置表达式引擎 | ❌ 不可行 | 放弃 |

## 4. 路线对比（供阶段 2/3 选型）

| 路线 | 零重启 | 生产可用 | 跨平台 | 能逼近的命令 | 代表项目 |
|---|:---:|:---:|:---:|---|---|
| **A1. eBPF uprobe-on-RET** | ✅ | ✅（Linux+root） | ❌ 仅 Linux | watch/trace/monitor/stack/tt | Odigos、Pixie、DeepFlow |
| **A2. monkey-patch** | ✅ | ❌ 仅测试 | ❌ 限 x86 | redefine（替换整函数） | gomonkey、gohook |
| **A3. Delve / ptrace** | ✅ | ⚠️ 未充分验证 | ⚠️ | watch（断点式） | Delve API |
| **B. 编译期 -toolexec + AST** | ❌ 需重编译 | ✅✅ 最稳 | ✅ | watch/trace/monitor/stack/tt | orchestrion、阿里 otel |
| **C. 魔改 runtime/内核** | 视方案 | ❌ 维护地狱 | ❌ | 理论全部 | 无成熟项目 |

**关键约束（均经验证）**：
- **A1 是 eBPF 唯一可生产姿势**：绝不能用 uretprobe，要静态分析二进制定位**每个 RET 指令各挂一个 uprobe**（Odigos/Pixie 工业级做法）。
- **A2 硬伤**：内联开启时静默失败（须 `-gcflags=all=-l`）、**非线程安全**（并发调用被 patch 函数会 panic）、W^X 加固系统不可用、ARM 基本不支持；`bouk/monkey` 已归档，`gohook` 2020 后无更新。**只能测试，绝不上生产**。
- **B 最优**：`orchestrion`（Datadog）与`阿里 opentelemetry-go-auto-instrumentation`（已进 OTel SIG）均用 `-toolexec` 拦截编译器、AST 重写，入口注入 `OnEnter` 捕获入参、`defer OnExit` 捕获返回值、`recover` 捕 panic——正好是 `watch` 语义。用法只是把 `go build` 换成带前缀的命令。

---

## 阶段 0：修复与固本（前置，必做）

> 路线图开始前先修复现有缺陷，否则发布流水线不可用、并发路径有竞态。

**任务**
- [ ] 修 P0：`Makefile build-cli` 与 `scripts/release.sh` 构建 `cli/main.go`（`package cli` 单文件，必失败），改为 `./cmd/go-arthas`。
- [ ] 修 WebSocket 同一连接的 ping goroutine 与广播 goroutine 并发写 `*websocket.Conn`（gorilla 禁止并发写）：加 per-conn 写锁。
- [ ] `wsManager.run()` 锁外读 `len(clients)` 的竞态；全量跑 `go test -race`。
- [ ] 文档/代码一致性：空的 `docs/BUILD.md`、CLI 缺 `--version`、`release.sh` 的死版本戳 `-X main.Version`、占位 `your-org/go-arthas`。
- [ ] 清理死代码：`agent.mu`、`cpuStatsWarned`、`cli/format.go` 的 `printTable/printRow/printSeparator`。

**验收**：`make build` 与 `release.sh` 可产出全平台二进制；`go test -race ./...` 通过。

---

## 阶段 1：只读诊断做满（跨平台 · 零依赖 · 零风险）

> 用 Go 原生能力把「不需要方法级 hook」的 Arthas 高频命令全部补齐。性价比最高，且不碰任何危险技术。

**目标**：go-arthas 从「指标面板」升级为「在线诊断工具」。

**任务**
- [ ] `thread`：`runtime.Stack(buf, true)` 全 goroutine 栈 dump；按状态（running/runnable/IO wait/select…）聚合计数；支持按 goroutine ID/栈关键字过滤。
- [ ] `thread -b`：死锁/长时间阻塞检测（分析 goroutine dump，识别互相等待与超长 wait）。
- [ ] 在线 heap / goroutine / allocs profile 的交互式拉取与 Top-N 展示（在现有 `/debug/pprof` 之上做结构化 API + Web 可视化）。
- [ ] `GODEBUG` 派生指标：GC trace、`schedtrace`/`scheddetail` 调度延迟。
- [ ] **Flight Recorder**：接入 Go 1.25 正式的 `runtime/trace.FlightRecorder`，做「故障前 N 秒执行轨迹」环形缓冲回放——Go 官方在可观测性上的主推方向。
- [ ] 新增 HTTP 端点 + CLI 子命令 + Web 面板，复用现有 Agent/CLI/Console 架构。

**技术路线**：纯 Go 标准库（`runtime`、`runtime/trace`、`runtime/pprof`），无外部依赖，全平台可用。

**交付物**：`/api/v1/thread`、`/api/v1/trace/flight` 等端点；`go-arthas thread` 等子命令；Web 新增协程/死锁/轨迹面板。

**验收**：在示例应用上能 dump 全部 goroutine 栈、识别人为构造的死锁、抓取并下载 flight trace；全平台编译通过；Agent 故障安全契约不破坏。

**风险**：低。`runtime.Stack(all=true)` 会 STW 短暂停顿，需注意大并发下的开销（文档化、限频）。

---

## 阶段 2：方法级 watch / trace（编译期插桩 · 可生产）— 路线 B

> 这是在生产环境获得 `watch/trace/monitor/stack/tt` 的最稳妥路径，代价是目标应用需用 go-arthas 的构建包装器编译。

**目标**：提供编译期织入的方法级观察，运行时通过 go-arthas 通道动态开关。

**任务**
- [ ] 实现 `-toolexec` 织入器（自研，或集成阿里 `opentelemetry-go-auto-instrumentation` 机制）：构建期拦截 `go tool compile`，对配置选中的函数做 AST 重写。
- [ ] 注入模板：入口 `OnEnter`（捕获入参）、`defer OnExit`（捕获返回值、计时）、`recover`（捕 panic/异常）、`runtime.Callers`（调用来源）。
- [ ] 运行时控制面：通过现有 HTTP/WS 通道**动态开关**某方法的观察（织入时广撒采集点，运行时按开关过滤），逼近 Arthas 交互体验。
- [ ] `tt`（时间隧道）：记录每次调用的入参/返回快照到环形缓冲，支持回放。
- [ ] 配置：按包/函数/方法选择器（glob 或正则）声明织入点。
- [ ] 构建包装器：`go-arthas build ...` 透传 `-toolexec`，文档化使用方式。

**技术路线**：路线 B（`-toolexec` + AST 重写）。参考 `orchestrion`、阿里 `otel-auto-instrumentation`。

**交付物**：织入器二进制；构建包装命令；运行时方法观察的 API/CLI/Web；织入配置格式。

**验收**：对示例应用选定函数，重新编译后能在运行时动态开启/关闭 watch，正确捕获标量与常见复合类型的入参/返回/panic，并输出调用耗时与调用栈；关闭时性能开销可忽略。

**风险/开放问题**：
- 编译期是**静态织入**，要做到 Arthas「运行时任选任意方法」需织入时广撒点 + 运行时过滤，存在性能权衡——需基准测试并文档化默认织入范围。
- 复合类型（接口/嵌套结构体/map）的序列化深度与体积需限制。

---

## 阶段 3：Linux 零重启 attach（eBPF）— 路线 A1

> 最接近 Arthas 体验：attach 到**未经特殊编译的现网进程**。代价是 Linux + root + 较新内核，且实现复杂。

**目标**：对运行中的 Go 进程零重启注入方法级观察。

**任务**
- [ ] eBPF 加载器：`cilium/ebpf`（纯 Go，无需 CGO/BCC）。
- [ ] 二进制静态分析：解析目标 ELF/符号表/DWARF，定位目标函数入口与**所有 RET 指令**。
- [ ] **uprobe-on-RET**：入口挂 uprobe 捕获 OnEnter，每个 RET 各挂一个 uprobe 捕获 OnExit（**严禁 uretprobe**）。
- [ ] 按 Go 版本 + 架构读参：1.17+ 从寄存器（amd64 RAX/RBX/RCX…、arm64…）、1.17- 从栈。
- [ ] 能力探测与降级：内核版本/权限/符号缺失时优雅回退，并提示改用阶段 2。

**技术路线**：路线 A1。参考 Odigos、Pixie、DeepFlow。

**交付物**：`go-arthas attach <pid>` 的 eBPF 模式；Linux 专属诊断通道。

**验收**：在 Linux（root + 较新内核）上 attach 到示例进程，无需重编译即可观察选定函数的入参/返回/耗时，且**真实负载下不崩溃**（验证可移动栈场景下的稳定性）。

**风险/开放问题**：
- 复杂结构体/接口/string 等复合类型在寄存器中的还原是难点（见开放问题）。
- 仅 Linux；需 root/CAP_BPF 与较新内核；符号被 strip 时依赖 DWARF/外部符号。

---

## 5. 明确不做

- **`jad` / `redefine` / `ognl`**：Go 无字节码、无类加载器热替换、无内置表达式引擎，强行对等只会引入脆弱方案。（`ognl` 若确有需求，仅可嵌入 `expr`/`yaegi` 解释器跑独立脚本，但**无法访问进程内运行时变量**，与 Arthas 语义不同。）
- **生产环境用 monkey-patch**：非线程安全、需关内联、W^X 不可用、ARM 受限，仅限测试。
- **eBPF uretprobe**：直接崩溃 Go 进程，永不使用。
- **依赖 `//go:linkname`**：官方持续收紧，不作为长期方案基石。
- **魔改 Go runtime / 内核（路线 C）**：维护成本与分发成本过高，不纳入路线。

## 6. 待验证 / 开放问题

- **Delve / ptrace 路线（A3）**：本轮一级证据不足。若希望「零重启 + 跨平台 + 不写 eBPF」，需单独验证其性能与生产可用性。
- **eBPF 复合类型读取**：不重编译下，uprobe-on-RET + 寄存器读参能否稳定还原结构体/接口/string，生产负载下的可靠性与开销？
- **编译期插桩的运行时交互性**：阶段 2 如何弥合「构建期静态织入」与「Arthas 运行时任选方法」之间的差距？
- **DWARF + `go:linkname` 驱动的按方法名动态定位**能力边界，以及 `go:linkname` 收紧的影响。

## 7. 促成官方支持的现实方向

- **改语言无望**：可移动栈是 Go 性能基石，官方立场稳定 8 年，不会为动态探针让步。
- **可参与的方向**：① 关注/反馈 `runtime/trace` Flight Recorder API 演进；② 推动/贡献 eBPF 侧工具链（uprobe-on-RET 自动化、复合类型读取）；③ 编译期插桩标准化（orchestrion 已牵动 [golang/go#69887](https://github.com/golang/go/issues/69887) 关于 toolexec 的讨论）。

## 8. 参考来源（一级为主）

- Go 可移动栈与动态探针不兼容：[golang/go#22008](https://github.com/golang/go/issues/22008)、[iovisor/bcc#1320](https://github.com/iovisor/bcc/issues/1320)、[cilium/ebpf#759](https://github.com/cilium/ebpf/discussions/759)
- eBPF uprobe-on-RET 与读参：[Odigos how-it-works](https://github.com/odigos-io/opentelemetry-go-instrumentation/blob/master/docs/how-it-works.md)、[Pixie eBPF function tracing](https://blog.px.dev/ebpf-function-tracing/)
- 寄存器 ABI 障碍：[Quarkslab: hooking Golang](https://blog.quarkslab.com/lets-go-into-the-rabbit-hole-part-1-the-challenges-of-dynamically-hooking-golang-program.html)、[golang/go#40724](https://github.com/golang/go/issues/40724)
- 编译期插桩：[DataDog/orchestrion](https://github.com/DataDog/orchestrion)、[阿里 opentelemetry-go-auto-instrumentation](https://github.com/alibaba/opentelemetry-go-auto-instrumentation)、[OTel 编译期插桩博客](https://opentelemetry.io/blog/2025/go-compile-time-auto-instrumentation/)
- monkey-patch 限制：[bouk/monkey](https://github.com/bouk/monkey)、[gomonkey](https://github.com/agiledragon/gomonkey)、[gohook](https://github.com/brahma-adshonor/gohook)、[funchook](https://github.com/kubo/funchook)
- 官方可观测性方向：[Go Flight Recorder](https://go.dev/blog/flight-recorder)、[golang/go#69887](https://github.com/golang/go/issues/69887)
