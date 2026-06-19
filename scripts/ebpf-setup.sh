#!/usr/bin/env bash
# 在较新 Linux（root + CAP_BPF + 内核 ≥ 5.15，需 BTF）上搭建 eBPF 工具链，
# 生成 bpf2go binding，并构建/测试 go-arthas 的 eBPF attach（路线 A1）。
#
# 典型用法（OrbStack）：
#   orbctl create ubuntu arthas              # 创建 Linux machine（一次）
#   orb -m arthas sudo bash scripts/ebpf-setup.sh
#
# 之后即可：sudo go-arthas attach <pid> --func <symbol>
set -euo pipefail

GOVER="${GOVER:-1.25.0}"
case "$(uname -m)" in
	aarch64) GOARCH=arm64 ;;
	x86_64)  GOARCH=amd64 ;;
	*) echo "unsupported arch $(uname -m)"; exit 1 ;;
esac

echo "== 检查 BTF（CO-RE 必需）=="
if [ ! -f /sys/kernel/btf/vmlinux ]; then
	echo "ERROR: /sys/kernel/btf/vmlinux 不存在，内核缺少 BTF，无法继续"
	exit 1
fi

echo "== 安装工具链 (clang/llvm/libbpf/bpftool) =="
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y clang llvm libbpf-dev bpftool curl make

echo "== 安装 Go ${GOVER} =="
if ! /usr/local/go/bin/go version 2>/dev/null | grep -q "go${GOVER}"; then
	curl -fsSL "https://go.dev/dl/go${GOVER}.linux-${GOARCH}.tar.gz" -o /tmp/go.tgz
	rm -rf /usr/local/go
	tar -C /usr/local -xzf /tmp/go.tgz
fi
export PATH="$PATH:/usr/local/go/bin"
go version

echo "== 生成 vmlinux.h =="
mkdir -p ebpf/bpf/headers
bpftool btf dump file /sys/kernel/btf/vmlinux format c > ebpf/bpf/headers/vmlinux.h

echo "== bpf2go 生成 eBPF binding =="
( cd ebpf && go generate ./... )

echo "== 构建与测试 =="
go build ./...
go test ./ebpf/ -v

echo ""
echo "== 完成。示例：=="
echo "   # 找一个运行中的 Go 进程 pid，列出其函数符号："
echo "   go-arthas attach <pid> --list main."
echo "   # 观察某函数（需 root）："
echo "   sudo go-arthas attach <pid> --func main.handler --duration 30s"
