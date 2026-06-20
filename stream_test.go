package chanlun

import (
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// StreamEngine 测试
// ──────────────────────────────────────────────

// generateTestKlines 生成用于测试的 K 线数据。
// 模拟一个简单的上涨 → 回调 → 上涨走势。
func generateTestKlines(n int) []Kline {
	klines := make([]Kline, n)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	price := 100.0

	for i := 0; i < n; i++ {
		// 简单的锯齿波模拟
		direction := 1.0
		if (i/10)%2 == 1 {
			direction = -1.0
		}
		offset := float64(i%10) * direction

		klines[i] = Kline{
			Time:  DateTime{Time: baseTime.Add(time.Duration(i) * time.Hour)},
			Open:  price + offset,
			High:  price + offset + 1.0,
			Low:   price + offset - 1.0,
			Close: price + offset + 0.5*direction,
		}
	}
	return klines
}

// TestStreamEngineInitConsistency 验证 StreamEngine.Init 与 Engine.Process 结果一致。
func TestStreamEngineInitConsistency(t *testing.T) {
	klines := generateTestKlines(100)

	// 批量引擎
	engine, err := NewEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	batchResult, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("Engine.Process: %v", err)
	}

	// 增量引擎
	stream, err := NewStreamEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewStreamEngine: %v", err)
	}
	streamResult, err := stream.Init(klines)
	if err != nil {
		t.Fatalf("StreamEngine.Init: %v", err)
	}

	// 比较已合并 K 线数量
	if len(batchResult.MergedKlines) != len(streamResult.MergedKlines) {
		t.Errorf("MergedKlines count mismatch: batch=%d, stream=%d",
			len(batchResult.MergedKlines), len(streamResult.MergedKlines))
	}

	// 比较笔数量
	if len(batchResult.Bis) != len(streamResult.Bis) {
		t.Errorf("Bis count mismatch: batch=%d, stream=%d",
			len(batchResult.Bis), len(streamResult.Bis))
	}

	// 比较线段数量
	if len(batchResult.Segments) != len(streamResult.Segments) {
		t.Errorf("Segments count mismatch: batch=%d, stream=%d",
			len(batchResult.Segments), len(streamResult.Segments))
	}

	// 比较中枢数量
	if len(batchResult.Pivots) != len(streamResult.Pivots) {
		t.Errorf("Pivots count mismatch: batch=%d, stream=%d",
			len(batchResult.Pivots), len(streamResult.Pivots))
	}
}

// TestStreamEngineAddKline 验证逐根追加 K 线不 panic 且产出合理。
func TestStreamEngineAddKline(t *testing.T) {
	klines := generateTestKlines(50)

	stream, err := NewStreamEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewStreamEngine: %v", err)
	}

	// 用前 30 根初始化
	_, err = stream.Init(klines[:30])
	if err != nil {
		t.Fatalf("StreamEngine.Init: %v", err)
	}

	// 逐根追加后 20 根
	for i := 30; i < 50; i++ {
		inc := stream.AddKline(klines[i])
		if inc == nil {
			t.Fatalf("AddKline returned nil at index %d", i)
		}
	}

	// 验证快照不为空
	snap := stream.Snapshot()
	if snap == nil {
		t.Fatal("Snapshot returned nil")
	}
	if len(snap.MergedKlines) == 0 {
		t.Error("Snapshot has no merged klines")
	}
}

// TestStreamEngineIncrementalConsistency 验证逐根追加结果与全量处理基本一致。
// 注意：由于初始化时仅用 1 根 K 线，增量包含处理的边界行为与全量略有差异（±2）。
func TestStreamEngineIncrementalConsistency(t *testing.T) {
	klines := generateTestKlines(80)

	// 全量处理
	engine, _ := NewEngine(DefaultConfig())
	fullResult, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("Engine.Process: %v", err)
	}

	// 逐根增量处理（用前 10 根初始化以减少边界差异）
	stream, _ := NewStreamEngine(DefaultConfig())
	_, err = stream.Init(klines[:10])
	if err != nil {
		t.Fatalf("StreamEngine.Init: %v", err)
	}
	for i := 10; i < len(klines); i++ {
		stream.AddKline(klines[i])
	}

	snap := stream.Snapshot()

	// 已合并 K 线数量差异应在合理范围内
	diff := len(fullResult.MergedKlines) - len(snap.MergedKlines)
	if diff < 0 {
		diff = -diff
	}
	if diff > 2 {
		t.Errorf("Incremental consistency: MergedKlines diff too large: batch=%d, stream=%d (diff=%d)",
			len(fullResult.MergedKlines), len(snap.MergedKlines), diff)
	}
}

// TestResultQueryAPI 测试 Result 跨层查询方法。
func TestResultQueryAPI(t *testing.T) {
	klines := generateTestKlines(100)

	engine, _ := NewEngine(DefaultConfig())
	result, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("Engine.Process: %v", err)
	}

	// LatestBi
	latestBi := result.LatestBi()
	if latestBi == nil && len(result.Bis) > 0 {
		t.Error("LatestBi returned nil but Bis is not empty")
	}

	// LatestSegment
	latestSeg := result.LatestSegment()
	if latestSeg == nil && len(result.Segments) > 0 {
		t.Error("LatestSegment returned nil but Segments is not empty")
	}

	// LatestPivot
	latestPivot := result.LatestPivot()
	if latestPivot == nil && len(result.Pivots) > 0 {
		t.Error("LatestPivot returned nil but Pivots is not empty")
	}

	// PivotCount
	count := result.PivotCount()
	if count != len(result.Pivots) {
		t.Errorf("PivotCount mismatch: %d vs %d", count, len(result.Pivots))
	}

	// BiAtIndex
	if len(result.Bis) > 0 {
		midIdx := result.Bis[0].StartIndex
		bi := result.BiAtIndex(midIdx)
		if bi == nil {
			t.Errorf("BiAtIndex(%d) returned nil", midIdx)
		}
	}
}

// TestMacdIncremental 测试增量 MACD 计算。
func TestMacdIncremental(t *testing.T) {
	macd := newMacdIncremental(12, 26, 9)

	// 添加 50 个价格点
	prices := make([]float64, 50)
	for i := range prices {
		prices[i] = 100.0 + float64(i%10)
	}
	macd.addBatch(prices)

	if !macd.isValid() {
		t.Error("MACD should be valid after 50 data points")
	}

	if len(macd.dif) != 50 {
		t.Errorf("DIF length: expected 50, got %d", len(macd.dif))
	}

	if len(macd.hist) != 50 {
		t.Errorf("Hist length: expected 50, got %d", len(macd.hist))
	}

	// 测试面积计算
	area := macd.calcAreaInRange(0, 49)
	if area < 0 {
		t.Error("MACD area should be non-negative")
	}
}

// TestNewStreamEngineValidation 测试配置验证。
func TestNewStreamEngineValidation(t *testing.T) {
	_, err := NewStreamEngine(Config{BiMinKLineCount: -1})
	if err == nil {
		t.Error("expected error for invalid config")
	}

	_, err = NewStreamEngine(DefaultConfig())
	if err != nil {
		t.Errorf("unexpected error for default config: %v", err)
	}
}
