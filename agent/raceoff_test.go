//go:build !race

package agent

// raceEnabled 在普通构建下为 false。详见 raceon_test.go。
const raceEnabled = false
