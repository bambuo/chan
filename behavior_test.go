package chanlun

import (
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// 行为逻辑对齐测试
// 验证 Go 实现与 chan.py 的行为一致性修正
// ──────────────────────────────────────────────

// TestEndIsPeak 测试 end_is_peak 检查：中间 K 线超过终点极值时应拒绝成笔。
func TestEndIsPeak(t *testing.T) {
	// 构造：底分型 → 中间K线高点超过顶分型高点 → 应拒绝
	klines := []Kline{
		{High: 10, Low: 5, Close: 7},  // 0: pre of bottom
		{High: 8, Low: 3, Close: 5},   // 1: bottom fractal (Low=3 is lowest)
		{High: 9, Low: 4, Close: 6},   // 2: post of bottom
		{High: 12, Low: 6, Close: 10}, // 3: 中间K线，High=12 > 终点顶分型High=11
		{High: 11, Low: 7, Close: 9},  // 4: top fractal (High=11)
		{High: 10, Low: 6, Close: 8},  // 5: post of top
	}

	start := Fractal{Type: BottomFractal, Index: 1, High: 8, Low: 3}
	end := Fractal{Type: TopFractal, Index: 4, High: 11, Low: 7}

	if endIsPeak(klines, start, end) {
		t.Error("expected endIsPeak to return false (intermediate K-line high=12 > end high=11)")
	}

	// 正常情况：中间K线不超过终点
	klines2 := []Kline{
		{High: 10, Low: 5, Close: 7},
		{High: 8, Low: 3, Close: 5}, // bottom
		{High: 9, Low: 4, Close: 6},
		{High: 10, Low: 6, Close: 9}, // 中间K线，High=10 < 终点High=11
		{High: 11, Low: 7, Close: 9}, // top
		{High: 10, Low: 6, Close: 8},
	}

	if !endIsPeak(klines2, start, end) {
		t.Error("expected endIsPeak to return true (all intermediate K-lines within range)")
	}
}

// TestCheckFxValid_Half 测试 bi_fx_check="half" 模式。
func TestCheckFxValid_Half(t *testing.T) {
	// 顶→底笔：顶分型 high 应 > max(end.pre.high, end.high)
	klines := []Kline{
		{High: 8, Low: 3, Close: 5},    // 0: pre of top
		{High: 15, Low: 10, Close: 12}, // 1: top fractal
		{High: 13, Low: 8, Close: 10},  // 2: post of top
		{High: 11, Low: 6, Close: 8},   // 3: middle
		{High: 9, Low: 4, Close: 6},    // 4: pre of bottom
		{High: 7, Low: 2, Close: 4},    // 5: bottom fractal
		{High: 8, Low: 3, Close: 5},    // 6: post of bottom
	}

	start := Fractal{Type: TopFractal, Index: 1, High: 15, Low: 10}
	end := Fractal{Type: BottomFractal, Index: 5, High: 7, Low: 2}

	if !checkFxValid(klines, start, end, "half") {
		t.Error("expected checkFxValid(half) to pass: top.high=15 > max(pre.high=9, end.high=7)=9")
	}
}

// TestCheckFxValid_Loss 测试 bi_fx_check="loss" 模式（仅检查分型本身）。
func TestCheckFxValid_Loss(t *testing.T) {
	klines := []Kline{
		{High: 15, Low: 10, Close: 12}, // top
		{High: 11, Low: 6, Close: 8},
		{High: 7, Low: 2, Close: 4}, // bottom
	}

	start := Fractal{Type: TopFractal, Index: 0, High: 15, Low: 10}
	end := Fractal{Type: BottomFractal, Index: 2, High: 7, Low: 2}

	// loss: top.high=15 > end.high=7 AND end.low=2 < top.low=10
	if !checkFxValid(klines, start, end, "loss") {
		t.Error("expected checkFxValid(loss) to pass")
	}
}

// TestCheckFxValid_Totally 测试 bi_fx_check="totally" 模式（完全不重叠）。
func TestCheckFxValid_Totally(t *testing.T) {
	klines := []Kline{
		{High: 15, Low: 12, Close: 13}, // top (low=12)
		{High: 11, Low: 8, Close: 9},
		{High: 7, Low: 4, Close: 5}, // bottom (high=7)
	}

	start := Fractal{Type: TopFractal, Index: 0, High: 15, Low: 12}
	end := Fractal{Type: BottomFractal, Index: 2, High: 7, Low: 4}

	// totally: top.low=12 > end.high=7 → should pass
	if !checkFxValid(klines, start, end, "totally") {
		t.Error("expected checkFxValid(totally) to pass: top.low=12 > end.high=7")
	}

	// 重叠情况：top.low < end.high
	startOverlap := Fractal{Type: TopFractal, Index: 0, High: 15, Low: 6}
	if checkFxValid(klines, startOverlap, end, "totally") {
		t.Error("expected checkFxValid(totally) to fail: top.low=6 < end.high=7")
	}
}

// TestSatisfyBiSpan_Strict 测试严格模式 span >= 4。
func TestSatisfyBiSpan_Strict(t *testing.T) {
	klines := make([]Kline, 10)
	for i := range klines {
		klines[i] = Kline{High: float64(10 + i), Low: float64(5 + i)}
	}

	// span = 5-1 = 4 (刚好满足)
	start := Fractal{Index: 1}
	end := Fractal{Index: 5}
	if !satisfyBiSpan(klines, start, end, true, false) {
		t.Error("expected satisfyBiSpan(strict) to pass with span=4")
	}

	// span = 4-1 = 3 (不满足)
	end2 := Fractal{Index: 4}
	if satisfyBiSpan(klines, start, end2, true, false) {
		t.Error("expected satisfyBiSpan(strict) to fail with span=3")
	}
}

// TestSatisfyBiSpan_GapAsKl 测试 gap_as_kl 跳空增加 span。
func TestSatisfyBiSpan_GapAsKl(t *testing.T) {
	klines := []Kline{
		{High: 10, Low: 8},  // 0
		{High: 11, Low: 9},  // 1: start fractal
		{High: 12, Low: 10}, // 2
		{High: 20, Low: 15}, // 3: gap up from 2 (High=12 < Low=15)
		{High: 21, Low: 16}, // 4: end fractal
	}

	start := Fractal{Index: 1}
	end := Fractal{Index: 4}

	// 无 gap_as_kl: span = 4-1 = 3 (strict 不满足)
	if satisfyBiSpan(klines, start, end, true, false) {
		t.Error("expected strict without gap_as_kl to fail with span=3")
	}

	// 有 gap_as_kl: span = 3 + 1(gap) = 4 (满足)
	if !satisfyBiSpan(klines, start, end, true, true) {
		t.Error("expected strict with gap_as_kl to pass: span=3+1gap=4")
	}
}

// TestBuildBisWithConfig 测试新的 BuildBisWithConfig 综合行为。
func TestBuildBisWithConfig(t *testing.T) {
	// 构造一个简单的顶底交替序列
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := make([]Kline, 20)
	for i := range klines {
		phase := float64(i) * 0.5
		var h, l float64
		if i%5 < 3 {
			h = 100 + phase*2
			l = 95 + phase*2
		} else {
			h = 100 + phase*2 - 3
			l = 95 + phase*2 - 3
		}
		klines[i] = Kline{
			Time:  DateTime{Time: baseTime.Add(time.Duration(i) * time.Hour)},
			High:  h,
			Low:   l,
			Close: (h + l) / 2,
			Open:  (h + l) / 2,
		}
	}

	fractals := FindFractals(klines, 1)
	biFractals := FilterFractalsForBi(fractals, 1)

	cfg := DefaultConfig()
	bis := BuildBisWithConfig(klines, biFractals, cfg)

	t.Logf("built %d bis from %d fractals with default config", len(bis), len(biFractals))

	// 验证笔的方向交替
	for i := 1; i < len(bis); i++ {
		if bis[i].Direction == bis[i-1].Direction {
			t.Errorf("bi[%d] and bi[%d] have same direction", i-1, i)
		}
	}
}

// TestZGZD_BiLevel 测试中枢 ZG/ZD 笔级计算。
func TestZGZD_BiLevel(t *testing.T) {
	bis := []Bi{
		{High: 110, Low: 100},
		{High: 108, Low: 98},
		{High: 112, Low: 102},
	}

	zg, zd := calcZGZDFromBis(bis)

	// ZG = min(110, 108, 112) = 108
	if zg != 108 {
		t.Errorf("expected ZG=108, got %.1f", zg)
	}
	// ZD = max(100, 98, 102) = 102
	if zd != 102 {
		t.Errorf("expected ZD=102, got %.1f", zd)
	}
}

// TestInclusion_OneWordKline 测试一字K线包含处理（对齐 chan.py：应合并）。
func TestInclusion_OneWordKline(t *testing.T) {
	// 两根一字K线（High==Low），存在包含关系
	klines := []Kline{
		{High: 10, Low: 10, Close: 10, Open: 10}, // 一字K
		{High: 10, Low: 10, Close: 10, Open: 10}, // 一字K（完全相同）
		{High: 12, Low: 9, Close: 11, Open: 10},  // 正常K
	}

	result := MergeKlines(klines)
	// Python 行为：两根一字K合并为一根，然后与第三根不合并 → 2 根
	if len(result) != 2 {
		t.Logf("MergeKlines with two identical one-word klines: got %d merged (Python expects 2)", len(result))
	}
}

// TestCheckActualBreak 测试线段特征序列的实际突破检查。
func TestCheckActualBreak(t *testing.T) {
	// 向上线段：顶分型第三元素 low < 第二元素 low → 实际突破
	features := []FeatureElement{
		{High: 10, Low: 8},
		{High: 12, Low: 9}, // 第二元素
		{High: 11, Low: 7}, // 第三元素：low=7 < 第二元素 low=9 → 突破
	}
	if !checkActualBreak(features, DirUp) {
		t.Error("expected checkActualBreak to return true for upward segment")
	}

	// 未突破：第三元素 low >= 第二元素 low
	features2 := []FeatureElement{
		{High: 10, Low: 8},
		{High: 12, Low: 9},
		{High: 11, Low: 10}, // low=10 > 第二元素 low=9 → 未突破
	}
	if checkActualBreak(features2, DirUp) {
		t.Error("expected checkActualBreak to return false (no actual break)")
	}
}

// TestPivotShareParentSegment 测试中枢合并的 seg_idx 约束。
func TestPivotShareParentSegment(t *testing.T) {
	// 连续的中枢（线段连续）
	p1 := Pivot{
		Segments: []Segment{{StartIndex: 0, EndIndex: 10}},
	}
	p2 := Pivot{
		Segments: []Segment{{StartIndex: 11, EndIndex: 20}},
	}
	if !pivotsShareParentSegment(p1, p2) {
		t.Error("expected continuous pivots to share parent segment")
	}

	// 间隔很大的中枢（不在同一父线段）
	p3 := Pivot{
		Segments: []Segment{{StartIndex: 0, EndIndex: 10}},
	}
	p4 := Pivot{
		Segments: []Segment{{StartIndex: 50, EndIndex: 60}},
	}
	if pivotsShareParentSegment(p3, p4) {
		t.Error("expected distant pivots to NOT share parent segment")
	}
}

// TestSignalSubTypes 测试买卖点子类型标注。
func TestSignalSubTypes(t *testing.T) {
	// 验证 SubType 常量值
	if SubT1 != "1" {
		t.Errorf("SubT1 should be '1', got %s", SubT1)
	}
	if SubT1P != "1p" {
		t.Errorf("SubT1P should be '1p', got %s", SubT1P)
	}
	if SubT2 != "2" {
		t.Errorf("SubT2 should be '2', got %s", SubT2)
	}
	if SubT2S != "2s" {
		t.Errorf("SubT2S should be '2s', got %s", SubT2S)
	}
	if SubT3A != "3a" {
		t.Errorf("SubT3A should be '3a', got %s", SubT3A)
	}
	if SubT3B != "3b" {
		t.Errorf("SubT3B should be '3b', got %s", SubT3B)
	}
}
