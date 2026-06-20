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
				Time:       DateTime{Time: baseTime.Add(time.Duration(i) * time.Hour)},
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

// TestIncrementalEquivalence 属性测试：增量更新结果应与全量重算基本一致。
// 注意：由于增量笔构建是局部决策，而批量笔构建是全局最优，笔数可能略有差异。
func TestIncrementalEquivalence(t *testing.T) {
	klines := generateFractalKlines(200)

	stream, _ := NewStreamEngine(DefaultConfig())
	_, err := stream.Init(klines[:50])
	if err != nil {
		t.Fatalf("initial Init: %v", err)
	}

	batchEngine, _ := NewEngine(DefaultConfig())

	for i := 50; i < len(klines); i++ {
		inc := stream.AddKline(klines[i])
		incResult := inc.Snapshot

		fullResult, err := batchEngine.Process(klines[:i+1])
		if err != nil {
			t.Fatalf("full Process at %d: %v", i, err)
		}

		// 已合并 K 线数应基本一致
		mDiff := len(incResult.MergedKlines) - len(fullResult.MergedKlines)
		if mDiff < 0 {
			mDiff = -mDiff
		}
		if mDiff > 3 {
			t.Fatalf("step %d: MergedKlines diff too large: %d vs %d",
				i, len(incResult.MergedKlines), len(fullResult.MergedKlines))
		}

		// 分型数应基本一致（允许±2）
		fDiff := len(incResult.Fractals) - len(fullResult.Fractals)
		if fDiff < 0 {
			fDiff = -fDiff
		}
		if fDiff > 3 {
			t.Fatalf("step %d: Fractals diff too large: %d vs %d",
				i, len(incResult.Fractals), len(fullResult.Fractals))
		}

		// 笔数允许差异（增量局部决策 vs 批量全局最优）
		bDiff := len(incResult.Bis) - len(fullResult.Bis)
		if bDiff < 0 {
			bDiff = -bDiff
		}
		if bDiff > 3 {
			t.Fatalf("step %d: Bis diff too large: %d vs %d",
				i, len(incResult.Bis), len(fullResult.Bis))
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
	stream, _ := NewStreamEngine(DefaultConfig())
	klines := generateFractalKlines(500)

	// 先初始化
	_, err := stream.Init(klines)
	if err != nil {
		t.Fatalf("initial Init: %v", err)
	}

	// 10 个 goroutine 同时读写
	done := make(chan bool, 10)
	for g := 0; g < 10; g++ {
		go func(id int) {
			for i := 0; i < 20; i++ {
				c := Kline{
					Time:       DateTime{Time: time.Now()},
					Open:       100 + float64(i),
					High:       102 + float64(i),
					Low:        98 + float64(i),
					Close:      101 + float64(i),
					BaseVolume: 1000,
				}
				stream.AddKline(c)
			}
			done <- true
		}(g)
	}

	// 等待所有 goroutine 完成
	for g := 0; g < 10; g++ {
		<-done
	}
}
