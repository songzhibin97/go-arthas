# 构建指南

本文档说明如何从源码构建 Go-Arthas 的各个组件。

## 前置要求

- Go 1.18 或更高版本（阶段性功能如 Flight Recorder 需要 Go 1.25+）
- Node.js 18+ 与 npm（仅构建 Web Console 时需要）
- make（可选，用于 Makefile 快捷目标）

## 快速构建

```bash
make build        # 构建全部（CLI + Web Console）
make build-cli    # 仅构建 CLI，产物：bin/go-arthas
make build-web    # 仅构建 Web Console，产物：web/dist/
```

## 手动构建 CLI

CLI 入口在 `cmd/go-arthas`：

```bash
go build -o bin/go-arthas ./cmd/go-arthas
```

> 注意：`cli/` 是库包（`package cli`），**不能**用 `go build ./cli/main.go` 单文件构建，
> 否则会报 `undefined: NewCLI` 等错误。必须构建 `./cmd/go-arthas`。

> ⚠️ **Linux 构建额外前置**：CLI 依赖 `ebpf` 包，而 `ebpf/loader_linux.go`（`//go:build linux`）
> 引用 bpf2go 生成的符号（`watchObjects` 等），这些产物不入库。**在 Linux 主机上**首次
> 构建前必须先生成：
>
> ```bash
> cd ebpf && go generate ./...   # 需要 clang/libbpf/bpftool，见 ebpf/README.md
> ```
>
> 否则 `go build ./cmd/go-arthas`（及 `make build-cli`）会报 `undefined: watchObjects`。
> 非 Linux 主机（macOS/Windows）走 `loader_other.go` stub，无需此步。

注入版本信息：

```bash
go build -ldflags "\
  -X github.com/songzhibin97/go-arthas/cli.Version=$(git describe --tags --always) \
  -X github.com/songzhibin97/go-arthas/cli.GitCommit=$(git rev-parse --short HEAD)" \
  -o bin/go-arthas ./cmd/go-arthas

bin/go-arthas version
```

## 构建 Web Console

```bash
cd web
npm install
npm run build     # 产物输出到 web/dist/
npm run dev       # 开发模式，默认 http://localhost:3000
```

## 交叉编译与发布

一次性构建全部目标平台：

```bash
./scripts/release.sh [version]
```

产物输出到 `release/`，包含：

- `go-arthas-linux-amd64`、`go-arthas-linux-arm64`
- `go-arthas-darwin-amd64`、`go-arthas-darwin-arm64`
- `go-arthas-windows-amd64.exe`、`go-arthas-windows-arm64.exe`
- 每个文件的 `.sha256` 与汇总 `checksums.txt`
- `go-arthas-web-<version>.tar.gz`（打包后的 Web Console）

手动交叉编译单个平台（CLI 不依赖 CGO，可直接静态交叉编译）：

```bash
GOOS=linux GOARCH=amd64 go build -o go-arthas-linux-amd64 ./cmd/go-arthas
```

## 测试

```bash
make test         # 单元测试 + 属性测试
make test-race    # 竞态检测
make coverage     # 生成 coverage.html
```

## 常见问题

- **`undefined: NewCLI` 等错误**：用了 `go build ./cli/main.go` 单文件构建。改为 `go build ./cmd/go-arthas`。
- **Web 构建失败**：确认 Node.js / npm 版本，先执行 `npm install`。
