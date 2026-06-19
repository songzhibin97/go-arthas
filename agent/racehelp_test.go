package agent

import "testing"

// skipEnvSensitive 跳过那些对运行环境（机器负载、GC 时机、调度延迟、race
// instrumentation）敏感、作为默认门禁会 flaky 的测试：测量绝对内存开销、
// goroutine 计数、墙钟时序阈值的性能/资源基准。
//
// 这些断言验证的是性能特征而非功能正确性，在共享/高负载机器或 -race 下会因
// instrumentation 与负载偏移而系统性误报。功能正确性由不依赖绝对资源/时序阈值
// 的单元与 property 测试覆盖。需要运行这些基准时使用 `make test-perf`
// （非 -short、非 -race），并知晓其结果依赖运行环境。
func skipEnvSensitive(t *testing.T) {
	t.Helper()
	if raceEnabled {
		t.Skip("环境敏感的性能/资源基准在 -race 下不可靠，由 make test-perf 覆盖")
	}
	if testing.Short() {
		t.Skip("环境敏感的性能/资源基准在 -short 下跳过，由 make test-perf 覆盖")
	}
}
