package agent

import (
	"os"
	"testing"
)

// skipEnvSensitive 跳过那些对运行环境（机器负载、GC 时机、调度延迟、race
// instrumentation）敏感、作为默认门禁会 flaky 的测试：测量绝对内存开销、
// goroutine 计数、墙钟时序阈值的性能/资源基准。
//
// 这些断言验证的是性能特征而非功能正确性,在共享/高负载机器上会系统性误报。
// 因此它们**默认跳过**(包括裸 `go test ./...`、`make test`、`make test-race`、
// `make coverage`),只有显式设置 ARTHAS_PERF_TESTS=1 时才运行,由 `make test-perf`
// 设置该变量。功能正确性由不依赖绝对资源/时序阈值的单元与 property 测试覆盖。
func skipEnvSensitive(t *testing.T) {
	t.Helper()
	if os.Getenv("ARTHAS_PERF_TESTS") != "1" {
		t.Skip("环境敏感的性能/资源基准默认跳过；设 ARTHAS_PERF_TESTS=1（或 make test-perf）运行")
	}
}
