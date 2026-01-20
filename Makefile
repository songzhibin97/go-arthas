.PHONY: all build build-cli build-web test test-unit test-property test-race coverage clean install help

# 默认目标
all: build

# 构建所有组件
build: build-cli build-web
	@echo "✓ 所有组件构建完成"

# 构建 CLI 工具
build-cli:
	@echo "构建 CLI 工具..."
	@cd cli && go build -o ../bin/go-arthas -ldflags="-s -w" ./main.go
	@echo "✓ CLI 工具构建完成: bin/go-arthas"

# 构建 Web Console
build-web:
	@echo "构建 Web Console..."
	@cd web && npm install && npm run build
	@echo "✓ Web Console 构建完成: web/dist/"

# 运行所有测试
test: test-unit test-property
	@echo "✓ 所有测试通过"

# 运行单元测试
test-unit:
	@echo "运行单元测试..."
	@go test -v ./...

# 运行属性测试
test-property:
	@echo "运行属性测试..."
	@go test -v -run TestProperty ./...

# 运行竞态检测
test-race:
	@echo "运行竞态检测..."
	@go test -race ./...

# 生成代码覆盖率报告
coverage:
	@echo "生成代码覆盖率报告..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ 覆盖率报告生成: coverage.html"

# 清理构建产物
clean:
	@echo "清理构建产物..."
	@rm -rf bin/
	@rm -rf web/dist/
	@rm -f coverage.out coverage.html
	@rm -f *.prof
	@echo "✓ 清理完成"

# 安装 CLI 工具到系统
install: build-cli
	@echo "安装 CLI 工具..."
	@cp bin/go-arthas $(GOPATH)/bin/go-arthas
	@echo "✓ CLI 工具已安装到 $(GOPATH)/bin/go-arthas"

# 运行示例应用程序
run-example:
	@echo "运行示例应用程序..."
	@cd examples/simple && go run main.go

# 启动 Web Console 开发服务器
dev-web:
	@echo "启动 Web Console 开发服务器..."
	@cd web && npm run dev

# 格式化代码
fmt:
	@echo "格式化代码..."
	@go fmt ./...
	@echo "✓ 代码格式化完成"

# 运行 linter
lint:
	@echo "运行 linter..."
	@golangci-lint run ./...
	@echo "✓ Linter 检查完成"

# 运行基准测试
bench:
	@echo "运行基准测试..."
	@go test -bench=. -benchmem ./...

# 显示帮助信息
help:
	@echo "Go-Arthas Makefile"
	@echo ""
	@echo "可用目标:"
	@echo "  make build         - 构建所有组件（CLI + Web Console）"
	@echo "  make build-cli     - 仅构建 CLI 工具"
	@echo "  make build-web     - 仅构建 Web Console"
	@echo "  make test          - 运行所有测试"
	@echo "  make test-unit     - 运行单元测试"
	@echo "  make test-property - 运行属性测试"
	@echo "  make test-race     - 运行竞态检测"
	@echo "  make coverage      - 生成代码覆盖率报告"
	@echo "  make clean         - 清理构建产物"
	@echo "  make install       - 安装 CLI 工具到系统"
	@echo "  make run-example   - 运行示例应用程序"
	@echo "  make dev-web       - 启动 Web Console 开发服务器"
	@echo "  make fmt           - 格式化代码"
	@echo "  make lint          - 运行 linter"
	@echo "  make bench         - 运行基准测试"
	@echo "  make help          - 显示此帮助信息"
