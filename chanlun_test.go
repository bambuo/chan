package chanlun

import (
	"math"
	"testing"
	"time"
)

func kline(h, l float64) Kline {
	return Kline{High: h, Low: l, Time: time.Now()}
}

func klineOHLC(o, h, l, c float64) Kline {
	return Kline{Open: o, High: h, Low: l, Close: c, Time: time.Now()}
}

// ──────────────────────────────────────────────
// §1  K 线包含处理 测试
// ──────────────────────────────────────────────

func TestMergeKlines_NoContainment(t *testing.T) {
	// 无包含关系
	input := []Kline{
		kline(10, 8),
		kline(12, 10),
		kline(14, 12),
	}
	result := MergeKlines(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 klines, got %d", len(result))
	}
}

func TestMergeKlines_UpContainment(t *testing.T) {
	// 向上方向，K3 被 K2 包含：取高高、高低
	// K1(10,8) → K2(12,10) 构成向上关系
	// K3(11, 10.2): H=11≤12, L=10.2≥10 → 被 K2 包含
	input := []Kline{
		kline(10, 8),
		kline(12, 10),
		kline(11, 10.2),
	}
	result := MergeKlines(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 klines, got %d", len(result))
	}
	// 向上合并：H=max(12,11)=12, L=max(10,10.2)=10.2
	merged := result[1]
	if merged.High != 12 || merged.Low != 10.2 {
		t.Errorf("up merge: expected H=12 L=10.2, got H=%.1f L=%.1f", merged.High, merged.Low)
	}
}

func TestMergeKlines_DownContainment(t *testing.T) {
	// 向下方向，K3 被 K2 包含：取低高、低低
	// K1(12,10) → K2(10,8) 构成向下关系
	// K3(9.5, 8.5): H=9.5≤10, L=8.5≥8 → 被 K2 包含
	input := []Kline{
		kline(12, 10),
		kline(10, 8),
		kline(9.5, 8.5),
	}
	result := MergeKlines(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 klines, got %d", len(result))
	}
	// 向下合并：H=min(10,9.5)=9.5, L=min(8,8.5)=8
	merged := result[1]
	if merged.High != 9.5 || merged.Low != 8 {
		t.Errorf("down merge: expected H=9.5 L=8, got H=%.1f L=%.1f", merged.High, merged.Low)
	}
}

func TestMergeKlines_MultiLevelContainment(t *testing.T) {
	// 连续多层包含
	// K1(10,8) → K2(12,10) 向上
	// K3(11,10.2) 被 K2 包含 → 合并为 (12,10.2)
	// K4(11.5,10.5) 被合并后的 (12,10.2) 包含 → 合并为 (12,10.5)
	input := []Kline{
		kline(10, 8),
		kline(12, 10),
		kline(11, 10.2),
		kline(11.5, 10.5),
	}
	result := MergeKlines(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 klines, got %d", len(result))
	}
	merged := result[1]
	if merged.High != 12 || merged.Low != 10.5 {
		t.Errorf("multi-level up: expected H=12 L=10.5, got H=%.1f L=%.1f", merged.High, merged.Low)
	}
}

func TestMergeKlines_NilEmpty(t *testing.T) {
	result := MergeKlines(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
	result = MergeKlines([]Kline{})
	if len(result) != 0 {
		t.Error("expected empty for empty input")
	}
}

func TestMergeKlines_Single(t *testing.T) {
	input := []Kline{kline(10, 8)}
	result := MergeKlines(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 kline, got %d", len(result))
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
	input := []Kline{
		kline(10, 8),  // 0
		kline(15, 13), // 1 → 顶分型中间
		kline(12, 10), // 2
		kline(12, 10), // 3
		kline(12, 10), // 4 (filler)
		kline(7, 5),   // 5 → 底分型中间
		kline(14, 11), // 6
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
	input := []Kline{
		kline(10, 8),  // 0
		kline(15, 13), // 1 → 顶(低)
		kline(12, 10), // 2
		kline(13, 11), // 3
		kline(18, 15), // 4 → 顶(高)
		kline(14, 12), // 5
	}
	result := FindFractals(input, 1)
	if len(result) != 3 {
		t.Fatalf("expected 3 objective fractals, got %d", len(result))
	}
	filtered := FilterFractalsForBi(result, 1)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered fractal, got %d", len(filtered))
	}
	if filtered[0].High != 18 {
		t.Errorf("expected filtered top H=18, got H=%.1f", filtered[0].High)
	}
}

func TestFindFractals_InsuffientGap(t *testing.T) {
	// 异向分型间隔不足 minGap 被过滤
	// 顶@1, 底@3: 间隔=3-1-2=0 < minGap(2)
	input := []Kline{
		kline(10, 8),  // 0
		kline(15, 13), // 1 → 顶
		kline(12, 10), // 2
		kline(9, 7),   // 3 → 底（间隔不足）
		kline(11, 9),  // 4
	}
	result := FindFractals(input, 2)
	if len(result) != 2 {
		t.Fatalf("expected 2 objective fractals, got %d", len(result))
	}
	filtered := FilterFractalsForBi(result, 2)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered fractal, got %d", len(filtered))
	}
}

func TestFindFractals_NotEnoughKlines(t *testing.T) {
	result := FindFractals([]Kline{kline(10, 8), kline(12, 10)}, 1)
	if result != nil {
		t.Error("expected nil for < 3 klines")
	}
}

func TestFindFractals_OnlyTop(t *testing.T) {
	// 仅有一个顶分型
	input := []Kline{
		kline(10, 8),
		kline(15, 13), // 顶
		kline(12, 10),
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
	klines := []Kline{
		kline(10, 8),  // 0
		kline(15, 13), // 1 → 顶
		kline(12, 10), // 2
		kline(11, 9),  // 3
		kline(11, 9),  // 4 (filler)
		kline(7, 5),   // 5 → 底
		kline(13, 11), // 6
	}
	fractals := []Fractal{
		{Type: TopFractal, Index: 1, High: 15, Low: 13},
		{Type: BottomFractal, Index: 5, Low: 5, High: 7},
	}
	bis := BuildBis(klines, fractals, 1, 0)
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
	klines := []Kline{
		kline(12, 10), // 0
		kline(7, 5),   // 1 → 底
		kline(10, 8),  // 2
		kline(11, 9),  // 3
		kline(11, 9),  // 4 (filler)
		kline(15, 13), // 5 → 顶
		kline(13, 11), // 6
	}
	fractals := []Fractal{
		{Type: BottomFractal, Index: 1, Low: 5, High: 7},
		{Type: TopFractal, Index: 5, High: 15, Low: 13},
	}
	bis := BuildBis(klines, fractals, 1, 0)
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
	klines := []Kline{
		kline(12, 10), // 0
		kline(7, 5),   // 1 → 底
		kline(9, 7),   // 2
		kline(11, 9),  // 3
		kline(11, 9),  // 4 (filler)
		kline(15, 13), // 5 → 顶
		kline(13, 11), // 6
		kline(12, 10), // 7
		kline(12, 10), // 8 (filler)
		kline(8, 6),   // 9 → 底
		kline(14, 12), // 10
	}
	fractals := []Fractal{
		{Type: BottomFractal, Index: 1, Low: 5, High: 7},
		{Type: TopFractal, Index: 5, High: 15, Low: 13},
		{Type: BottomFractal, Index: 9, Low: 6, High: 8},
	}
	bis := BuildBis(klines, fractals, 1, 0)
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
	klines := []Kline{
		kline(10, 8),
		kline(15, 13), // 顶
		kline(12, 10),
		kline(16, 14), // 顶(更高)
		kline(13, 11),
	}
	fractals := []Fractal{
		{Type: TopFractal, Index: 1, High: 15},
		{Type: TopFractal, Index: 3, High: 16},
	}
	bis := BuildBis(klines, fractals, 1, 0)
	if len(bis) != 0 {
		t.Error("expected no bi for same type fractals")
	}
}

func TestBuildBis_InsuffientGap(t *testing.T) {
	// 不足最小 K 线数
	// 底@1, 顶@3: 独立K线=3-1-2=0 < 2
	klines := []Kline{
		kline(10, 8),  // 0
		kline(7, 5),   // 1 → 底
		kline(12, 10), // 2
		kline(15, 13), // 3 → 顶
	}
	fractals := []Fractal{
		{Type: BottomFractal, Index: 1, Low: 5},
		{Type: TopFractal, Index: 3, High: 15},
	}
	bis := BuildBis(klines, fractals, 2, 0)
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
	// ZG = min(Z段s0.Top, s2.Top) = min(15,16) = 15
	// ZD = max(Z段s0.Bottom, s2.Bottom) = max(10,11) = 11
	if pivots[0].ZG != 15 || pivots[0].ZD != 11 {
		t.Errorf("expected ZG=15 ZD=11, got ZG=%.1f ZD=%.1f", pivots[0].ZG, pivots[0].ZD)
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
		{ZG: 13, ZD: 11, GG: 14, DD: 10, StartIndex: 0, EndIndex: 30},
		{ZG: 18, ZD: 15, GG: 19, DD: 15, StartIndex: 35, EndIndex: 60}, // DD(15) > GG(14) → 上涨
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
		{ZG: 15, ZD: 13, GG: 16, DD: 13, StartIndex: 0, EndIndex: 30},
		{ZG: 12, ZD: 10, GG: 12, DD: 9, StartIndex: 35, EndIndex: 60}, // GG(12) < DD(13) → 下跌
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
		hist[i] = 2.0 // 前段MACD面积=22
		macd[i] = 3.0 // 前段DIF极值=3.0
		signal[i] = 1.0
	}
	for i := 11; i <= 20; i++ {
		hist[i] = 0.5 // 后段MACD面积=5.5 < 22 ✓
		macd[i] = 1.0 // 后段DIF极值=1.0 < 3.0 ✓
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

// generateFractalKlines 生成能产生分型的振荡 K 线数据。
func generateFractalKlines(count int) []Kline {
	n := count
	if n < 60 {
		n = 60
	}
	klines := make([]Kline, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range klines {
		phase := float64(i) * 0.4
		base := 100.0 + float64(i)*0.15
		osc := 8.0*math.Sin(phase) + 4.0*math.Sin(phase*2.7)
		mid := base + osc

		klines[i] = Kline{
			Time:       t.Add(time.Duration(i) * time.Hour),
			Open:       mid - 0.5,
			High:       mid + 2.0,
			Low:        mid - 1.5,
			Close:      mid + 0.3,
			BaseVolume: 1000,
		}
	}
	return klines
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

	klines := generateFractalKlines(200)

	result, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("Engine.Process failed: %v", err)
	}

	// 验证所有中间结果
	if len(result.MergedKlines) == 0 {
		t.Error("MergedKlines is empty")
	}
	if len(result.Fractals) < 2 {
		t.Errorf("expected >=2 fractals, got %d", len(result.Fractals))
	}
	if len(result.Bis) < 3 {
		t.Errorf("expected >=3 bis, got %d", len(result.Bis))
	}

	t.Logf("MergedKlines: %d, Fractals: %d, Bis: %d, Segments: %d, Pivots: %d, Trends: %d, Signals: %d, Deviations: %d",
		len(result.MergedKlines), len(result.Fractals), len(result.Bis),
		len(result.Segments), len(result.Pivots), len(result.Trends),
		len(result.Signals), len(result.Deviations))
}

func TestEngine_ShortData_ReturnsError(t *testing.T) {
	engine, err := NewEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	_, err = engine.Process([]Kline{kline(10, 8)})
	if err == nil {
		t.Error("expected error for <3 klines")
	}
}

func TestEngine_Update_Incremental(t *testing.T) {
	engine, err := NewEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// 初始数据
	klines := generateFractalKlines(200)

	result1, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("initial Process failed: %v", err)
	}
	_ = result1

	// 增量更新一根 K 线
	last := klines[len(klines)-1]
	newKline := Kline{
		Time:       last.Time.Add(time.Hour),
		Open:       last.Close - 1,
		High:       last.Close + 3,
		Low:        last.Close - 2,
		Close:      last.Close + 1,
		BaseVolume: 1000,
	}
	result2, err := engine.Update(newKline)
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
	klines := generateFractalKlines(100)

	result, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// 基本验证
	if len(result.MergedKlines) == 0 {
		t.Error("no merged klines")
	}
	t.Logf("MergedKlines: %d, Fractals: %d, Bis: %d, Segments: %d, Pivots: %d, Trends: %d, Signals: %d",
		len(result.MergedKlines), len(result.Fractals), len(result.Bis),
		len(result.Segments), len(result.Pivots), len(result.Trends), len(result.Signals))
}

// ──────────────────────────────────────────────
// v5.0 合规性回归测试
// ──────────────────────────────────────────────

// TestPivot_ZGZD_LockedDuringExtension 验证 ZG/ZD 在中枢延伸时不变。
func TestPivot_ZGZD_LockedDuringExtension(t *testing.T) {
	// 构造至少能形成 1 个中枢并延伸到 5 段的线段序列
	segments := []Segment{
		{Direction: DirDown, StartIndex: 0, EndIndex: 10, Top: 15, Bottom: 10},
		{Direction: DirUp, StartIndex: 10, EndIndex: 20, Top: 14, Bottom: 9},
		{Direction: DirDown, StartIndex: 20, EndIndex: 30, Top: 16, Bottom: 11},
		// 延伸段：在中枢区间 [11, 14] 内震荡
		{Direction: DirUp, StartIndex: 30, EndIndex: 40, Top: 14, Bottom: 10},
		{Direction: DirDown, StartIndex: 40, EndIndex: 50, Top: 13, Bottom: 12},
	}
	pivots := FindPivots(segments)
	if len(pivots) != 1 {
		t.Fatalf("expected 1 pivot, got %d", len(pivots))
	}
	p := pivots[0]

	// ZG/ZD 应在形成时锁定（基于前两个 Z 段 s0+s2），延伸不改变
	// Z 段: s0↓(Top=15,Bot=10), s2↓(Top=16,Bot=11)
	// ZG = min(15, 16) = 15, ZD = max(10, 11) = 11
	expectedZG := 15.0
	expectedZD := 11.0
	if p.ZG != expectedZG || p.ZD != expectedZD {
		t.Errorf("ZG locked at %.1f (want %.1f), ZD locked at %.1f (want %.1f)",
			p.ZG, expectedZG, p.ZD, expectedZD)
	}
	// GG/DD 应随延伸扩展
	if p.GG < 16 || p.DD > 9 {
		t.Errorf("GG=%.1f (expect >= 16), DD=%.1f (expect <= 9)", p.GG, p.DD)
	}
	if p.OverlapCount != 5 {
		t.Errorf("expected 5 overlapping segments, got %d", p.OverlapCount)
	}
	if p.State != PivotExtending {
		t.Errorf("expected PivotExtending, got %d", p.State)
	}
}

// TestPivot_ThirdBuySell_TwoSegment 验证第三类买卖点的两段结构。
func TestPivot_ThirdBuySell_TwoSegment(t *testing.T) {
	// 中枢：Z段=s0↓(12,10), s2↓(14,9) → ZG=min(12,14)=12, ZD=max(10,9)=10
	segments := []Segment{
		{Direction: DirDown, StartIndex: 0, EndIndex: 10, Top: 12, Bottom: 10},
		{Direction: DirUp, StartIndex: 10, EndIndex: 20, Top: 12, Bottom: 8},
		{Direction: DirDown, StartIndex: 20, EndIndex: 30, Top: 14, Bottom: 9},
		// 离开段：向上突破 ZG=12
		{Direction: DirUp, StartIndex: 30, EndIndex: 40, Top: 18, Bottom: 13},
		// 回抽段：向下回试不触及 ZG=12
		{Direction: DirDown, StartIndex: 40, EndIndex: 50, Top: 15, Bottom: 13},
	}
	pivots := FindPivots(segments)
	if len(pivots) != 1 {
		t.Fatalf("expected 1 pivot, got %d", len(pivots))
	}
	p := pivots[0]

	if p.State != PivotDestroyed {
		t.Errorf("expected PivotDestroyed, got %d. ZG=%.1f ZD=%.1f", p.State, p.ZG, p.ZD)
	}
	// 中枢应包含 3 形成段 + 2 破坏段（离开+回抽）= 5 段
	if p.OverlapCount < 5 {
		t.Errorf("expected at least 5 segments (3 form + 2 destroy), got %d", p.OverlapCount)
	}
}

// TestPivot_ThirdBuySell_NotTriggeredWhenPullbackEnters 验证回抽进入中枢时不触发三买。
func TestPivot_ThirdBuySell_NotTriggeredWhenPullbackEnters(t *testing.T) {
	// Z段=s0↓(15,10), s2↓(16,11) → ZG=min(15,16)=15, ZD=max(10,11)=11
	segments := []Segment{
		{Direction: DirDown, StartIndex: 0, EndIndex: 10, Top: 15, Bottom: 10},
		{Direction: DirUp, StartIndex: 10, EndIndex: 20, Top: 14, Bottom: 9},
		{Direction: DirDown, StartIndex: 20, EndIndex: 30, Top: 16, Bottom: 11},
		// 离开段：向上突破 ZG=15
		{Direction: DirUp, StartIndex: 30, EndIndex: 40, Top: 20, Bottom: 16},
		// 回抽段：向下回试但低点=13 < ZG=15 → 进入中枢，不是三买
		{Direction: DirDown, StartIndex: 40, EndIndex: 50, Top: 17, Bottom: 13},
	}
	pivots := FindPivots(segments)
	if len(pivots) != 1 {
		t.Fatalf("expected 1 pivot, got %d", len(pivots))
	}
	// 回抽触及 ZG，中枢不应被破坏
	if pivots[0].State == PivotDestroyed {
		t.Error("pivot should NOT be destroyed when pullback enters ZG zone")
	}
}

// TestSegment_GapBased_Case1_Case2 验证线段破坏的缺口判定（文档 §4.6）。
//
// 向上线段的特征序列 = 向下笔序列，出现顶分型才破坏线段。
// 向下线段的特征序列 = 向上笔序列，出现底分型才破坏线段。
func TestSegment_GapBased_Case1_Case2(t *testing.T) {
	// Case 1: 向上线段，特征序列（向下笔）出现顶分型，第一、二元素无缺口 → 线段立即结束 (BreakStd)
	// 特征序列: s2↓(12,7), s4↓(14,9), s6↓(8,5)
	//   s4 为顶分型: H=14 > max(12,8)=12 ✓  L=9 > max(7,5)=7 ✓
	//   s2-s4 有重叠 (无缺口): s2.High(12) >= s4.Low(9) ✓
	bisCase1 := []MergedBi{
		makeBi(DirUp, 0, 3, 15, 10),   // s1↑
		makeBi(DirDown, 3, 6, 12, 7),  // s2↓ (特征元素1)
		makeBi(DirUp, 6, 9, 16, 11),   // s3↑
		makeBi(DirDown, 9, 12, 14, 9), // s4↓ (特征元素2, 顶分型中间, 与s2有重叠=无缺口)
		makeBi(DirUp, 12, 15, 17, 10), // s5↑
		makeBi(DirDown, 15, 18, 8, 5), // s6↓ (特征元素3)
	}
	segments := BuildSegments(bisCase1)
	if len(segments) == 0 {
		t.Fatal("expected at least 1 segment")
	}
	if len(segments) >= 1 && !segments[0].IsBroken {
		t.Error("Case 1: upward segment should be broken (top fractal in feature seq, no gap)")
	}
	if len(segments) >= 1 && segments[0].BreakType != BreakStd {
		t.Errorf("Case 1: expected BreakStd (1), got %d", segments[0].BreakType)
	}

		// Case 2: 向上线段，特征序列（向下笔）出现顶分型但第一、二元素有缺口 → 需二次确认 (BreakStroke)
		//
		// 理论流程（第67课）：
		//   1. 第一特征序列（向下笔）: s2↓(14,10), s4↓(18,16), s6↓(11,7)
		//      s4 为顶分型中间: H=18>max(14,11)✓, L=16>max(10,7)✓
		//      s2-s4 有缺口: s2.High(14) < s4.Low(16) → gapPending
		//   2. 从 s4 极点开始形成新（试探性）向下线段
		//      新线段的特征序列 = 向上笔: s5↑(25,18), s7↑(20,12), s9↑(23,16)
		//      s7 为底分型中间: L=12<min(18,16)✓, H=20<min(25,23)✓
		//   3. 新特征序列出现底分型 → 确认原线段在 s4 极点处结束
		bisCase2 := []MergedBi{
			makeBi(DirUp, 0, 3, 20, 12),     // s1↑
			makeBi(DirDown, 3, 6, 14, 10),   // s2↓ (特征元素1)
			makeBi(DirUp, 6, 9, 22, 14),     // s3↑
			makeBi(DirDown, 9, 12, 18, 16),  // s4↓ (特征元素2, 顶分型中间, 与s2有缺口)
			makeBi(DirUp, 12, 15, 25, 18),   // s5↑ (新特征序列元素1)
			makeBi(DirDown, 15, 18, 11, 7),  // s6↓ (特征元素3)
			makeBi(DirUp, 18, 21, 20, 12),   // s7↑ (新特征序列元素2, 底分型中间)
			makeBi(DirDown, 21, 24, 13, 9),  // s8↓
			makeBi(DirUp, 24, 27, 23, 16),   // s9↑ (新特征序列元素3)
			makeBi(DirDown, 27, 30, 10, 6),  // s10↓
		}
	segments2 := BuildSegments(bisCase2)
	if len(segments2) == 0 {
		t.Fatal("expected at least 1 segment for Case 2")
	}
	if len(segments2) >= 1 && !segments2[0].IsBroken {
		t.Error("Case 2: upward segment should be broken (gap top fractal + second confirmation)")
	}
	if len(segments2) >= 1 && segments2[0].BreakType != BreakStroke {
		t.Errorf("Case 2: expected BreakStroke (2), got %d", segments2[0].BreakType)
	}
}

// TestPivot_GG_DD_Tracking 验证 GG/DD 随延伸正确更新。
func TestPivot_GG_DD_Tracking(t *testing.T) {
	segments := []Segment{
		{Direction: DirDown, StartIndex: 0, EndIndex: 10, Top: 20, Bottom: 15},
		{Direction: DirUp, StartIndex: 10, EndIndex: 20, Top: 18, Bottom: 12},
		{Direction: DirDown, StartIndex: 20, EndIndex: 30, Top: 22, Bottom: 14},
		// 延伸段：波动更大
		{Direction: DirUp, StartIndex: 30, EndIndex: 40, Top: 25, Bottom: 10},
		{Direction: DirDown, StartIndex: 40, EndIndex: 50, Top: 17, Bottom: 8},
	}
	pivots := FindPivots(segments)
	if len(pivots) != 1 {
		t.Fatalf("expected 1 pivot, got %d", len(pivots))
	}
	p := pivots[0]

	// GG = max(all highs) = 25
	if p.GG != 25 {
		t.Errorf("GG expected 25, got %.1f", p.GG)
	}
	// DD = min(all lows) = 8
	if p.DD != 8 {
		t.Errorf("DD expected 8, got %.1f", p.DD)
	}
	// ZG/ZD should remain based on Z-segments (s0+s2)
	// ZG = min(s0.Top, s2.Top) = min(20,22) = 20, ZD = max(s0.Bottom, s2.Bottom) = max(15,14) = 15
	if p.ZG != 20 || p.ZD != 15 {
		t.Errorf("ZG=%.1f (want 20), ZD=%.1f (want 15)", p.ZG, p.ZD)
	}
}

// TestBiInclusion_DirectionFromNonContained 验证笔包含方向由非包含笔对确定。
func TestBiInclusion_DirectionFromNonContained(t *testing.T) {
	// 构造笔序列，验证包含合并使用正确的方向
	bis := []Bi{
		{Direction: DirUp, StartIndex: 0, EndIndex: 5, High: 20, Low: 10, StartPrice: 10, EndPrice: 20, KLineCount: 6, Length: 10, Slope: 1.67},
		{Direction: DirUp, StartIndex: 5, EndIndex: 10, High: 18, Low: 12, StartPrice: 12, EndPrice: 18, KLineCount: 6, Length: 6, Slope: 1.0},
	}
	merged := MergeBis(bis)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged bi, got %d", len(merged))
	}
	// 笔2 被 笔1 包含，向上关系 → 取高高/高低
	if merged[0].High != 20 {
		t.Errorf("expected merged high=20 (max of 20,18), got %.1f", merged[0].High)
	}
	if merged[0].Low != 12 {
		t.Errorf("expected merged low=12 (max of 10,12), got %.1f", merged[0].Low)
	}
}
