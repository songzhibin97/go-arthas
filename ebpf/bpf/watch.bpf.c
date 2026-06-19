//go:build ignore

// watch.bpf.c —— go-arthas 路线 A1 的 eBPF 程序。
//
// 在目标 Go 函数的入口与每个 RET 指令处各挂一个 uprobe（严禁 uretprobe：Go 的
// 可增长/可移动栈与 uretprobe 根本不兼容，会破坏栈、崩溃进程）。每次触发读取
// Go ABIInternal 的整数寄存器并经 ringbuf 上报；用 bpf cookie 区分是哪个函数。
//
// 需要 vmlinux.h（bpftool btf dump 生成）与 libbpf headers，由 bpf2go 编译。

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

char __license[] SEC("license") = "GPL";

#define MAX_REGS 6

// 与 Go 端 rawEvent 内存布局严格对应（8 字节对齐字段在前）
struct event {
	__u64 ktime;
	__u64 cookie;
	__u64 regs[MAX_REGS];
	__u32 pid;
	__u32 tid;
	__u8  kind; // 0=enter, 1=exit
	__u8  _pad[7];
};

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 1 << 22); // 4MB
} events SEC(".maps");

static __always_inline int emit(struct pt_regs *ctx, __u8 kind) {
	struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
	if (!e)
		return 0;

	__u64 pid_tgid = bpf_get_current_pid_tgid();
	e->pid = pid_tgid >> 32;
	e->tid = (__u32)pid_tgid;
	e->kind = kind;
	e->ktime = bpf_ktime_get_ns();
	e->cookie = bpf_get_attach_cookie(ctx);

#if defined(__TARGET_ARCH_x86)
	// Go ABIInternal (amd64) 整数寄存器顺序：RAX, RBX, RCX, RDI, RSI, R8 ...
	e->regs[0] = ctx->ax;
	e->regs[1] = ctx->bx;
	e->regs[2] = ctx->cx;
	e->regs[3] = ctx->di;
	e->regs[4] = ctx->si;
	e->regs[5] = ctx->r8;
#elif defined(__TARGET_ARCH_arm64)
	// Go ABIInternal (arm64) 整数寄存器顺序：R0..R7（取前 6 个）
	e->regs[0] = ctx->regs[0];
	e->regs[1] = ctx->regs[1];
	e->regs[2] = ctx->regs[2];
	e->regs[3] = ctx->regs[3];
	e->regs[4] = ctx->regs[4];
	e->regs[5] = ctx->regs[5];
#endif

	bpf_ringbuf_submit(e, 0);
	return 0;
}

SEC("uprobe")
int uprobe_enter(struct pt_regs *ctx) {
	return emit(ctx, 0);
}

SEC("uprobe")
int uprobe_exit(struct pt_regs *ctx) {
	return emit(ctx, 1);
}
