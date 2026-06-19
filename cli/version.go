package cli

import "fmt"

// 以下变量由构建时通过 -ldflags "-X" 注入，源码直接运行时使用默认值。
// 注入路径示例：
//
//	go build -ldflags "-X github.com/songzhibin97/go-arthas/cli.Version=v1.0.0"
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// printVersion 打印版本信息
func printVersion() {
	fmt.Printf("go-arthas %s\n", Version)
	fmt.Printf("  build time: %s\n", BuildTime)
	fmt.Printf("  git commit: %s\n", GitCommit)
}
