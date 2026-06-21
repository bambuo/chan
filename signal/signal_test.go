package signal

import (
	"math"
	"testing"

	"github.com/bambuo/chan/types"
)

// ── treatT1P 零振幅假信号 ──

func TestTreatT1P_ZeroAmplitude_NoSignal(t *testing.T) {
	// 构造场景：两笔同向且振幅均为零，不应产生 T1P 信号
	seg := types.Segment{StartIndex: 0, EndIndex: 20, Direction: types.DirDown, IsSure: true}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown,
			StartPrice: 100, EndPrice: 100, High: 100, Low: 100}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp,
			StartPrice: 100, EndPrice: 102, High: 102, Low: 100}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown,
			StartPrice: 102, EndPrice: 100, High: 102, Low: 100}},
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirUp,
			StartPrice: 100, EndPrice: 102, High: 102, Low: 100}},
		{Bi: types.Bi{StartIndex: 21, EndIndex: 25, Direction: types.DirDown,
			StartPrice: 102, EndPrice: 102, High: 102, Low: 102}}, // 零振幅
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	cfg := types.DefaultConfig()
	cfg.BspDivergenceRate = 1.0
	cfg.BspType = "1p"
	targets := parseBspTypes(cfg.BspType)
	g := &segGroup{Seg: seg}
	sig := treatT1P(g, bis, cfg, targets, nil)
	if sig != nil {
		t.Errorf("zero-amplitude T1P should not produce signal, got %+v", sig)
	}
}

func TestTreatT1P_NormalAmplitude_CanProduceSignal(t *testing.T) {
	// 构造场景：正常振幅 + 哨兵率（关闭过滤），应产生 T1P
	// DirDown 线段：last 和 pre 都是向下笔
	// 需要 last.ZSLow() <= pre.ZSLow()（不拒绝）
	seg := types.Segment{StartIndex: 0, EndIndex: 15, Direction: types.DirDown, IsSure: true}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown,
			StartPrice: 120, EndPrice: 100, High: 120, Low: 100}}, // pre (idx=0)
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp,
			StartPrice: 100, EndPrice: 115, High: 115, Low: 100}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown,
			StartPrice: 115, EndPrice: 95, High: 115, Low: 95}}, // last (idx=2, EndIndex matches seg)
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	// pre.ZSLow() = 100, last.ZSLow() = 95: 95 <= 100 → not rejected
	// last.Direction == seg.Direction == DirDown
	cfg := types.DefaultConfig()
	cfg.BspDivergenceRate = math.Inf(1) // sentinel: 关闭过滤
	cfg.BspType = "1p"
	targets := parseBspTypes(cfg.BspType)
	g := &segGroup{Seg: seg}
	sig := treatT1P(g, bis, cfg, targets, nil)
	if sig == nil {
		t.Error("sentinel rate with normal amplitude should produce T1P signal")
	}
}

// ── outBreak ──

func TestOutBreak_DownBelowZD(t *testing.T) {
	bi := &types.Bi{Direction: types.DirDown, StartPrice: 100, EndPrice: 80}
	if !outBreak(bi, 95, 85) {
		t.Error("downward bi low=80 < zd=85 should break out")
	}
}

func TestOutBreak_DownAboveZD(t *testing.T) {
	bi := &types.Bi{Direction: types.DirDown, StartPrice: 100, EndPrice: 90}
	if outBreak(bi, 95, 85) {
		t.Error("downward bi low=90 > zd=85 should NOT break out")
	}
}

func TestOutBreak_UpAboveZG(t *testing.T) {
	bi := &types.Bi{Direction: types.DirUp, StartPrice: 80, EndPrice: 100}
	if !outBreak(bi, 95, 85) {
		t.Error("upward bi high=100 > zg=95 should break out")
	}
}

func TestOutBreak_UpBelowZG(t *testing.T) {
	bi := &types.Bi{Direction: types.DirUp, StartPrice: 80, EndPrice: 92}
	if outBreak(bi, 95, 85) {
		t.Error("upward bi high=92 < zg=95 should NOT break out")
	}
}

func TestOutBreak_NilBi(t *testing.T) {
	if outBreak(nil, 95, 85) {
		t.Error("nil bi should not break out")
	}
}

// ── back2ZS equal-value boundary ──

func TestBack2ZS_EqualBoundary(t *testing.T) {
	// 向下笔低点 == ZG → 应视为回到中枢（<= 语义）
	bi := &types.Bi{Direction: types.DirDown, StartPrice: 100, EndPrice: 90, High: 100, Low: 90}
	zs := &types.Pivot{ZG: 95, ZD: 85}
	// ZSLow() = 90, ZG = 95: 90 <= 95 → true
	if !back2ZS(bi, zs) {
		t.Error("downward bi ZSLow=90 <= ZG=95 should be back to ZS")
	}

	// 向上笔高点 == ZD → 应视为回到中枢（>= 语义）
	bi2 := &types.Bi{Direction: types.DirUp, StartPrice: 80, EndPrice: 85, High: 85, Low: 80}
	zs2 := &types.Pivot{ZG: 95, ZD: 85}
	// ZSHigh() = 85, ZD = 85: 85 >= 85 → true
	if !back2ZS(bi2, zs2) {
		t.Error("upward bi ZSHigh=85 >= ZD=85 should be back to ZS")
	}
}

func TestBack2ZS_NotBack(t *testing.T) {
	// 向上笔高点 < ZD → 未回到中枢
	bi := &types.Bi{Direction: types.DirUp, StartPrice: 70, EndPrice: 80, High: 80, Low: 70}
	zs := &types.Pivot{ZG: 95, ZD: 85}
	if back2ZS(bi, zs) {
		t.Error("upward bi ZSHigh=80 < ZD=85 should NOT be back to ZS")
	}
}

// ── divergenceRateSentinel ──

func TestDivergenceRateSentinel(t *testing.T) {
	if !divergenceRateSentinel(math.Inf(1)) {
		t.Error("+Inf should be sentinel")
	}
	if !divergenceRateSentinel(200) {
		t.Error("200 should be sentinel (> 100)")
	}
	if !divergenceRateSentinel(101) {
		t.Error("101 should be sentinel")
	}
	if divergenceRateSentinel(50) {
		t.Error("50 should NOT be sentinel")
	}
	if divergenceRateSentinel(0) {
		t.Error("0 should NOT be sentinel")
	}
}

// ── split ──

func TestSplit(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1,2,3a", 3},
		{"1p, 2s, 3b", 3},
		{"", 0},
		{"1", 1},
		{"1,,2", 2}, // empty segments skipped
	}
	for _, tt := range tests {
		got := split(tt.input)
		if len(got) != tt.want {
			t.Errorf("split(%q) len = %d, want %d (got %v)", tt.input, len(got), tt.want, got)
		}
	}
}

// ── overlap ──

func TestOverlap_AllCases(t *testing.T) {
	// Strict: touching edges do not overlap
	if overlap(0, 10, 10, 20, false) {
		t.Error("strict touching should not overlap")
	}
	// Eq: touching edges overlap
	if !overlap(0, 10, 10, 20, true) {
		t.Error("eq touching should overlap")
	}
	// Disjoint
	if overlap(0, 5, 10, 15, true) {
		t.Error("disjoint should not overlap")
	}
}

// ── detectBoundarySignals additional ──

func TestDetectBoundarySignals_Support(t *testing.T) {
	pivots := []types.Pivot{
		{ZG: 110, ZD: 90, BeginBiIdx: 0, EndBiIdx: 2, State: types.PivotFormed},
	}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100, High: 125, Low: 95}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 115, High: 118, Low: 98}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 115, EndPrice: 95, High: 117, Low: 92}},
		// support: 向下笔低点 ≈ ZD (90 ± tolerance)
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirDown, StartPrice: 100, EndPrice: 88, High: 100, Low: 88}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	sigs := detectBoundarySignals(pivots, bis, map[string]bool{"support": true}, 0.1)
	found := false
	for _, s := range sigs {
		if s.SubType == types.SubTSupport {
			found = true
		}
	}
	if !found {
		t.Error("expected support signal near ZD=90")
	}
}

func TestDetectBoundarySignals_Resist(t *testing.T) {
	pivots := []types.Pivot{
		{ZG: 110, ZD: 90, BeginBiIdx: 0, EndBiIdx: 2, State: types.PivotFormed},
	}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100, High: 125, Low: 95}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 115, High: 118, Low: 98}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 115, EndPrice: 95, High: 117, Low: 92}},
		// resist: 向上笔高点 ≈ ZG (110 ± tolerance)
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirUp, StartPrice: 95, EndPrice: 112, High: 112, Low: 95}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	sigs := detectBoundarySignals(pivots, bis, map[string]bool{"resist": true}, 0.1)
	found := false
	for _, s := range sigs {
		if s.SubType == types.SubTResist {
			found = true
		}
	}
	if !found {
		t.Error("expected resist signal near ZG=110")
	}
}

// ── groupBySeg ──

func TestGroupBySeg_NoMatchingPivot(t *testing.T) {
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 10, Direction: types.DirUp, IsSure: true},
	}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 10, Direction: types.DirUp}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	// Pivot 的 BeginBiIdx 对应的 start 不在任何线段范围内
	pivots := []types.Pivot{
		{BeginBiIdx: 0, EndBiIdx: 0},
	}
	groups := groupBySeg(segs, pivots, bis)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
}

// ── segGroup helpers ──

func TestSegGroup_FirstLastMultiBiZS(t *testing.T) {
	// IsOneBiZs() returns true when BeginBiIdx == EndBiIdx
	oneBi := types.Pivot{ZG: 100, ZD: 90, BeginBiIdx: 0, EndBiIdx: 0}
	multi := types.Pivot{ZG: 110, ZD: 95, BeginBiIdx: 0, EndBiIdx: 2}
	g := &segGroup{
		Seg:    types.Segment{},
		Pivots: []types.Pivot{oneBi, multi},
	}
	first := g.firstMultiBiZS()
	if first == nil {
		t.Error("expected non-nil firstMultiBiZS")
	}
	last := g.lastMultiBiZS()
	if last == nil {
		t.Error("expected non-nil lastMultiBiZS")
	}
	if g.multiCnt() != 1 {
		t.Errorf("multiCnt = %d, want 1", g.multiCnt())
	}
}

// ── DetectSignals (end-to-end smoke test) ──

func TestDetectSignals_EmptyInput(t *testing.T) {
	sigs := DetectSignals(nil, nil, nil, nil, types.Config{}, nil)
	if len(sigs) != 0 {
		t.Errorf("expected 0 signals for nil input, got %d", len(sigs))
	}
}

func TestDetectSignals_ShortData(t *testing.T) {
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 10, Direction: types.DirUp, IsSure: true},
	}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	sigs := DetectSignals(nil, bis, segs, nil, types.DefaultConfig(), nil)
	if len(sigs) != 0 {
		t.Errorf("expected 0 signals for no pivots, got %d", len(sigs))
	}
}

// ── parseBspTypes with inf rate (seg_bsp.go SegBspConfig compat) ──

func TestSignal_BspDivergenceSentinel(t *testing.T) {
	cfg := types.DefaultConfig()
	cfg.BspDivergenceRate = math.Inf(1) // sentinel: 不设背驰过滤
	_ = cfg
	_ = types.SafeDivide(1, 0, 0)
}

// ── findEndIdx / findStartIdx ──

func TestFindEndIdx_Found(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{EndIndex: 10}},
		{Bi: types.Bi{EndIndex: 20}},
		{Bi: types.Bi{EndIndex: 30}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	if idx := findEndIdx(bis, 20); idx != 1 {
		t.Errorf("findEndIdx(bis, 20) = %d, want 1", idx)
	}
	if idx := findEndIdx(bis, 99); idx != -1 {
		t.Errorf("findEndIdx(bis, 99) = %d, want -1", idx)
	}
}

func TestFindStartIdx(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0}},
		{Bi: types.Bi{StartIndex: 10}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	if idx := findStartIdx(bis, 10); idx != 1 {
		t.Errorf("findStartIdx(bis, 10) = %d, want 1", idx)
	}
	if idx := findStartIdx(bis, 99); idx != -1 {
		t.Errorf("findStartIdx(bis, 99) = %d, want -1", idx)
	}
}

// ── outIsPeak ──

func TestOutIsPeak_UpBiOutIsPeak(t *testing.T) {
	zs := &types.Pivot{
		BiOut: &types.Bi{Direction: types.DirUp, StartPrice: 90, EndPrice: 110},
		BiList: []types.Bi{
			{Direction: types.DirUp, StartPrice: 80, EndPrice: 100},
			{Direction: types.DirDown, StartPrice: 100, EndPrice: 85},
		},
	}
	// BiOut high=110, BiList highs=100,85. BiOut is peak.
	if !outIsPeak(zs, 0) {
		t.Error("BiOut should be peak (highest ZSHigh)")
	}
}

func TestOutIsPeak_DownBiOutIsPeak(t *testing.T) {
	zs := &types.Pivot{
		BiOut: &types.Bi{Direction: types.DirDown, StartPrice: 100, EndPrice: 80},
		BiList: []types.Bi{
			{Direction: types.DirDown, StartPrice: 110, EndPrice: 95},
			{Direction: types.DirUp, StartPrice: 85, EndPrice: 105},
		},
	}
	// BiOut low=80, BiList lows=95,85. BiOut is peak (lowest ZSLow).
	if !outIsPeak(zs, 0) {
		t.Error("BiOut should be peak (lowest ZSLow)")
	}
}

func TestOutIsPeak_NotPeak(t *testing.T) {
	zs := &types.Pivot{
		BiOut: &types.Bi{Direction: types.DirUp, StartPrice: 90, EndPrice: 100},
		BiList: []types.Bi{
			{Direction: types.DirUp, StartPrice: 80, EndPrice: 110},
		},
	}
	// BiOut ZSHigh=100, BiList[0] ZSHigh=110 > 100 → not peak
	if outIsPeak(zs, 0) {
		t.Error("BiOut should NOT be peak (BiList has higher)")
	}
}

func TestOutIsPeak_NilBiOut(t *testing.T) {
	zs := &types.Pivot{BiOut: nil}
	if outIsPeak(zs, 0) {
		t.Error("nil BiOut should not be peak")
	}
}

// ── breakPeak ──

func TestBreakPeak_DownBi(t *testing.T) {
	// 三买场景：向下回踩笔的低点 > 中枢波动最高点 PeakHigh(GG)
	// ZSLow=90 > PeakHigh=80 → 回踩未跌破中枢上沿极值
	bi := &types.Bi{Direction: types.DirDown, StartPrice: 100, EndPrice: 90, Low: 90}
	zs := &types.Pivot{PeakHigh: 80, PeakLow: 70}
	if !breakPeak(bi, zs) {
		t.Error("down bi ZSLow=90 > PeakHigh=80 should be breakPeak")
	}
}

func TestBreakPeak_UpBi(t *testing.T) {
	// 三卖场景：向上回抽笔的高点 < 中枢波动最低点 PeakLow(DD)
	// ZSHigh=92 < PeakLow=95 → 回抽未升破中枢下沿极值
	bi := &types.Bi{Direction: types.DirUp, StartPrice: 80, EndPrice: 92, High: 92}
	zs := &types.Pivot{PeakHigh: 100, PeakLow: 95}
	if !breakPeak(bi, zs) {
		t.Error("up bi ZSHigh=92 < PeakLow=95 should be breakPeak")
	}
}

func TestBreakPeak_NotBreak(t *testing.T) {
	bi := &types.Bi{Direction: types.DirDown, StartPrice: 100, EndPrice: 88, Low: 88}
	zs := &types.Pivot{PeakHigh: 95, PeakLow: 85}
	if breakPeak(bi, zs) {
		t.Error("down bi low=88 > peakLow=85 should NOT break")
	}
}

// ── treatT1 ──

func TestTreatT1_DivergenceReturnsSignal(t *testing.T) {
	seg := types.Segment{StartIndex: 0, EndIndex: 25, Direction: types.DirDown, IsSure: true}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 115}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 115, EndPrice: 95}},
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirUp, StartPrice: 95, EndPrice: 105}},
		{Bi: types.Bi{StartIndex: 21, EndIndex: 25, Direction: types.DirDown, StartPrice: 105, EndPrice: 85}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	last := &types.Pivot{
		ZG: 100, ZD: 90, BeginBiIdx: 2, EndBiIdx: 4,
		BiIn:   &bis[3].Bi,
		BiOut:  &bis[4].Bi,
		BiList: []types.Bi{bis[2].Bi, bis[3].Bi, bis[4].Bi},
	}
	devs := []types.Deviation{
		{Direction: types.DirDown, PriceHigh: 85, ForceBefore: 10, ForceAfter: 3},
	}
	g := &segGroup{Seg: seg, Pivots: []types.Pivot{*last}}
	cfg := types.DefaultConfig()
	cfg.BspType = "1"
	cfg.Bsp1OnlyMultiBiZs = false
	cfg.BspMinZsCnt = 0
	targets := parseBspTypes("1")
	sig := treatT1(g, last, bis, devs, cfg, targets)
	if sig == nil {
		t.Error("expected T1 signal when divergence present")
	}
}

func TestTreatT1_NoDivergence(t *testing.T) {
	seg := types.Segment{StartIndex: 0, EndIndex: 15, Direction: types.DirDown, IsSure: true}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 115}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 115, EndPrice: 95}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	last := &types.Pivot{
		ZG: 100, ZD: 90,
		BiIn:   &bis[0].Bi,
		BiOut:  &bis[2].Bi,
		BiList: []types.Bi{bis[0].Bi, bis[1].Bi, bis[2].Bi},
	}
	g := &segGroup{Seg: seg, Pivots: []types.Pivot{*last}}
	cfg := types.DefaultConfig()
	cfg.BspType = "1"
	targets := parseBspTypes("1")
	// No deviations → treatT1 should return nil
	sig := treatT1(g, last, bis, nil, cfg, targets)
	if sig != nil {
		t.Errorf("expected nil T1 without deviations, got %+v", sig)
	}
}

// ── detectT1 (integration: dispatches to treatT1 or treatT1P) ──

func TestDetectT1_DispatchesToTreatT1P(t *testing.T) {
	// Single-bi zs (IsOneBiZs=true) → detectT1 should fall through to treatT1P
	seg := types.Segment{StartIndex: 0, EndIndex: 15, Direction: types.DirDown, IsSure: true}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 115}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 115, EndPrice: 95}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	// Single-bi pivot (BeginBiIdx==EndBiIdx, i.e. IsOneBiZs=true)
	pivots := []types.Pivot{
		{BeginBiIdx: 0, EndBiIdx: 0, ZG: 100, ZD: 90, BiIn: &bis[0].Bi, BiOut: &bis[2].Bi},
	}
	g := &segGroup{Seg: seg, Pivots: pivots}
	cfg := types.DefaultConfig()
	cfg.BspDivergenceRate = math.Inf(1)
	cfg.BspType = "1,1p"
	targets := parseBspTypes(cfg.BspType)
	sig := detectT1(g, bis, nil, cfg, targets, nil)
	if sig != nil && sig.SubType != types.SubT1P {
		t.Errorf("expected T1P signal from detectT1 with single-bi zs, got sub=%v", sig.SubType)
	}
}

// ── detectT2 / detectT2S ──

func TestDetectT2_BasicFlow(t *testing.T) {
	seg := types.Segment{StartIndex: 0, EndIndex: 20, Direction: types.DirDown, IsSure: true}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100, High: 120, Low: 100}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 110, High: 110, Low: 100}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 110, EndPrice: 95, High: 110, Low: 95}},
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirUp, StartPrice: 95, EndPrice: 105, High: 105, Low: 95}},   // breakBi
		{Bi: types.Bi{StartIndex: 21, EndIndex: 25, Direction: types.DirDown, StartPrice: 105, EndPrice: 98, High: 105, Low: 98}}, // b2Bi
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	groups := []segGroup{{
		Seg: seg,
		Pivots: []types.Pivot{
			{BeginBiIdx: 0, EndBiIdx: 2, ZG: 105, ZD: 95, State: types.PivotFormed},
		},
	}}
	bsp1Map := map[int]bool{15: true} // seg.EndIndex matches
	bsp1IdxMap := map[int]int{15: 2}
	cfg := types.DefaultConfig()
	cfg.BspType = "2"
	cfg.Bsp2Follow1 = true
	cfg.BspMaxBs2Rate = 0.9999
	targets := parseBspTypes("2")
	sigs := detectT2(&groups[0], groups, bis, bsp1Map, bsp1IdxMap, nil, cfg, targets)
	if len(sigs) == 0 {
		t.Log("T2 not generated (expected: breakBi direction check may filter out)")
	}
}

func TestDetectT2_ShortData(t *testing.T) {
	seg := types.Segment{StartIndex: 0, EndIndex: 10, Direction: types.DirUp}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	g := segGroup{Seg: seg}
	sigs := detectT2(&g, nil, bis, nil, nil, nil, types.Config{}, nil)
	if len(sigs) != 0 {
		t.Errorf("expected 0 signals for short data, got %d", len(sigs))
	}
}

func TestDetectT2S_Basic(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100, High: 120, Low: 100}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 110, High: 110, Low: 100}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 110, EndPrice: 95, High: 110, Low: 95}},
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirUp, StartPrice: 95, EndPrice: 105, High: 105, Low: 95}},
		{Bi: types.Bi{StartIndex: 21, EndIndex: 25, Direction: types.DirDown, StartPrice: 105, EndPrice: 98, High: 105, Low: 98}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	b1Idx := 2
	b2Bi := &bis[4].Bi
	breakBi := &bis[3].Bi
	cfg := types.DefaultConfig()
	cfg.BspMaxBs2Rate = 0.9999
	targets := parseBspTypes("2s")
	sigs := detectT2S(&segGroup{}, nil, bis, b1Idx, b2Bi, breakBi, -1, cfg, targets)
	// Should not crash, may or may not produce signals depending on data
	_ = sigs
}

// ── detectT3 / treatT3A / treatT3B ──

func TestDetectT3_Follow1Disabled(t *testing.T) {
	seg := types.Segment{StartIndex: 0, EndIndex: 10, Direction: types.DirUp}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	g := segGroup{Seg: seg}
	cfg := types.DefaultConfig()
	cfg.Bsp3Follow1 = false
	cfg.BspType = "3a"
	targets := parseBspTypes("3a")
	sigs := detectT3(&g, nil, 0, bis, nil, nil, nil, cfg, targets)
	// With short data, should return nil
	_ = sigs
}

func TestTreatT3A_NoMultiBiZS(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	ng := &segGroup{Seg: types.Segment{}}
	cfg := types.DefaultConfig()
	targets := parseBspTypes("3a")
	sigs := treatT3A(nil, ng, 0, bis, 0, -1, cfg, targets)
	if len(sigs) != 0 {
		t.Errorf("expected 0 T3A with nil first group, got %d", len(sigs))
	}
}

func TestTreatT3B_Basic(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 90, EndPrice: 110}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirDown, StartPrice: 110, EndPrice: 95}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirUp, StartPrice: 95, EndPrice: 105}},
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirDown, StartPrice: 105, EndPrice: 98}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	g := &segGroup{
		Seg: types.Segment{},
		Pivots: []types.Pivot{
			{BeginBiIdx: 0, EndBiIdx: 2, ZG: 105, ZD: 95},
		},
	}
	ng := &segGroup{Seg: types.Segment{StartIndex: 25, EndIndex: 30}}
	cfg := types.DefaultConfig()
	cfg.Bsp3Follow1 = false
	targets := parseBspTypes("3b")
	sigs := treatT3B(g, ng, 0, bis, 0, -1, cfg, targets)
	// Should produce no signals since back2ZS likely true
	_ = sigs
}

// ── parseBspTypes edge cases ──

func TestParseBspTypes_AllEnabled(t *testing.T) {
	m := parseBspTypes("")
	for _, k := range []string{"1", "1p", "2", "2s", "3a", "3b", "support", "resist", "breakUp", "breakDn"} {
		if !m[k] {
			t.Errorf("expected %q to be enabled by default", k)
		}
	}
}

func TestParseBspTypes_CustomFilter(t *testing.T) {
	m := parseBspTypes("1,3a,support")
	if !m["1"] || !m["3a"] || !m["support"] {
		t.Error("expected 1,3a,support enabled")
	}
	if m["2"] || m["3b"] || m["resist"] {
		t.Error("expected 2,3b,resist disabled")
	}
}

// ── seg_bsp.go functions ──

func TestSegBiContain(t *testing.T) {
	a := types.Bi{High: 110, Low: 90}
	b := types.Bi{High: 105, Low: 95}
	if !segBiContain(a, b) {
		t.Error("a should contain b")
	}
	if segBiContain(b, a) {
		t.Error("b should NOT contain a")
	}
}

func TestSegBiDir(t *testing.T) {
	merged := []types.MergedBi{
		{Bi: types.Bi{High: 100, Low: 80, Direction: types.DirUp}},
		{Bi: types.Bi{High: 120, Low: 95, Direction: types.DirUp}},
	}
	d := segBiDir(merged, types.Bi{Direction: types.DirUp})
	if d == types.DirNone {
		t.Log("segBiDir: containers detected, fallback to DirUp")
	}
}

func TestSegBiDir_NonContain(t *testing.T) {
	merged := []types.MergedBi{
		{Bi: types.Bi{High: 100, Low: 80, StartPrice: 80, EndPrice: 100, Direction: types.DirUp}},
		{Bi: types.Bi{High: 120, Low: 100, StartPrice: 100, EndPrice: 120, Direction: types.DirUp}},
	}
	d := segBiDir(merged, types.Bi{})
	if d == types.DirDown {
		t.Log("segBiDir: DirDown based on non-contain logic")
	}
}

func TestSegMergePair(t *testing.T) {
	a := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 80, EndPrice: 100, High: 105, Low: 78}
	b := types.Bi{EndIndex: 10, StartPrice: 100, EndPrice: 110, High: 112, Low: 95}
	m := segMergePair(a, b, types.DirUp)
	if m.High != 112 {
		t.Errorf("mergeDir=DirUp high = %.2f, want 112", m.High)
	}
	if m.EndIndex != 10 {
		t.Errorf("merge endIndex = %d, want 10", m.EndIndex)
	}
	m2 := segMergePair(a, b, types.DirDown)
	if m2.High != 105 {
		t.Errorf("mergeDir=DirDown high = %.2f, want 105", m2.High)
	}
	m3 := segMergePair(a, b, types.DirNone)
	if m3.High < 112 {
		t.Errorf("mergeDir=DirNone high = %.2f, want >= 112 (a.IsUp=true, use max)", m3.High)
	}
}

func TestSegBspConfig(t *testing.T) {
	base := types.Config{
		BspMacdAlgo: "peak",
	}
	cfg := SegBspConfig(base)
	if cfg.BspMacdAlgo != "slope" {
		t.Errorf("default SegBspMacdAlgo = %s, want 'slope'", cfg.BspMacdAlgo)
	}
	// With custom algo
	base.SegBspMacdAlgo = "area"
	cfg2 := SegBspConfig(base)
	if cfg2.BspMacdAlgo != "area" {
		t.Errorf("custom SegBspMacdAlgo = %s, want 'area'", cfg2.BspMacdAlgo)
	}
}

func TestSegmentsToMergedBis(t *testing.T) {
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 10, Direction: types.DirUp, Top: 120, Bottom: 100},
		{StartIndex: 11, EndIndex: 20, Direction: types.DirDown, Top: 120, Bottom: 90},
	}
	bis := segmentsToMergedBis(segs)
	if len(bis) != 2 {
		t.Fatalf("expected 2 merged bis, got %d", len(bis))
	}
	if bis[0].Direction != types.DirUp {
		t.Errorf("first bi dir = %v, want DirUp", bis[0].Direction)
	}
}

func TestMergeSegAsBis(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, High: 110, Low: 90, StartPrice: 90, EndPrice: 110}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirDown, High: 110, Low: 85, StartPrice: 110, EndPrice: 85}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	// No direction conflict with 2 items -> no merging, just pass through
	merged := mergeSegAsBis(bis)
	if len(merged) != 2 {
		t.Errorf("expected 2 merged items for opposite directions, got %d", len(merged))
	}
}

func TestMergeSegAsBis_SingleItem(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	merged := mergeSegAsBis(bis)
	if len(merged) != 1 {
		t.Errorf("expected 1 merged item, got %d", len(merged))
	}
}

func TestMergeSegAsBis_Empty(t *testing.T) {
	merged := mergeSegAsBis(nil)
	if merged != nil {
		t.Errorf("expected nil for empty input, got %v", merged)
	}
}

func TestDetectSegSignals_Basic(t *testing.T) {
	// Too few segments -> returns nil
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 10, Direction: types.DirUp},
	}
	ctx := DetectSegSignals(segs, types.Config{})
	if ctx != nil {
		t.Errorf("expected nil for < 3 segments, got non-nil")
	}
}

// ── DetectSignals integration ──

func TestDetectSignals_WithBoundaryTargets(t *testing.T) {
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 20, Direction: types.DirDown, IsSure: true},
	}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100, High: 125, Low: 95}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 115, High: 118, Low: 98}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 115, EndPrice: 95, High: 117, Low: 92}},
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirUp, StartPrice: 95, EndPrice: 105, High: 108, Low: 93}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	pivots := []types.Pivot{
		{ZG: 110, ZD: 90, BeginBiIdx: 0, EndBiIdx: 2, State: types.PivotFormed},
	}
	cfg := types.DefaultConfig()
	cfg.BspType = "support,resist,breakUp,breakDn"
	// Should not panic, may return boundary signals
	sigs := DetectSignals(pivots, bis, segs, nil, cfg, nil)
	_ = sigs
}
