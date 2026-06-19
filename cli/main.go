package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	case "thread":
		return runThread(args[1:])
	case "flight":
		return runFlight(args[1:])
	case "methods":
		return runMethods(args[1:])
	case "watch":
		return runWatch(args[1:])
	case "build":
		return runBuild(args[1:])
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
	fmt.Println("  thread [options]              Dump goroutines (state summary + suspected blocks)")
	fmt.Println("  flight <start|snapshot|stop>  Execution trace flight recorder (Go 1.25+)")
	fmt.Println("  methods [--host]              List compile-time watched methods")
	fmt.Println("  watch <id> [--off|--records]  Toggle or inspect a watched method")
	fmt.Println("  build --targets <ids> [...]   Build with compile-time watch instrumentation")
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

// runThread 执行 thread 命令（goroutine 诊断）
func runThread(args []string) int {
	fs := flag.NewFlagSet("thread", flag.ExitOnError)
	host := fs.String("host", "localhost:8563", "Agent address")
	full := fs.Bool("full", false, "Print raw full stack trace (all goroutines)")
	stacks := fs.Bool("stacks", false, "Include per-goroutine stacks in structured output")
	minWait := fs.Int("min-wait", 1, "Flag goroutines blocked >= N minutes as suspected")
	fs.Parse(args)

	cli := NewCLI(*host)

	if *full {
		text, err := cli.GetGoroutinesText()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Print(text)
		return 0
	}

	dump, err := cli.GetGoroutines(*stacks, *minWait)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	FormatGoroutineDump(dump, *stacks)
	return 0
}

// runFlight 执行 flight 命令（执行轨迹飞行记录器）
func runFlight(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: flight command requires an action (start|snapshot|stop)")
		fmt.Fprintln(os.Stderr, "Usage: go-arthas flight <start|snapshot|stop> [--host <host:port>]")
		return 1
	}

	action := args[0]
	fs := flag.NewFlagSet("flight", flag.ExitOnError)
	host := fs.String("host", "localhost:8563", "Agent address")
	fs.Parse(args[1:])

	cli := NewCLI(*host)

	switch action {
	case "start":
		if err := cli.FlightStart(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Println("Flight recorder started.")
		return 0
	case "stop":
		if err := cli.FlightStop(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Println("Flight recorder stopped.")
		return 0
	case "snapshot":
		data, err := cli.FlightSnapshot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		filename, err := cli.SaveTrace(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("Trace saved to %s (%s)\n", filename, FormatBytesSize(uint64(len(data))))
		fmt.Printf("Analyze with: go tool trace %s\n", filename)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown flight action '%s' (use start|snapshot|stop)\n", action)
		return 1
	}
}

// runMethods 执行 methods 命令：列出可观察方法
func runMethods(args []string) int {
	fs := flag.NewFlagSet("methods", flag.ExitOnError)
	host := fs.String("host", "localhost:8563", "Agent address")
	fs.Parse(args)

	cli := NewCLI(*host)
	methods, err := cli.GetMethods()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	FormatMethods(methods)
	return 0
}

// runWatch 执行 watch 命令：开关或查看某方法
func runWatch(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: watch requires a method id")
		fmt.Fprintln(os.Stderr, "Usage: go-arthas watch <id> [--off] [--records] [--host <host:port>]")
		return 1
	}
	id := args[0]
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	host := fs.String("host", "localhost:8563", "Agent address")
	off := fs.Bool("off", false, "Disable watch instead of enabling")
	records := fs.Bool("records", false, "Show recorded invocations (time tunnel)")
	fs.Parse(args[1:])

	cli := NewCLI(*host)

	if *records {
		recs, err := cli.GetRecords(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		FormatRecords(id, recs)
		return 0
	}

	on := !*off
	if err := cli.SetWatch(id, on); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Printf("watch %s -> %v\n", id, on)
	return 0
}

// runBuild 执行 build 命令：以编译期 watch 织入构建目标程序
func runBuild(args []string) int {
	// 仅提取 --targets，其余参数原样透传给 `go build`（如 -o、./...、-tags 等）
	var targetList string
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--targets" || a == "-targets":
			if i+1 < len(args) {
				targetList = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--targets="):
			targetList = strings.TrimPrefix(a, "--targets=")
		case strings.HasPrefix(a, "-targets="):
			targetList = strings.TrimPrefix(a, "-targets=")
		default:
			rest = append(rest, a)
		}
	}

	if targetList == "" {
		fmt.Fprintln(os.Stderr, "Error: build requires --targets \"pkg.Func,...\"")
		return 1
	}

	tmpDir, err := os.MkdirTemp("", "arthas-build-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	// 1. 构建 toolexec（需当前 module require go-arthas）
	toolexec := filepath.Join(tmpDir, "arthas-toolexec")
	if out, err := exec.Command("go", "build", "-o", toolexec, "github.com/songzhibin97/go-arthas/cmd/arthas-toolexec").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: build toolexec failed: %v\n%s", err, out)
		return 1
	}

	// 2. 预编译 arthastrace 获取其归档路径（供 toolexec 注入 importcfg）
	archiveOut, err := exec.Command("go", "list", "-export", "-f", "{{.Export}}", "github.com/songzhibin97/go-arthas/arthastrace").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: locate arthastrace archive failed: %v\n", err)
		return 1
	}
	archive := strings.TrimSpace(string(archiveOut))
	if archive == "" {
		fmt.Fprintln(os.Stderr, "Error: empty arthastrace archive path")
		return 1
	}

	// 3. 带 toolexec 构建
	buildArgs := append([]string{"build", "-toolexec", toolexec}, rest...)
	cmd := exec.Command("go", buildArgs...)
	cmd.Env = append(os.Environ(), "ARTHAS_TARGETS="+targetList, "ARTHAS_ARCHIVE="+archive)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: instrumented build failed: %v\n", err)
		return 1
	}

	fmt.Printf("Instrumented build complete. Watched: %s\n", targetList)
	fmt.Println("Reminder: the target binary must import the arthastrace package")
	fmt.Println("(pulled in automatically when you import the go-arthas agent).")
	return 0
}
