#!/bin/bash

# Go-Arthas 发布脚本
# 用于构建多平台二进制文件

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 版本号（从 git tag 获取，或使用默认值）
VERSION=${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}

# 构建信息
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 输出目录
OUTPUT_DIR="release"

# 支持的平台
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

echo -e "${GREEN}Go-Arthas 发布构建${NC}"
echo "版本: $VERSION"
echo "构建时间: $BUILD_TIME"
echo "Git 提交: $GIT_COMMIT"
echo ""

# 清理旧的构建产物
echo -e "${YELLOW}清理旧的构建产物...${NC}"
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# 构建 CLI 工具
echo -e "${YELLOW}构建 CLI 工具...${NC}"
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r -a array <<< "$platform"
    GOOS="${array[0]}"
    GOARCH="${array[1]}"
    
    output_name="go-arthas-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo "  构建 $GOOS/$GOARCH..."
    
    cd cli
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="-s -w -X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -X main.GitCommit=$GIT_COMMIT" \
        -o "../${OUTPUT_DIR}/${output_name}" \
        ./main.go
    cd ..
    
    # 计算校验和
    if [ "$GOOS" = "darwin" ] || [ "$GOOS" = "linux" ]; then
        shasum -a 256 "${OUTPUT_DIR}/${output_name}" > "${OUTPUT_DIR}/${output_name}.sha256"
    fi
    
    echo -e "  ${GREEN}✓${NC} $output_name"
done

# 构建 Web Console
echo ""
echo -e "${YELLOW}构建 Web Console...${NC}"
cd web
npm install
npm run build
cd ..

# 打包 Web Console
echo "  打包 Web Console..."
tar -czf "${OUTPUT_DIR}/go-arthas-web-${VERSION}.tar.gz" -C web/dist .
echo -e "  ${GREEN}✓${NC} go-arthas-web-${VERSION}.tar.gz"

# 生成校验和文件
echo ""
echo -e "${YELLOW}生成校验和文件...${NC}"
cd "$OUTPUT_DIR"
if command -v shasum &> /dev/null; then
    shasum -a 256 go-arthas-* > checksums.txt
elif command -v sha256sum &> /dev/null; then
    sha256sum go-arthas-* > checksums.txt
fi
cd ..

# 生成发布说明
echo ""
echo -e "${YELLOW}生成发布说明...${NC}"
cat > "${OUTPUT_DIR}/RELEASE_NOTES.md" << EOF
# Go-Arthas ${VERSION}

发布时间: ${BUILD_TIME}
Git 提交: ${GIT_COMMIT}

## 下载

### CLI 工具

- Linux (amd64): \`go-arthas-linux-amd64\`
- Linux (arm64): \`go-arthas-linux-arm64\`
- macOS (amd64): \`go-arthas-darwin-amd64\`
- macOS (arm64): \`go-arthas-darwin-arm64\`
- Windows (amd64): \`go-arthas-windows-amd64.exe\`

### Web Console

- \`go-arthas-web-${VERSION}.tar.gz\`

## 安装

### Linux/macOS

\`\`\`bash
# 下载二进制文件
wget https://github.com/your-org/go-arthas/releases/download/${VERSION}/go-arthas-linux-amd64

# 添加执行权限
chmod +x go-arthas-linux-amd64

# 移动到 PATH
sudo mv go-arthas-linux-amd64 /usr/local/bin/go-arthas

# 验证安装
go-arthas --version
\`\`\`

### Windows

1. 下载 \`go-arthas-windows-amd64.exe\`
2. 重命名为 \`go-arthas.exe\`
3. 添加到 PATH 环境变量

### Web Console

\`\`\`bash
# 解压
tar -xzf go-arthas-web-${VERSION}.tar.gz -C /var/www/html/go-arthas

# 使用 nginx 或其他 web 服务器提供服务
\`\`\`

## 校验和

所有文件的 SHA256 校验和在 \`checksums.txt\` 文件中。

验证下载文件：
\`\`\`bash
shasum -a 256 -c checksums.txt
\`\`\`

## 更新日志

请查看 [CHANGELOG.md](../CHANGELOG.md) 获取详细的更新日志。

## 文档

- [README](../README.md)
- [CLI 使用指南](../docs/CLI_USAGE.md)
- [Web Console 使用指南](../docs/WEB_CONSOLE.md)
- [示例应用程序](../examples/simple)

## 支持

- 问题反馈: https://github.com/your-org/go-arthas/issues
- 讨论: https://github.com/your-org/go-arthas/discussions
EOF

# 列出构建产物
echo ""
echo -e "${GREEN}构建完成！${NC}"
echo ""
echo "构建产物:"
ls -lh "$OUTPUT_DIR"

echo ""
echo -e "${GREEN}发布文件已准备就绪，位于 ${OUTPUT_DIR}/ 目录${NC}"
echo ""
echo "下一步:"
echo "  1. 测试构建的二进制文件"
echo "  2. 创建 git tag: git tag -a ${VERSION} -m 'Release ${VERSION}'"
echo "  3. 推送 tag: git push origin ${VERSION}"
echo "  4. 创建 GitHub Release 并上传文件"
