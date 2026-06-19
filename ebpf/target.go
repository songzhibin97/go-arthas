// Package ebpf 实现「路线 A1」：对运行中的 Go 进程零重启 attach，用 eBPF
// uprobe 做函数级观察。
//
// 本文件是其中与平台无关的「目标二进制静态分析」部分：解析目标 ELF 的符号表
// 定位函数入口地址与大小、读取 Go 构建信息（版本/架构）、并反汇编函数体定位
// 所有 RET 指令的偏移——后者是关键，因为 Go 的可增长/可移动栈与 eBPF uretprobe
// 根本不兼容（会破坏栈、崩溃进程），必须改为「在每个 RET 指令上各挂一个 uprobe」
// 来观察函数返回（Odigos / Pixie 的工业级做法）。
//
// 该分析不依赖 Linux，可在任意平台对 Linux 目标二进制运行，便于单元测试。
package ebpf

import (
	"debug/buildinfo"
	"debug/elf"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/arch/arm64/arm64asm"
	"golang.org/x/arch/x86/x86asm"
)

// FuncTarget 描述一个待观察函数在目标二进制中的位置
type FuncTarget struct {
	Name       string   // 符号名（如 main.handleRequest 或 net/http.(*Server).Serve）
	EntryAddr  uint64   // 函数入口虚拟地址
	Size       uint64   // 函数机器码字节数
	EntryOff   uint64   // 入口相对 .text 段的文件偏移（备用）
	ReturnOffs []uint64 // 各 RET 指令相对函数入口的偏移（uprobe-on-RET，配 symbol+offset 使用）
}

// TargetBinary 是对目标 Go 二进制的静态分析结果
type TargetBinary struct {
	Path      string
	GoVersion string // 如 go1.25.0
	GOARCH    string // amd64 / arm64
	file      *elf.File
	textBase  uint64 // .text 段虚拟地址
	textOff   uint64 // .text 段文件偏移
	symbols   map[string]elf.Symbol
}

// OpenTarget 打开并分析目标二进制
func OpenTarget(path string) (*TargetBinary, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open elf %s: %w", path, err)
	}

	tb := &TargetBinary{Path: path, file: f, symbols: map[string]elf.Symbol{}}

	text := f.Section(".text")
	if text == nil {
		f.Close()
		return nil, fmt.Errorf("%s: no .text section", path)
	}
	tb.textBase = text.Addr
	tb.textOff = text.Offset

	switch f.Machine {
	case elf.EM_X86_64:
		tb.GOARCH = "amd64"
	case elf.EM_AARCH64:
		tb.GOARCH = "arm64"
	default:
		tb.GOARCH = f.Machine.String()
	}

	syms, err := f.Symbols()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: read symbols (binary may be stripped): %w", path, err)
	}
	for _, s := range syms {
		if elf.ST_TYPE(s.Info) == elf.STT_FUNC && s.Value != 0 {
			tb.symbols[s.Name] = s
		}
	}

	if bi, err := buildinfo.ReadFile(path); err == nil {
		tb.GoVersion = bi.GoVersion
	}

	return tb, nil
}

// Close 释放底层文件
func (tb *TargetBinary) Close() error {
	if tb.file != nil {
		return tb.file.Close()
	}
	return nil
}

// GoMajorMinor 返回 Go 次版本号（如 117 表示 1.17），无法解析时返回 0。
// 用于判断参数传递约定：>=117 走寄存器(ABIInternal)，更早走栈(ABI0)。
func (tb *TargetBinary) GoMajorMinor() int {
	v := strings.TrimPrefix(tb.GoVersion, "go1.")
	if v == tb.GoVersion { // 没有 go1. 前缀
		return 0
	}
	// 取到第一个非数字前的主次部分，如 "25.0" → 25
	minor := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			break
		}
		minor = minor*10 + int(c-'0')
	}
	if minor == 0 {
		return 0
	}
	return 100 + minor // 1.<minor> → 1xx
}

// UsesRegisterABI 报告目标是否使用寄存器调用约定（Go 1.17+ 的 amd64、1.18+ 的 arm64）
func (tb *TargetBinary) UsesRegisterABI() bool {
	mm := tb.GoMajorMinor()
	switch tb.GOARCH {
	case "amd64":
		return mm >= 117
	case "arm64":
		return mm >= 118
	default:
		return mm >= 119 // 其余架构更晚跟进，保守判断
	}
}

// ResolveFunc 定位函数并反汇编其 RET 指令偏移
func (tb *TargetBinary) ResolveFunc(name string) (*FuncTarget, error) {
	sym, ok := tb.symbols[name]
	if !ok {
		return nil, fmt.Errorf("function %q not found in %s", name, tb.Path)
	}
	if sym.Size == 0 {
		return nil, fmt.Errorf("function %q has zero size (assembly/external?)", name)
	}

	// 入口文件偏移 = 函数虚拟地址 - .text 虚拟地址 + .text 文件偏移
	entryOff := sym.Value - tb.textBase + tb.textOff

	code := make([]byte, sym.Size)
	if _, err := tb.file.Section(".text").ReadAt(code, int64(sym.Value-tb.textBase)); err != nil {
		return nil, fmt.Errorf("read code for %q: %w", name, err)
	}

	rets, err := findReturnOffsets(code, tb.GOARCH)
	if err != nil {
		return nil, fmt.Errorf("disassemble %q: %w", name, err)
	}
	retOffs := make([]uint64, len(rets))
	for i, r := range rets {
		retOffs[i] = uint64(r) // 相对函数入口的偏移，供 uprobe symbol+offset 使用
	}

	return &FuncTarget{
		Name:       name,
		EntryAddr:  sym.Value,
		Size:       sym.Size,
		EntryOff:   entryOff,
		ReturnOffs: retOffs,
	}, nil
}

// ListFuncs 返回匹配 substr 的函数符号名（substr 为空则返回全部），按名排序
func (tb *TargetBinary) ListFuncs(substr string) []string {
	var out []string
	for name := range tb.symbols {
		if substr == "" || strings.Contains(name, substr) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// findReturnOffsets 反汇编机器码，返回所有 RET 指令相对函数入口的偏移
func findReturnOffsets(code []byte, arch string) ([]int, error) {
	switch arch {
	case "amd64":
		return findReturnOffsetsAMD64(code)
	case "arm64":
		return findReturnOffsetsARM64(code)
	default:
		return nil, fmt.Errorf("unsupported arch %q for return scanning", arch)
	}
}

func findReturnOffsetsAMD64(code []byte) ([]int, error) {
	var rets []int
	for pc := 0; pc < len(code); {
		inst, err := x86asm.Decode(code[pc:], 64)
		if err != nil || inst.Len == 0 {
			// amd64 是变长指令：解码失败意味着线性反汇编已与真实指令边界**错位**，
			// 此后的偏移都不可信。对安全攸关的 uprobe-on-RET 绝不能靠 pc++ 猜着继续——
			// 错位的 RET 会把 uprobe 的 INT3(0xCC) 写进某条真实指令的中部，破坏指令、
			// 崩溃目标进程（正是当初否决 uretprobe 的同一类崩溃）。宁可报错，让调用方
			// 知道该函数无法安全分析（通常是手写汇编/ABI0），也不挂可能错位的探针。
			return nil, fmt.Errorf("decode failed at byte offset %d; cannot reliably locate RET (hand-written assembly or non-code bytes?)", pc)
		}
		if inst.Op == x86asm.RET {
			rets = append(rets, pc)
		}
		pc += inst.Len
	}
	return rets, nil
}

func findReturnOffsetsARM64(code []byte) ([]int, error) {
	var rets []int
	for pc := 0; pc+4 <= len(code); pc += 4 {
		inst, err := arm64asm.Decode(code[pc:])
		if err != nil {
			continue
		}
		if inst.Op == arm64asm.RET {
			rets = append(rets, pc)
		}
	}
	return rets, nil
}
