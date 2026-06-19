package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/songzhibin97/go-arthas/arthastrace"
)

// traceTestSeq 为每次运行生成唯一方法 id。arthastrace 注册表是全局/包级的,跨 -count
// 迭代会累积调用计数与 tt 记录;固定 id 在重复运行时会因累积而 Calls!=1 / 多条 record 失败
// (count=1 的常规 CI 不受影响,但重复跑/压测会暴露)。唯一 id 让每次运行用独立注册表项。
var traceTestSeq atomic.Uint64

func TestTraceMethodsControlPlane(t *testing.T) {
	port, err := startOnFreePort(Config{EnableMetrics: false, LogLevel: "error"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer Stop()

	id := fmt.Sprintf("agenttest.Foo.%d", traceTestSeq.Add(1))
	arthastrace.Register(id)

	client := &http.Client{Timeout: 3 * time.Second}
	base := fmt.Sprintf("http://127.0.0.1:%d/api/v1/trace/methods", port)

	// 开启 watch
	resp, err := client.Post(base+"/watch?id="+id+"&on=true", "", nil)
	if err != nil {
		t.Fatalf("watch on: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("watch on status=%d", resp.StatusCode)
	}
	if !arthastrace.Enabled(id) {
		t.Fatal("watch should be enabled after POST on=true")
	}

	// 模拟一次被织入调用
	arthastrace.Enter(id, []arthastrace.Arg{{Name: "x", Value: "1"}}).
		Exit([]arthastrace.Arg{{Name: "ret0", Value: "2"}}, nil)

	// methods 列表应包含该 id 且状态正确
	resp2, err := client.Get(base)
	if err != nil {
		t.Fatalf("methods: %v", err)
	}
	defer resp2.Body.Close()
	var methods []arthastrace.MethodInfo
	if err := json.NewDecoder(resp2.Body).Decode(&methods); err != nil {
		t.Fatalf("decode methods: %v", err)
	}
	found := false
	for _, m := range methods {
		if m.ID == id {
			found = true
			if !m.Enabled || m.Calls != 1 {
				t.Errorf("method state mismatch: %+v", m)
			}
		}
	}
	if !found {
		t.Errorf("id %s not present in methods list", id)
	}

	// records 应返回该次调用
	resp3, err := client.Get(base + "/records?id=" + id)
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	defer resp3.Body.Close()
	var recs []arthastrace.Record
	if err := json.NewDecoder(resp3.Body).Decode(&recs); err != nil {
		t.Fatalf("decode records: %v", err)
	}
	if len(recs) != 1 || len(recs[0].Args) != 1 || recs[0].Args[0].Value != "1" {
		t.Errorf("records mismatch: %+v", recs)
	}

	// 关闭 watch
	resp4, err := client.Post(base+"/watch?id="+id+"&on=false", "", nil)
	if err != nil {
		t.Fatalf("watch off: %v", err)
	}
	resp4.Body.Close()
	if arthastrace.Enabled(id) {
		t.Error("watch should be disabled after on=false")
	}

	// 缺少 id → 400
	resp5, err := client.Get(base + "/records")
	if err == nil {
		resp5.Body.Close()
		if resp5.StatusCode != http.StatusBadRequest {
			t.Errorf("missing id should return 400, got %d", resp5.StatusCode)
		}
	}
}
