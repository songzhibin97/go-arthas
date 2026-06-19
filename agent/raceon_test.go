//go:build race

package agent

// raceEnabled 在 -race 构建下为 true，供测试跳过那些在 race instrumentation
// 下不可靠的性能/内存断言（race 会显著改变内存与 CPU 特征）。
const raceEnabled = true
