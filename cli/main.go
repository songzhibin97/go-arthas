package cli

import (
	"flag"
	"fmt"
	"os"
)

// Run 运行 CLI 应用
func Run(args []string) int {
	if len(args) < 1 {
		printUsage()
		return 1
	}

	command := args[0]

	switch command {
	case "connect":
		return runConnect(args[1:])
	case "metrics":
		return runMetrics(args[1:])
	case "info":
		return runInfo(args[1:])
	case "profile":
		return runProfile(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return 0
	case "version", "-v", "--version":
		printVersion()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		return 1
	}
}

// printUsage 打印使用说明
func printUsage() {
	fmt.Println("Go-Arthas CLI - Runtime monitoring and performance analysis tool for Go")
	fmt.Println()
	fmt.Println("Note: Go-Arthas focuses on runtime metrics and profiling.")
	fmt.Println("      For method-level tracing, consider OpenTelemetry or manual instrumentation.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go-arthas <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  connect <host:port>           Connect to agent and verify connection")
	fmt.Println("  metrics [--host <host:port>]  Display current runtime metrics")
	fmt.Println("  info [--host <host:port>]     Display system information")
	fmt.Println("  profile <type> [options]      Capture performance profile")
	fmt.Println("  version                       Show version information")
	fmt.Println()
	fmt.Println("Profile types:")
	fmt.Println("  cpu       CPU profile (requires --duration)")
	fmt.Println("  heap      Memory heap profile")
	fmt.Println("  goroutine Goroutine profile")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --host <host:port>   Agent address (default: localhost:8563)")
	fmt.Println("  --duration <seconds> Profile duration for CPU profile (default: 30)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go-arthas connect localhost:8563")
	fmt.Println("  go-arthas metrics --host localhost:8563")
	fmt.Println("  go-arthas info")
	fmt.Println("  go-arthas profile cpu --duration 30")
	fmt.Println("  go-arthas profile heap")
	fmt.Println()
	fmt.Println("For more information: https://github.com/songzhibin97/go-arthas")
}

// runConnect 执行 connect 命令
func runConnect(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: connect command requires host:port argument")
		fmt.Fprintln(os.Stderr, "Usage: go-arthas connect <host:port>")
		return 1
	}

	host := args[0]
	cli := NewCLI(host)

	fmt.Printf("Connecting to %s...\n", host)
	if err := cli.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Successfully connected to %s\n", host)
	return 0
}

// runMetrics 执行 metrics 命令
func runMetrics(args []string) int {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)
	host := fs.String("host", "localhost:8563", "Agent address")
	fs.Parse(args)

	cli := NewCLI(*host)

	metrics, err := cli.GetMetrics()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	FormatMetrics(metrics)
	return 0
}

// runInfo 执行 info 命令
func runInfo(args []string) int {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	host := fs.String("host", "localhost:8563", "Agent address")
	fs.Parse(args)

	cli := NewCLI(*host)

	info, err := cli.GetInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	FormatSystemInfo(info)
	return 0
}

// runProfile 执行 profile 命令
func runProfile(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: profile command requires profile type")
		fmt.Fprintln(os.Stderr, "Usage: go-arthas profile <cpu|heap|goroutine> [options]")
		return 1
	}

	profileType := args[0]
	fs := flag.NewFlagSet("profile", flag.ExitOnError)
	host := fs.String("host", "localhost:8563", "Agent address")
	duration := fs.Int("duration", 30, "Profile duration in seconds (for CPU profile)")
	fs.Parse(args[1:])

	// 验证 profile 类型
	validTypes := map[string]bool{
		"cpu":       true,
		"heap":      true,
		"goroutine": true,
	}

	if !validTypes[profileType] {
		fmt.Fprintf(os.Stderr, "Error: invalid profile type '%s'\n", profileType)
		fmt.Fprintln(os.Stderr, "Valid types: cpu, heap, goroutine")
		return 1
	}

	// CPU profile 需要 duration
	if profileType == "cpu" && *duration <= 0 {
		fmt.Fprintln(os.Stderr, "Error: CPU profile requires positive duration")
		return 1
	}

	cli := NewCLI(*host)

	fmt.Printf("Capturing %s profile", profileType)
	if profileType == "cpu" {
		fmt.Printf(" for %d seconds", *duration)
	}
	fmt.Println("...")

	data, err := cli.GetProfile(profileType, *duration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	filename, err := cli.SaveProfile(profileType, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Profile saved to %s (%s)\n", filename, FormatBytesSize(uint64(len(data))))
	fmt.Printf("Analyze with: go tool pprof %s\n", filename)
	return 0
}
