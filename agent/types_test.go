package agent

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// TestMetrics_MarshalJSON 测试 Metrics 结构体序列化为 JSON
func TestMetrics_MarshalJSON(t *testing.T) {
	now := time.Now()
	metrics := &Metrics{
		Timestamp:  now,
		Goroutines: 10,
		Memory: MemoryMetrics{
			HeapAlloc:    1024,
			HeapInuse:    2048,
			HeapIdle:     512,
			HeapReleased: 256,
			StackInuse:   128,
			TotalAlloc:   4096,
			Sys:          8192,
		},
		CPU: CPUMetrics{
			UsagePercent: 25.5,
		},
		GC: GCMetrics{
			NumGC:      5,
			PauseTotal: 100 * time.Millisecond,
			LastPause:  20 * time.Millisecond,
			PauseAvg:   20 * time.Millisecond,
		},
	}

	// 序列化为 JSON
	data, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("Failed to marshal Metrics: %v", err)
	}

	// 验证 JSON 不为空
	if len(data) == 0 {
		t.Fatal("Marshaled JSON is empty")
	}

	// 验证 JSON 包含预期字段
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// 验证关键字段存在
	if _, ok := result["timestamp"]; !ok {
		t.Error("JSON missing 'timestamp' field")
	}
	if _, ok := result["goroutines"]; !ok {
		t.Error("JSON missing 'goroutines' field")
	}
	if _, ok := result["memory"]; !ok {
		t.Error("JSON missing 'memory' field")
	}
	if _, ok := result["cpu"]; !ok {
		t.Error("JSON missing 'cpu' field")
	}
	if _, ok := result["gc"]; !ok {
		t.Error("JSON missing 'gc' field")
	}
}

// TestMetrics_UnmarshalJSON 测试从 JSON 反序列化 Metrics 结构体
func TestMetrics_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"timestamp": "2024-01-15T10:30:00Z",
		"goroutines": 15,
		"memory": {
			"heap_alloc": 2048,
			"heap_inuse": 4096,
			"heap_idle": 1024,
			"heap_released": 512,
			"stack_inuse": 256,
			"total_alloc": 8192,
			"sys": 16384
		},
		"cpu": {
			"usage_percent": 30.5
		},
		"gc": {
			"num_gc": 10,
			"pause_total": 200000000,
			"last_pause": 40000000,
			"pause_avg": 20000000
		}
	}`

	var metrics Metrics
	if err := json.Unmarshal([]byte(jsonData), &metrics); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// 验证字段值
	if metrics.Goroutines != 15 {
		t.Errorf("Expected goroutines=15, got %d", metrics.Goroutines)
	}
	if metrics.Memory.HeapAlloc != 2048 {
		t.Errorf("Expected HeapAlloc=2048, got %d", metrics.Memory.HeapAlloc)
	}
	if metrics.CPU.UsagePercent != 30.5 {
		t.Errorf("Expected UsagePercent=30.5, got %f", metrics.CPU.UsagePercent)
	}
	if metrics.GC.NumGC != 10 {
		t.Errorf("Expected NumGC=10, got %d", metrics.GC.NumGC)
	}
}

// TestMetrics_RoundTrip 测试 JSON 序列化和反序列化的往返
func TestMetrics_RoundTrip(t *testing.T) {
	original := &Metrics{
		Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Goroutines: 20,
		Memory: MemoryMetrics{
			HeapAlloc:    3072,
			HeapInuse:    6144,
			HeapIdle:     1536,
			HeapReleased: 768,
			StackInuse:   384,
			TotalAlloc:   12288,
			Sys:          24576,
		},
		CPU: CPUMetrics{
			UsagePercent: 45.8,
		},
		GC: GCMetrics{
			NumGC:      15,
			PauseTotal: 300 * time.Millisecond,
			LastPause:  60 * time.Millisecond,
			PauseAvg:   20 * time.Millisecond,
		},
	}

	// 序列化
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// 反序列化
	var decoded Metrics
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// 验证关键字段匹配
	if decoded.Goroutines != original.Goroutines {
		t.Errorf("Goroutines mismatch: got %d, want %d", decoded.Goroutines, original.Goroutines)
	}
	if decoded.Memory.HeapAlloc != original.Memory.HeapAlloc {
		t.Errorf("HeapAlloc mismatch: got %d, want %d", decoded.Memory.HeapAlloc, original.Memory.HeapAlloc)
	}
	if decoded.CPU.UsagePercent != original.CPU.UsagePercent {
		t.Errorf("UsagePercent mismatch: got %f, want %f", decoded.CPU.UsagePercent, original.CPU.UsagePercent)
	}
}

// TestSafeMetrics_ConcurrentGetSet 测试 safeMetrics 的并发 Get/Set 操作
func TestSafeMetrics_ConcurrentGetSet(t *testing.T) {
	sm := &safeMetrics{}

	// 初始化一个指标
	initialMetrics := &Metrics{
		Timestamp:  time.Now(),
		Goroutines: 5,
	}
	sm.Set(initialMetrics)

	// 并发读写测试
	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// 启动多个 goroutine 进行写操作
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				metrics := &Metrics{
					Timestamp:  time.Now(),
					Goroutines: id*numOperations + j,
				}
				sm.Set(metrics)
			}
		}(i)
	}

	// 启动多个 goroutine 进行读操作
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				m := sm.Get()
				if m == nil {
					t.Error("Get returned nil")
				}
			}
		}()
	}

	wg.Wait()

	// 验证最终状态
	final := sm.Get()
	if final == nil {
		t.Fatal("Final Get returned nil")
	}
}

// TestSafeMetrics_GetReturnsNilWhenEmpty 测试空 safeMetrics 返回 nil
func TestSafeMetrics_GetReturnsNilWhenEmpty(t *testing.T) {
	sm := &safeMetrics{}

	result := sm.Get()
	if result != nil {
		t.Errorf("Expected nil for empty safeMetrics, got %v", result)
	}
}

// TestSafeMetrics_GetReturnsCopy 测试 Get 返回的是副本而非原始指针
func TestSafeMetrics_GetReturnsCopy(t *testing.T) {
	sm := &safeMetrics{}

	original := &Metrics{
		Timestamp:  time.Now(),
		Goroutines: 10,
		Memory: MemoryMetrics{
			HeapAlloc: 1024,
		},
	}
	sm.Set(original)

	// 获取副本
	copy1 := sm.Get()
	copy2 := sm.Get()

	// 验证返回的是不同的指针
	if copy1 == copy2 {
		t.Error("Get should return different pointers (deep copies)")
	}

	// 修改副本不应影响存储的值
	copy1.Goroutines = 999
	copy1.Memory.HeapAlloc = 9999

	// 再次获取，验证原始值未被修改
	copy3 := sm.Get()
	if copy3.Goroutines == 999 {
		t.Error("Modifying returned copy affected stored value")
	}
	if copy3.Memory.HeapAlloc == 9999 {
		t.Error("Modifying returned copy affected stored value")
	}
}

// TestMetrics_Clone 测试 Metrics 的 Clone 方法
func TestMetrics_Clone(t *testing.T) {
	original := &Metrics{
		Timestamp:  time.Now(),
		Goroutines: 25,
		Memory: MemoryMetrics{
			HeapAlloc: 2048,
			HeapInuse: 4096,
		},
		CPU: CPUMetrics{
			UsagePercent: 50.0,
		},
		GC: GCMetrics{
			NumGC:     20,
			LastPause: 30 * time.Millisecond,
		},
	}

	cloned := original.Clone()

	// 验证克隆不为 nil
	if cloned == nil {
		t.Fatal("Clone returned nil")
	}

	// 验证是不同的指针
	if original == cloned {
		t.Error("Clone should return a different pointer")
	}

	// 验证字段值相同
	if cloned.Goroutines != original.Goroutines {
		t.Errorf("Goroutines mismatch: got %d, want %d", cloned.Goroutines, original.Goroutines)
	}
	if cloned.Memory.HeapAlloc != original.Memory.HeapAlloc {
		t.Errorf("HeapAlloc mismatch: got %d, want %d", cloned.Memory.HeapAlloc, original.Memory.HeapAlloc)
	}
	if cloned.CPU.UsagePercent != original.CPU.UsagePercent {
		t.Errorf("UsagePercent mismatch: got %f, want %f", cloned.CPU.UsagePercent, original.CPU.UsagePercent)
	}

	// 修改克隆不应影响原始值
	cloned.Goroutines = 999
	if original.Goroutines == 999 {
		t.Error("Modifying clone affected original")
	}
}

// TestMetrics_CloneNil 测试 Clone nil 指针
func TestMetrics_CloneNil(t *testing.T) {
	var m *Metrics
	cloned := m.Clone()
	if cloned != nil {
		t.Errorf("Clone of nil should return nil, got %v", cloned)
	}
}
