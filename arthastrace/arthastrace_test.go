package arthastrace

import (
	"strings"
	"sync"
	"testing"
)

// reset 清空全局状态以隔离测试
func reset() {
	registry = sync.Map{}
	globalSeq.Store(0)
}

func TestRegisterEnabledSetWatch(t *testing.T) {
	reset()
	if Enabled("x.F") {
		t.Error("unregistered id should be disabled")
	}
	Register("x.F")
	if Enabled("x.F") {
		t.Error("registered id defaults to disabled")
	}
	SetWatch("x.F", true)
	if !Enabled("x.F") {
		t.Error("should be enabled after SetWatch(true)")
	}
	SetWatch("x.F", false)
	if Enabled("x.F") {
		t.Error("should be disabled after SetWatch(false)")
	}
}

func TestEnterExitRecords(t *testing.T) {
	reset()
	SetWatch("x.F", true)
	Enter("x.F", []Arg{{Name: "a", Value: "1"}}).Exit([]Arg{{Name: "ret0", Value: "2"}}, nil)

	recs := Records("x.F")
	if len(recs) != 1 {
		t.Fatalf("records=%d want 1", len(recs))
	}
	r := recs[0]
	if r.ID != "x.F" || len(r.Args) != 1 || r.Args[0].Value != "1" {
		t.Errorf("unexpected record args: %+v", r)
	}
	if len(r.Results) != 1 || r.Results[0].Value != "2" {
		t.Errorf("unexpected results: %+v", r.Results)
	}
	if r.Duration < 0 {
		t.Error("duration should be >= 0")
	}
}

func TestExitNilSafe(t *testing.T) {
	var inv *Invocation
	inv.Exit(nil, nil) // watch 关闭路径：不应 panic
}

func TestExitPanic(t *testing.T) {
	reset()
	SetWatch("x.P", true)
	Enter("x.P", nil).Exit(nil, "boom")

	recs := Records("x.P")
	if len(recs) != 1 || recs[0].Panic != "boom" {
		t.Fatalf("expected panic record, got %+v", recs)
	}
	if recs[0].Stack == "" {
		t.Error("panic record should include stack")
	}
}

func TestRingBuffer(t *testing.T) {
	reset()
	SetWatch("x.R", true)
	total := defaultRingSize + 10
	for i := 0; i < total; i++ {
		Enter("x.R", nil).Exit(nil, nil)
	}
	recs := Records("x.R")
	if len(recs) != defaultRingSize {
		t.Fatalf("ring should hold %d, got %d", defaultRingSize, len(recs))
	}
	// 旧 → 新，seq 严格递增；且只保留最近 defaultRingSize 次
	for i := 1; i < len(recs); i++ {
		if recs[i].Seq <= recs[i-1].Seq {
			t.Errorf("records not in ascending seq order at %d", i)
		}
	}
	if recs[0].Seq != uint64(total-defaultRingSize+1) {
		t.Errorf("oldest retained seq=%d want %d", recs[0].Seq, total-defaultRingSize+1)
	}
}

func TestRecordsPartial(t *testing.T) {
	reset()
	SetWatch("x.Q", true)
	for i := 0; i < 3; i++ {
		Enter("x.Q", nil).Exit(nil, nil)
	}
	if recs := Records("x.Q"); len(recs) != 3 {
		t.Fatalf("want 3, got %d", len(recs))
	}
	if recs := Records("nonexistent"); recs != nil {
		t.Errorf("unknown id should return nil, got %v", recs)
	}
}

func TestMethods(t *testing.T) {
	reset()
	Register("a.A")
	Register("b.B")
	SetWatch("a.A", true)
	Enter("a.A", nil).Exit(nil, nil)

	ms := Methods()
	if len(ms) != 2 {
		t.Fatalf("methods=%d want 2", len(ms))
	}
	if ms[0].ID != "a.A" || !ms[0].Enabled || ms[0].Calls != 1 {
		t.Errorf("unexpected method[0]: %+v", ms[0])
	}
	if ms[1].ID != "b.B" || ms[1].Enabled || ms[1].Calls != 0 {
		t.Errorf("unexpected method[1]: %+v", ms[1])
	}
}

func TestFormat(t *testing.T) {
	if got := Format(42); got != "42" {
		t.Errorf("Format(42)=%q want 42", got)
	}
	long := strings.Repeat("x", maxValueLen+50)
	if got := Format(long); !strings.HasSuffix(got, "...(truncated)") {
		t.Errorf("long value should be truncated, got len=%d", len(got))
	}
}

type panicStringer struct{}

func (panicStringer) String() string { panic("nope") }

func TestFormatPanicSafe(t *testing.T) {
	// fmt 对 Stringer.String() 的 panic 有内置防护，返回 "%!v(PANIC=...)"；
	// Format 不应崩溃，且输出带 PANIC 标记（defer recover 作为额外兜底）。
	got := Format(panicStringer{})
	if got == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(got, "PANIC") {
		t.Errorf("expected guarded output containing PANIC, got %q", got)
	}
}

func TestConcurrent(t *testing.T) {
	reset()
	SetWatch("c.C", true)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Enter("c.C", []Arg{{Name: "x", Value: "1"}}).Exit(nil, nil)
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetWatch("c.C", true)
			_ = Enabled("c.C")
			_ = Methods()
			_ = Records("c.C")
		}()
	}
	wg.Wait()
}
