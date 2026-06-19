# go-arthas eBPF attach（路线 A1）

对**运行中**的 Go 进程零重启 attach，用 eBPF uprobe 做函数级观察——最接近 Java Arthas 的体验，无需重新编译目标程序。

## 为什么绝不用 uretprobe

Go 的可增长/可移动 goroutine 栈与 eBPF uretprobe 根本不兼容：uretprobe 在函数入口劫持栈上返回地址植入 trampoline，栈一移动（morestack/copystack）该地址即失效，触发 `runtime: unexpected return pc` / `fatal error: unknown caller pc` 崩溃。

因此本实现**绝不使用 uretprobe**，而是静态分析目标二进制、定位函数体内每个 `RET` 指令，在每个 RET 偏移上各挂一个 uprobe（Odigos / Pixie 的工业级做法）来观察返回。

## 组成

| 文件 | 作用 | 平台 |
|---|---|---|
| `target.go` | 目标二进制静态分析：ELF 符号定位函数入口/大小、读 Go 构建信息、反汇编定位所有 RET 偏移 | 跨平台（可测） |
| `events.go` | 事件类型、`AttachOptions`、`Attacher` 接口 | 跨平台 |
| `loader_linux.go` | cilium/ebpf 加载器：加载程序、对入口+各 RET 偏移挂 uprobe（bpf cookie 区分函数）、读 ringbuf | Linux |
| `loader_other.go` | 非 Linux stub | 非 Linux |
| `bpf/watch.bpf.c` | eBPF 程序：uprobe 读 Go ABIInternal 寄存器经 ringbuf 上报 | Linux（clang 编译） |

## 读参的 ABI

Go 1.17+（amd64）/ 1.18+（arm64）用寄存器传参（ABIInternal）：amd64 顺序 RAX/RBX/RCX/RDI/RSI/R8…，arm64 R0..R7。`UsesRegisterABI()` 据 Go 版本与架构判断；更早版本走栈（本 MVP 聚焦寄存器 ABI）。

## 环境要求

- Linux，内核 ≥ 5.15（`bpf_get_attach_cookie` 需要），启用 BTF（`/sys/kernel/btf/vmlinux`）。
- root，或 CAP_BPF + CAP_PERFMON。
- 构建期：clang/llvm、libbpf-dev、bpftool、Go 1.25。

## 构建与验证

**非 Linux 主机（macOS/Windows，仅静态分析，不含 eBPF 运行时）：**

```bash
go build ./...        # loader_linux.go 被 build-tag 排除，走 loader_other.go stub，不需要 cilium/clang
go test ./ebpf/       # 二进制分析测试：交叉编译 Linux 目标后解析
```

> ⚠️ **Linux 主机**会编译 `loader_linux.go`，它引用 bpf2go 生成的符号（不入库）。直接
> `go build ./...` 会报 `undefined: watchObjects`，需**先**生成（见下「Linux 完整链路」
> 或 `cd ebpf && go generate ./...`，依赖 clang/libbpf）。

**Linux 完整链路（OrbStack 示例）：**

```bash
orbctl create ubuntu arthas
orb -m arthas sudo bash scripts/ebpf-setup.sh   # 装工具链 + bpf2go 生成 + 构建测试
```

`scripts/ebpf-setup.sh` 会：检查 BTF、装 clang/libbpf/bpftool/Go、用 bpftool 生成 `vmlinux.h`、`go generate`（bpf2go）、`go build`、`go test ./ebpf/`。

## 运行

```bash
go-arthas attach <pid> --list main.                       # 列出含 "main." 的函数符号
sudo go-arthas attach <pid> --func main.handler --duration 30s
```

## MVP 范围与限制

**支持**：未 strip（有符号表）的 Go 二进制、寄存器 ABI、整数/指针类寄存器值、amd64（x86asm）+ arm64（arm64asm）的 RET 定位。

**暂不支持 / 待办**：
- strip 二进制（无符号表）——需 `.gopclntab` 兜底。
- 栈传参（Go < 1.17）。
- 内联函数（符号可能不独立存在）。
- 复合类型（结构体/接口/字符串）目前以**原始寄存器值**暴露；完整解释需结合 DWARF 类型信息按 ABI 拆解（后续）。

> eBPF 运行时部分必须在 Linux + root + clang 环境验证；`target.go` 的静态分析为纯 Go，跨平台可单元测试。
