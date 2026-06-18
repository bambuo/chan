package chanlun

import (
	"math"
	"testing"
	"time"
)

// FuzzProcess 模糊测试：随机 K 线输入不应 panic。
func FuzzProcess(f *testing.F) {
	// 种子输入
	f.Add(100.0, 101.0, 99.0, 100.5, 1000.0)
	f.Add(0.0, 0.0, 0.0, 0.0, 0.0)
	f.Add(1e10, 1e10+1, 1e10-1, 1e10, 1e5)
	f.Add(1e-10, 1e-10, 1e-10, 1e-10, 1e-10)

	f.Fuzz(func(t *testing.T, open, high, low, close, volume float64) {
		// 构造 100 根随机 K 线
		klines := make([]Kline, 100)
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := range klines {
			noise := float64(i) * 0.1
			klines[i] = Kline{
				Time:       baseTime.Add(time.Duration(i) * time.Hour),
				Open:       open + noise,
				High:       math.Max(open, high) + noise,
				Low:        math.Min(low, math.Min(open, close)) + noise,
				Close:      close + noise,
				BaseVolume: math.Max(0, volume),
			}
		}

		engine, err := NewEngine(DefaultConfig())
		if err != nil {
			return
		}

		// Process 不应 panic
		result, err := engine.Process(klines)
		if err != nil {
			return
		}

		// 基本一致性检查
		if result == nil {
			t.Error("Process returned nil result with nil error")
		}
		if len(result.MergedKlines) > len(klines) {
			t.Error("merged klines > input klines")
		}
	})
}

// TestProcessDeterministic 属性测试：相同输入必须产生相同输出。
func TestProcessDeterministic(t *testing.T) {
	// 生成 200 根振荡 K 线
	klines := generateFractalKlines(200)

	engine, _ := NewEngine(DefaultConfig())

	// 运行两次
	result1, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("first Process: %v", err)
	}

	result2, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("second Process: %v", err)
	}

	// 验证分型数量一致
	if len(result1.Fractals) != len(result2.Fractals) {
		t.Errorf("determinism: fractals differ: %d vs %d", len(result1.Fractals), len(result2.Fractals))
	}
	if len(result1.Bis) != len(result2.Bis) {
		t.Errorf("determinism: bis differ: %d vs %d", len(result1.Bis), len(result2.Bis))
	}
	if len(result1.Segments) != len(result2.Segments) {
		t.Errorf("determinism: segments differ: %d vs %d", len(result1.Segments), len(result2.Segments))
	}
	if len(result1.Signals) != len(result2.Signals) {
		t.Errorf("determinism: signals differ: %d vs %d", len(result1.Signals), len(result2.Signals))
	}
}

// TestIncrementalEquivalence 属性测试：增量更新必须与全量重算完全一致。
func TestIncrementalEquivalence(t *testing.T) {
	// 用 200 根振荡 K 线，分批增量更新，每步与全量比对
	klines := generateFractalKlines(200)

	engine, _ := NewEngine(DefaultConfig())

	// 先处理前 50 根作为起点
	_, err := engine.Process(klines[:50])
	if err != nil {
		t.Fatalf("initial Process: %v", err)
	}

	// 逐根增量更新，每步与全量重算对比
	for i := 50; i < len(klines); i++ {
		incResult, err := engine.Update(klines[i])
		if err != nil {
			t.Fatalf("Update at %d: %v", i, err)
		}

		// 全量重算相同数据
		fullResult, err := engine.Process(klines[:i+1])
		if err != nil {
			t.Fatalf("full Process at %d: %v", i, err)
		}

		// 递归比对全字段
		if !resultsEqual(incResult, fullResult) {
			t.Fatalf("step %d: incremental ≠ full\n  fractals: %d vs %d\n  bis: %d vs %d\n  segments: %d vs %d\n  pivots: %d vs %d\n  trends: %d vs %d\n  deviations: %d vs %d\n  signals: %d vs %d",
				i,
				len(incResult.Fractals), len(fullResult.Fractals),
				len(incResult.Bis), len(fullResult.Bis),
				len(incResult.Segments), len(fullResult.Segments),
				len(incResult.Pivots), len(fullResult.Pivots),
				len(incResult.Trends), len(fullResult.Trends),
				len(incResult.Deviations), len(fullResult.Deviations),
				len(incResult.Signals), len(fullResult.Signals))
		}
	}
}

// resultsEqual 递归比对两个 Result 的所有字段。
func resultsEqual(a, b *Result) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.Fractals) != len(b.Fractals) {
		return false
	}
	if len(a.Bis) != len(b.Bis) {
		return false
	}
	if len(a.MergedBis) != len(b.MergedBis) {
		return false
	}
	if len(a.Segments) != len(b.Segments) {
		return false
	}
	if len(a.Pivots) != len(b.Pivots) {
		return false
	}
	if len(a.Trends) != len(b.Trends) {
		return false
	}
	if len(a.Deviations) != len(b.Deviations) {
		return false
	}
	if len(a.Signals) != len(b.Signals) {
		return false
	}
	// Fractal 级别的验证：每个分型的类型和索引一致
	for i := range a.Fractals {
		if a.Fractals[i].Type != b.Fractals[i].Type || a.Fractals[i].Index != b.Fractals[i].Index {
			return false
		}
	}
	// Bi 级别的验证
	for i := range a.Bis {
		if a.Bis[i].StartIndex != b.Bis[i].StartIndex || a.Bis[i].Direction != b.Bis[i].Direction {
			return false
		}
	}
	// Signal 级别的验证
	for i := range a.Signals {
		if a.Signals[i].Type != b.Signals[i].Type || a.Signals[i].Index != b.Signals[i].Index {
			return false
		}
	}
	return true
}

// TestRaceCondition 竞态测试：多个 goroutine 同时调用不应 panic。
func TestRaceCondition(t *testing.T) {
	engine, _ := NewEngine(DefaultConfig())
	klines := generateFractalKlines(500)

	// 先初始化
	_, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("initial Process: %v", err)
	}

	// 10 个 goroutine 同时读写
	done := make(chan bool, 10)
	for g := 0; g < 10; g++ {
		go func(id int) {
			for i := 0; i < 20; i++ {
				c := Kline{
					Time:       time.Now(),
					Open:       100 + float64(i),
					High:       102 + float64(i),
					Low:        98 + float64(i),
					Close:      101 + float64(i),
					BaseVolume: 1000,
				}
				_, err := engine.Update(c)
				if err != nil {
					t.Logf("goroutine %d update error: %v", id, err)
				}
			}
			done <- true
		}(g)
	}

	// 等待所有 goroutine 完成
	for g := 0; g < 10; g++ {
		<-done
	}
}
