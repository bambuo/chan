package chanlun

import (
	"math"
	"testing"
	"time"
)

func candle(h, l float64) Candle {
	return Candle{High: h, Low: l, Time: time.Now()}
}

func candleOHLC(o, h, l, c float64) Candle {
	return Candle{Open: o, High: h, Low: l, Close: c, Time: time.Now()}
}

// ──────────────────────────────────────────────
// §1  K 线包含处理 测试
// ──────────────────────────────────────────────

func TestMergeCandles_NoContainment(t *testing.T) {
	// 无包含关系
	input := []Candle{
		candle(10, 8),
		candle(12, 10),
		candle(14, 12),
	}
	result := MergeCandles(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 candles, got %d", len(result))
	}
}

func TestMergeCandles_UpContainment(t *testing.T) {
	// 向上方向，K3 被 K2 包含：取高高、高低
	// K1(10,8) → K2(12,10) 构成向上关系
	// K3(11, 10.2): H=11≤12, L=10.2≥10 → 被 K2 包含
	input := []Candle{
		candle(10, 8),
		candle(12, 10),
		candle(11, 10.2),
	}
	result := MergeCandles(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(result))
	}
	// 向上合并：H=max(12,11)=12, L=max(10,10.2)=10.2
	merged := result[1]
	if merged.High != 12 || merged.Low != 10.2 {
		t.Errorf("up merge: expected H=12 L=10.2, got H=%.1f L=%.1f", merged.High, merged.Low)
	}
}

func TestMergeCandles_DownContainment(t *testing.T) {
	// 向下方向，K3 被 K2 包含：取低高、低低
	// K1(12,10) → K2(10,8) 构成向下关系
	// K3(9.5, 8.5): H=9.5≤10, L=8.5≥8 → 被 K2 包含
	input := []Candle{
		candle(12, 10),
		candle(10, 8),
		candle(9.5, 8.5),
	}
	result := MergeCandles(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(result))
	}
	// 向下合并：H=min(10,9.5)=9.5, L=min(8,8.5)=8
	merged := result[1]
	if merged.High != 9.5 || merged.Low != 8 {
		t.Errorf("down merge: expected H=9.5 L=8, got H=%.1f L=%.1f", merged.High, merged.Low)
	}
}

func TestMergeCandles_MultiLevelContainment(t *testing.T) {
	// 连续多层包含
	// K1(10,8) → K2(12,10) 向上
	// K3(11,10.2) 被 K2 包含 → 合并为 (12,10.2)
	// K4(11.5,10.5) 被合并后的 (12,10.2) 包含 → 合并为 (12,10.5)
	input := []Candle{
		candle(10, 8),
		candle(12, 10),
		candle(11, 10.2),
		candle(11.5, 10.5),
	}
	result := MergeCandles(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(result))
	}
	merged := result[1]
	if merged.High != 12 || merged.Low != 10.5 {
		t.Errorf("multi-level up: expected H=12 L=10.5, got H=%.1f L=%.1f", merged.High, merged.Low)
	}
}

func TestMergeCandles_NilEmpty(t *testing.T) {
	result := MergeCandles(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
	result = MergeCandles([]Candle{})
	if len(result) != 0 {
		t.Error("expected empty for empty input")
	}
}

func TestMergeCandles_Single(t *testing.T) {
	input := []Candle{candle(10, 8)}
	result := MergeCandles(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 candle, got %d", len(result))
	}
}

// ──────────────────────────────────────────────
// §2  分型 测试
// ──────────────────────────────────────────────

func TestFindFractals_TopBottom(t *testing.T) {
		// 标准顶分型 + 底分型
		// 顶分型@1: 中间K线(15,13), 左(10,8), 右(12,10)
		// 底分型@5: 中间K线(7,5), 左(12,10), 右(14,11)
		// 间隔=5-1-3=1 ≥ minGap(1) ✓
		input := []Candle{
			candle(10, 8),   // 0
			candle(15, 13),  // 1 → 顶分型中间
			candle(12, 10),  // 2
			candle(12, 10),  // 3
			candle(12, 10),  // 4 (filler)
			candle(7, 5),    // 5 → 底分型中间
			candle(14, 11),  // 6
		}
	result := FindFractals(input, 1)
	if len(result) != 2 {
		t.Fatalf("expected 2 fractals, got %d", len(result))
	}
	if result[0].Type != TopFractal {
		t.Error("first fractal should be top")
	}
	if result[1].Type != BottomFractal {
		t.Error("second fractal should be bottom")
	}
}

func TestFindFractals_SameDirectionMerge(t *testing.T) {
	// 同向多分型取极值 — 连续两个顶分型，取更高的
	// 顶分型@1: H=15, 顶分型@4: H=18
	// 同向合并取 H=18
	input := []Candle{
		candle(10, 8),   // 0
		candle(15, 13),  // 1 → 顶(低)
		candle(12, 10),  // 2
		candle(13, 11),  // 3
		candle(18, 15),  // 4 → 顶(高)
		candle(14, 12),  // 5
	}
	result := FindFractals(input, 1)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged fractal, got %d", len(result))
	}
	if result[0].High != 18 {
		t.Errorf("expected merged top H=18, got H=%.1f", result[0].High)
	}
}

func TestFindFractals_InsuffientGap(t *testing.T) {
	// 异向分型间隔不足 minGap 被过滤
	// 顶@1, 底@3: 间隔=3-1-2=0 < minGap(2)
	input := []Candle{
		candle(10, 8),  // 0
		candle(15, 13), // 1 → 顶
		candle(12, 10), // 2
		candle(9, 7),   // 3 → 底（间隔不足）
		candle(11, 9),  // 4
	}
	result := FindFractals(input, 2)
	if len(result) != 1 {
		t.Fatalf("expected 1 fractal (gap filtered), got %d", len(result))
	}
}

func TestFindFractals_NotEnoughCandles(t *testing.T) {
	result := FindFractals([]Candle{candle(10, 8), candle(12, 10)}, 1)
	if result != nil {
		t.Error("expected nil for < 3 candles")
	}
}

func TestFindFractals_OnlyTop(t *testing.T) {
	// 仅有一个顶分型
	input := []Candle{
		candle(10, 8),
		candle(15, 13), // 顶
		candle(12, 10),
	}
	result := FindFractals(input, 1)
	if len(result) != 1 || result[0].Type != TopFractal {
		t.Error("expected 1 top fractal")
	}
}

// ──────────────────────────────────────────────
// §3  笔 测试
// ──────────────────────────────────────────────

func TestBuildBis_DownStroke(t *testing.T) {
	// 顶→底：向下笔
	// 顶@1, 底@5: 独立K线=5-1-3=1 ≥ 1 ✓
	candles := []Candle{
		candle(10, 8),  // 0
		candle(15, 13), // 1 → 顶
		candle(12, 10), // 2
		candle(11, 9),  // 3
		candle(11, 9),  // 4 (filler)
		candle(7, 5),   // 5 → 底
		candle(13, 11), // 6
	}
		fractals := []Fractal{
			{Type: TopFractal, Index: 1, High: 15, Low: 13},
			{Type: BottomFractal, Index: 5, Low: 5, High: 7},
		}
		bis := BuildBis(candles, fractals, 1, 0)
		if len(bis) != 1 {
			t.Fatalf("expected 1 bi, got %d", len(bis))
		}
		if bis[0].Direction != DirDown {
			t.Error("expected downward bi (top→bottom)")
		}
	}
	
	func TestBuildBis_UpStroke(t *testing.T) {
		// 底→顶：向上笔
		// 底@1, 顶@5: 独立K线=5-1-3=1 ≥ 1 ✓
		candles := []Candle{
			candle(12, 10), // 0
			candle(7, 5),   // 1 → 底
			candle(10, 8),  // 2
			candle(11, 9),  // 3
			candle(11, 9),  // 4 (filler)
			candle(15, 13), // 5 → 顶
			candle(13, 11), // 6
		}
		fractals := []Fractal{
			{Type: BottomFractal, Index: 1, Low: 5, High: 7},
			{Type: TopFractal, Index: 5, High: 15, Low: 13},
		}
	bis := BuildBis(candles, fractals, 1, 0)
	if len(bis) != 1 {
		t.Fatalf("expected 1 bi, got %d", len(bis))
	}
	if bis[0].Direction != DirUp {
		t.Error("expected upward bi (bottom→top)")
	}
}

func TestBuildBis_Alternation(t *testing.T) {
	// 向上+向下笔序列
	// 底@1, 顶@5: 间隔=5-1-3=1 ✓ → 向上笔
	// 顶@5, 底@9: 间隔=9-5-3=1 ✓ → 向下笔
	candles := []Candle{
		candle(12, 10), // 0
		candle(7, 5),   // 1 → 底
		candle(9, 7),   // 2
		candle(11, 9),  // 3
		candle(11, 9),  // 4 (filler)
		candle(15, 13), // 5 → 顶
		candle(13, 11), // 6
		candle(12, 10), // 7
		candle(12, 10), // 8 (filler)
		candle(8, 6),   // 9 → 底
		candle(14, 12), // 10
	}
	fractals := []Fractal{
		{Type: BottomFractal, Index: 1, Low: 5, High: 7},
		{Type: TopFractal, Index: 5, High: 15, Low: 13},
		{Type: BottomFractal, Index: 9, Low: 6, High: 8},
	}
	bis := BuildBis(candles, fractals, 1, 0)
	if len(bis) != 2 {
		t.Fatalf("expected 2 bis (up+down), got %d", len(bis))
	}
	if bis[0].Direction != DirUp {
		t.Error("first bi should be up (bottom→top)")
	}
	if bis[1].Direction != DirDown {
		t.Error("second bi should be down (top→bottom)")
	}
}

func TestBuildBis_FractalTypeMismatch_NoStroke(t *testing.T) {
	// 同类型分型不构成笔
	candles := []Candle{
		candle(10, 8),
		candle(15, 13), // 顶
		candle(12, 10),
		candle(16, 14), // 顶(更高)
		candle(13, 11),
	}
	fractals := []Fractal{
		{Type: TopFractal, Index: 1, High: 15},
		{Type: TopFractal, Index: 3, High: 16},
	}
	bis := BuildBis(candles, fractals, 1, 0)
	if len(bis) != 0 {
		t.Error("expected no bi for same type fractals")
	}
}

func TestBuildBis_InsuffientGap(t *testing.T) {
	// 不足最小 K 线数
	// 底@1, 顶@3: 独立K线=3-1-2=0 < 2
	candles := []Candle{
		candle(10, 8),  // 0
		candle(7, 5),   // 1 → 底
		candle(12, 10), // 2
		candle(15, 13), // 3 → 顶
	}
	fractals := []Fractal{
		{Type: BottomFractal, Index: 1, Low: 5},
		{Type: TopFractal, Index: 3, High: 15},
	}
	bis := BuildBis(candles, fractals, 2, 0)
	if len(bis) != 0 {
		t.Error("expected no bi for insufficient gap")
	}
}

// ──────────────────────────────────────────────
// §3.5  笔的包含处理 测试
// ──────────────────────────────────────────────

func TestMergeBis_UpContainment(t *testing.T) {
	// 向上笔序列中包含处理：取高高、高低
	bis := []Bi{
		{Direction: DirUp, High: 15, Low: 10, StartIndex: 0, EndIndex: 5, StartPrice: 10, EndPrice: 15, KLineCount: 5},
		{Direction: DirUp, High: 14, Low: 11, StartIndex: 5, EndIndex: 10, StartPrice: 12, EndPrice: 14, KLineCount: 5},
	}
	merged := MergeBis(bis)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged bi, got %d", len(merged))
	}
	if merged[0].High != 15 || merged[0].Low != 11 {
		t.Errorf("up merge: expected H=15 L=11, got H=%.1f L=%.1f", merged[0].High, merged[0].Low)
	}
	if merged[0].OriginalCount != 2 {
		t.Errorf("expected OriginalCount=2, got %d", merged[0].OriginalCount)
	}
}

func TestMergeBis_DownContainment(t *testing.T) {
	// 向下笔序列中包含处理：取低高、低低
	bis := []Bi{
		{Direction: DirDown, High: 15, Low: 10, StartIndex: 0, EndIndex: 5, StartPrice: 15, EndPrice: 10, KLineCount: 5},
		{Direction: DirDown, High: 14, Low: 11, StartIndex: 5, EndIndex: 10, StartPrice: 14, EndPrice: 11, KLineCount: 5},
	}
	merged := MergeBis(bis)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged bi, got %d", len(merged))
	}
	// 向下取低高、低低
	if merged[0].High != 14 || merged[0].Low != 10 {
		t.Errorf("down merge: expected H=14 L=10, got H=%.1f L=%.1f", merged[0].High, merged[0].Low)
	}
}

func TestMergeBis_NoContainment_DifferentDirection(t *testing.T) {
	// 不同方向不合并
	bis := []Bi{
		{Direction: DirUp, High: 15, Low: 10},
		{Direction: DirDown, High: 12, Low: 8},
	}
	merged := MergeBis(bis)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged bis, got %d", len(merged))
	}
}

func TestMergeBis_Single(t *testing.T) {
	bis := []Bi{{Direction: DirUp, High: 15, Low: 10}}
	merged := MergeBis(bis)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged bi, got %d", len(merged))
	}
}

func TestMergeBis_ReverseContainment(t *testing.T) {
	// 后一笔包含前一笔，应替换
	bis := []Bi{
		{Direction: DirUp, High: 13, Low: 11, StartIndex: 0, EndIndex: 5, StartPrice: 11, EndPrice: 13, KLineCount: 5},
		{Direction: DirUp, High: 15, Low: 10, StartIndex: 5, EndIndex: 10, StartPrice: 10, EndPrice: 15, KLineCount: 5},
	}
	merged := MergeBis(bis)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged bi, got %d", len(merged))
	}
	// 后一笔更大，应合并到后一笔的范围
	if merged[0].OriginalCount != 2 {
		t.Errorf("expected OriginalCount=2, got %d", merged[0].OriginalCount)
	}
}

func TestMergeBis_ThreeInARow(t *testing.T) {
	// 三支同向笔依次包含
	// Bi1(15,11) 包含 Bi2(14,12) → 合并为 (15,12)
	// 合并后的 (15,12) 包含 Bi3(14.5,12.5)
	bis := []Bi{
		{Direction: DirUp, High: 15, Low: 11, StartIndex: 0, EndIndex: 5, StartPrice: 11, EndPrice: 15, KLineCount: 5},
		{Direction: DirUp, High: 14, Low: 12, StartIndex: 5, EndIndex: 10, StartPrice: 12, EndPrice: 14, KLineCount: 5},
		{Direction: DirUp, High: 14.5, Low: 12.5, StartIndex: 10, EndIndex: 15, StartPrice: 12.5, EndPrice: 14.5, KLineCount: 5},
	}
	merged := MergeBis(bis)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged bi, got %d", len(merged))
	}
	// 全部合并
	if merged[0].OriginalCount != 3 {
		t.Errorf("expected OriginalCount=3, got %d", merged[0].OriginalCount)
	}
}

// ──────────────────────────────────────────────
// §4  线段 测试
// ──────────────────────────────────────────────

func makeBi(dir Direction, start, end int, high, low float64) MergedBi {
	return MergedBi{
		Bi: Bi{
			Direction:  dir,
			StartIndex: start,
			EndIndex:   end,
			High:       high,
			Low:        low,
			StartPrice: low,
			EndPrice:   high,
			KLineCount: end - start + 1,
		},
		OriginalCount: 1,
	}
}

func TestBuildSegments_ThreeBiOverlap(t *testing.T) {
	// 三笔重叠形成线段
	// 向上-向下-向上
	bis := []MergedBi{
		makeBi(DirUp, 0, 5, 15, 10),
		makeBi(DirDown, 5, 10, 13, 8),
		makeBi(DirUp, 10, 15, 16, 11),
	}
	segments := BuildSegments(bis)
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].Direction != DirUp {
		t.Error("expected upward segment")
	}
}

func TestBuildSegments_NoOverlap(t *testing.T) {
	// 三笔无重叠 → 不构成线段
	bis := []MergedBi{
		makeBi(DirUp, 0, 5, 20, 15),
		makeBi(DirDown, 5, 10, 14, 10),
		makeBi(DirUp, 10, 15, 18, 12),
	}
	segments := BuildSegments(bis)
	if len(segments) != 0 {
		t.Errorf("expected 0 segments (no overlap), got %d", len(segments))
	}
}

func TestBuildSegments_FewerThanThree(t *testing.T) {
	bis := []MergedBi{
		makeBi(DirUp, 0, 5, 15, 10),
		makeBi(DirDown, 5, 10, 13, 8),
	}
	segments := BuildSegments(bis)
	if len(segments) != 0 {
		t.Errorf("expected 0 segments (<3 bis), got %d", len(segments))
	}
}

// ──────────────────────────────────────────────
// §5  中枢 测试
// ──────────────────────────────────────────────

func TestFindPivots_ThreeSegmentsOverlap(t *testing.T) {
	// 三段线段重叠形成中枢
	segments := []Segment{
		{StartIndex: 0, EndIndex: 10, Top: 15, Bottom: 10, Direction: DirUp},
		{StartIndex: 10, EndIndex: 20, Top: 13, Bottom: 8, Direction: DirDown},
		{StartIndex: 20, EndIndex: 30, Top: 16, Bottom: 11, Direction: DirUp},
	}
	pivots := FindPivots(segments)
	if len(pivots) != 1 {
		t.Fatalf("expected 1 pivot, got %d", len(pivots))
	}
	// ZG = min(15,13,16) = 13
	// ZD = max(10,8,11) = 11
	if pivots[0].ZG != 13 || pivots[0].ZD != 11 {
		t.Errorf("expected ZG=13 ZD=11, got ZG=%.1f ZD=%.1f", pivots[0].ZG, pivots[0].ZD)
	}
	if pivots[0].State != PivotFormed {
		t.Errorf("expected state PivotFormed, got %d", pivots[0].State)
	}
}

func TestFindPivots_NoOverlap(t *testing.T) {
	segments := []Segment{
		{StartIndex: 0, EndIndex: 10, Top: 20, Bottom: 15, Direction: DirUp},
		{StartIndex: 10, EndIndex: 20, Top: 12, Bottom: 8, Direction: DirDown},
		{StartIndex: 20, EndIndex: 30, Top: 25, Bottom: 22, Direction: DirUp},
	}
	pivots := FindPivots(segments)
	if len(pivots) != 0 {
		t.Errorf("expected 0 pivots (no overlap), got %d", len(pivots))
	}
}

// ──────────────────────────────────────────────
// §6  走势类型 测试
// ──────────────────────────────────────────────

func TestClassifyTrends_RangeOnly(t *testing.T) {
	// 单个中枢 → 盘整
	pivots := []Pivot{
		{ZG: 13, ZD: 11, StartIndex: 0, EndIndex: 30},
	}
	trends := ClassifyTrends(pivots)
	if len(trends) != 1 {
		t.Fatalf("expected 1 trend, got %d", len(trends))
	}
	if trends[0].Type != RangeOnly {
		t.Errorf("expected RangeOnly, got %d", trends[0].Type)
	}
}

func TestClassifyTrends_UpTrend(t *testing.T) {
	// 两个依次向上的中枢 → 上涨趋势
	pivots := []Pivot{
		{ZG: 13, ZD: 11, StartIndex: 0, EndIndex: 30},
		{ZG: 18, ZD: 15, StartIndex: 35, EndIndex: 60}, // ZD(15) > ZG(13) → 上涨
	}
	trends := ClassifyTrends(pivots)
	if len(trends) != 1 {
		t.Fatalf("expected 1 trend, got %d", len(trends))
	}
	if trends[0].Type != TrendUp {
		t.Errorf("expected TrendUp, got %d", trends[0].Type)
	}
	if len(trends[0].Pivots) != 2 {
		t.Errorf("expected 2 pivots in trend, got %d", len(trends[0].Pivots))
	}
}

func TestClassifyTrends_DownTrend(t *testing.T) {
	// 两个依次向下的中枢 → 下跌趋势
	pivots := []Pivot{
		{ZG: 15, ZD: 13, StartIndex: 0, EndIndex: 30},
		{ZG: 12, ZD: 10, StartIndex: 35, EndIndex: 60}, // ZG(12) < ZD(13) → 下跌
	}
	trends := ClassifyTrends(pivots)
	if len(trends) != 1 {
		t.Fatalf("expected 1 trend, got %d", len(trends))
	}
	if trends[0].Type != TrendDown {
		t.Errorf("expected TrendDown, got %d", trends[0].Type)
	}
}

// ──────────────────────────────────────────────
// §7  背驰 测试
// ──────────────────────────────────────────────

func TestDetectDeviations_UpDeviation(t *testing.T) {
	// 向上顶背驰：后段力度 < 前段力度
	segments := []Segment{
		{Direction: DirUp, StartIndex: 0, EndIndex: 10, Top: 10, Bottom: 5, BiList: []MergedBi{
			{Bi: Bi{StartPrice: 5, EndPrice: 10}},
		}},
		{Direction: DirUp, StartIndex: 11, EndIndex: 20, Top: 15, Bottom: 12, BiList: []MergedBi{
			{Bi: Bi{StartPrice: 12, EndPrice: 15}},
		}},
	}
	macd := make([]float64, 21)
	signal := make([]float64, 21)
	hist := make([]float64, 21)
	for i := 0; i <= 10; i++ {
		hist[i] = 2.0    // 前段MACD面积=22
		macd[i] = 3.0    // 前段DIF极值=3.0
		signal[i] = 1.0
	}
	for i := 11; i <= 20; i++ {
		hist[i] = 0.5    // 后段MACD面积=5.5 < 22 ✓
		macd[i] = 1.0    // 后段DIF极值=1.0 < 3.0 ✓
		signal[i] = 0.5
	}
	// DIF穿越0轴：DIF*DEA = -0.01 <= 0 ✓
	macd[10] = -0.1
	signal[10] = 0.1

	deviations := DetectDeviations(segments, macd, signal, hist)
	if len(deviations) == 0 {
		t.Fatal("expected deviation detected, got nil")
	}
	if deviations[0].Direction != DirUp {
		t.Error("expected top deviation (DirUp)")
	}
}

func TestDetectDeviations_NoDeviation(t *testing.T) {
	// 力度不衰减且 MACD 不确认 → 无背驰
	// 第一段：平缓上升: Top=10, Bottom=5, 11 bars → force=5/11≈0.45
	// 第二段：陡峭上升: Top=18, Bottom=14, 4 bars → force=4/4=1.0 (更强)
	segments := []Segment{
		{Direction: DirUp, StartIndex: 0, EndIndex: 10, Top: 10, Bottom: 5},
		{Direction: DirUp, StartIndex: 11, EndIndex: 14, Top: 18, Bottom: 14},
	}
	macd := make([]float64, 15)
	signal := make([]float64, 15)
	hist := make([]float64, 15)
	for i := 0; i <= 10; i++ {
		hist[i] = 1.0 // 前段 MACD 面积 = 11
		macd[i] = 5.0 // DIF * DEA = 5 > 0，不穿越
		signal[i] = 1.0
	}
	for i := 11; i <= 14; i++ {
		hist[i] = 4.0 // 后段 MACD 面积 = 16 > 11 (扩大)
		macd[i] = 5.0
		signal[i] = 1.0
	}

	deviations := DetectDeviations(segments, macd, signal, hist)
	if len(deviations) != 0 {
		t.Error("expected no deviation (force not reduced, MACD area expanded)")
	}
}

// ──────────────────────────────────────────────
// §8  买卖点 测试
// ──────────────────────────────────────────────

func TestDetectSignals_BuyPoint1(t *testing.T) {
	// 底背驰 → 一买
	deviation := Deviation{
		Level:     SegmentDeviation,
		Direction: DirDown,
		Type:      "trend",
		SegmentAfter: &Segment{
			Direction: DirDown,
			EndIndex:  20,
			Bottom:    8,
			Top:       12,
		},
	}
	signals := DetectSignals(nil, []Deviation{deviation}, nil, nil)
	if len(signals) == 0 {
		t.Fatal("expected at least 1 signal")
	}
	// 一买信号应该存在
	hasBuy1 := false
	for _, s := range signals {
		if s.Type == BuyPoint1 {
			hasBuy1 = true
			break
		}
	}
	if !hasBuy1 {
		t.Error("expected BuyPoint1 signal")
	}
}

func TestDetectSignals_SellPoint1(t *testing.T) {
	// 顶背驰 → 一卖
	deviation := Deviation{
		Level:     SegmentDeviation,
		Direction: DirUp,
		Type:      "trend",
		SegmentAfter: &Segment{
			Direction: DirUp,
			EndIndex:  20,
			Top:       15,
			Bottom:    10,
		},
	}
	signals := DetectSignals(nil, []Deviation{deviation}, nil, nil)
	hasSell1 := false
	for _, s := range signals {
		if s.Type == SellPoint1 {
			hasSell1 = true
			break
		}
	}
	if !hasSell1 {
		t.Error("expected SellPoint1 signal")
	}
}

// ──────────────────────────────────────────────
// §11  信号评分 测试
// ──────────────────────────────────────────────

func TestScoreSignal_TrendDeviation(t *testing.T) {
	// 走势级别背驰 → 高分
	ctx := &ScoringContext{
		Signal: Signal{
			Type:  BuyPoint1,
			Level: "走势级别",
			Deviation: &Deviation{
				Level: TrendDeviation,
			},
		},
		MultiLevelCount: 3,
	}
	score, _ := ScoreSignal(ctx)
	if score < 0.5 {
		t.Errorf("expected high score (>0.5), got %.2f", score)
	}
}

func TestScoreSignal_BiDeviation(t *testing.T) {
	// 笔背驰 → 低分
	ctx := &ScoringContext{
		Signal: Signal{
			Type:  BuyPoint1,
			Level: "笔级别",
			Deviation: &Deviation{
				Level: BiDeviation,
			},
		},
		MultiLevelCount: 1,
	}
	score, _ := ScoreSignal(ctx)
	if score > 0.5 {
		t.Errorf("expected low score (<0.5) for bi deviation, got %.2f", score)
	}
}

func TestScoreSignal_NoData(t *testing.T) {
	// 无上下文 → 0 分
	score, _ := ScoreSignal(nil)
	if score != 0 {
		t.Errorf("expected 0 for nil context, got %.2f", score)
	}
}

// generateFractalCandles 生成能产生分型的振荡 K 线数据。
func generateFractalCandles(count int) []Candle {
	n := count
	if n < 60 {
		n = 60
	}
	candles := make([]Candle, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range candles {
		phase := float64(i) * 0.4
		base := 100.0 + float64(i)*0.15
		osc := 8.0*math.Sin(phase) + 4.0*math.Sin(phase*2.7)
		mid := base + osc

		candles[i] = Candle{
			Time:   t.Add(time.Duration(i) * time.Hour),
			Open:   mid - 0.5,
			High:   mid + 2.0,
			Low:    mid - 1.5,
			Close:  mid + 0.3,
			Volume: 1000,
		}
	}
	return candles
}

// ──────────────────────────────────────────────
// §9  引擎 端到端集成测试
// ──────────────────────────────────────────────

func TestEngine_Process_EndToEnd(t *testing.T) {
	// 用 200 根振荡数据测试完整流水线
		config := DefaultConfig()
		engine, err := NewEngine(config)
		if err != nil {
			t.Fatalf("NewEngine failed: %v", err)
		}
	
		candles := generateFractalCandles(200)
	
		result, err := engine.Process(candles)
	if err != nil {
		t.Fatalf("Engine.Process failed: %v", err)
	}

	// 验证所有中间结果
	if len(result.MergedCandles) == 0 {
		t.Error("MergedCandles is empty")
	}
	if len(result.Fractals) < 2 {
		t.Errorf("expected >=2 fractals, got %d", len(result.Fractals))
	}
	if len(result.Bis) < 3 {
		t.Errorf("expected >=3 bis, got %d", len(result.Bis))
	}

	t.Logf("MergedCandles: %d, Fractals: %d, Bis: %d, Segments: %d, Pivots: %d, Trends: %d, Signals: %d, Deviations: %d",
		len(result.MergedCandles), len(result.Fractals), len(result.Bis),
		len(result.Segments), len(result.Pivots), len(result.Trends),
		len(result.Signals), len(result.Deviations))
}

func TestEngine_ShortData_ReturnsError(t *testing.T) {
	engine, err := NewEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	_, err = engine.Process([]Candle{candle(10, 8)})
	if err == nil {
		t.Error("expected error for <3 candles")
	}
}

func TestEngine_Update_Incremental(t *testing.T) {
	engine, err := NewEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// 初始数据
	candles := generateFractalCandles(200)

	result1, err := engine.Process(candles)
	if err != nil {
		t.Fatalf("initial Process failed: %v", err)
	}
	_ = result1

	// 增量更新一根 K 线
	last := candles[len(candles)-1]
	newCandle := Candle{
		Time:   last.Time.Add(time.Hour),
		Open:   last.Close - 1,
		High:   last.Close + 3,
		Low:    last.Close - 2,
		Close:  last.Close + 1,
		Volume: 1000,
	}
	result2, err := engine.Update(newCandle)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 增量更新后应该有结果
	if len(result2.Fractals) < 2 {
		t.Errorf("expected >=2 fractals after update, got %d", len(result2.Fractals))
	}
}

func TestWithRealData(t *testing.T) {
	// 用振荡数据模拟真实行情
	engine, err := NewEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	candles := generateFractalCandles(100)

	result, err := engine.Process(candles)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// 基本验证
	if len(result.MergedCandles) == 0 {
		t.Error("no merged candles")
	}
	t.Logf("MergedCandles: %d, Fractals: %d, Bis: %d, Segments: %d, Pivots: %d, Trends: %d, Signals: %d",
		len(result.MergedCandles), len(result.Fractals), len(result.Bis),
		len(result.Segments), len(result.Pivots), len(result.Trends), len(result.Signals))
}
