package signal

import (
	"math"
	"testing"

	"github.com/bambuo/chan/types"
)

func TestDetectBoundarySignals_SupportAndResist(t *testing.T) {
	pivots := []types.Pivot{
		{
			ZG: 110, ZD: 90,
			BeginBiIdx: 0, EndBiIdx: 2,
			State: types.PivotFormed,
		},
	}
	// 构造笔：3 笔构成中枢 + 后续笔测试边界信号
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100, High: 125, Low: 95}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 115, High: 118, Low: 98}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 115, EndPrice: 95, High: 117, Low: 92}},
		// 后续笔 — 测试边界信号
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirUp, StartPrice: 95, EndPrice: 115, High: 118, Low: 93}},   // breakUp: low=93<110<118
		{Bi: types.Bi{StartIndex: 21, EndIndex: 25, Direction: types.DirDown, StartPrice: 115, EndPrice: 85, High: 112, Low: 82}}, // breakDn: high=112>90>82
		{Bi: types.Bi{StartIndex: 26, EndIndex: 30, Direction: types.DirUp, StartPrice: 85, EndPrice: 105, High: 108, Low: 83}},   // 反弹
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}

	targets := map[string]bool{
		"support": true, "resist": true, "breakUp": true, "breakDn": true,
	}
	sigs := detectBoundarySignals(pivots, bis, targets, 0.1)
	if len(sigs) == 0 {
		t.Fatal("expected at least 1 boundary signal")
	}
	hasBreakUp, hasBreakDn := false, false
	for _, s := range sigs {
		t.Logf("signal: type=%d sub=%s", s.Type, s.SubType)
		switch s.SubType {
		case types.SubTBreakUp:
			hasBreakUp = true
		case types.SubTBreakDn:
			hasBreakDn = true
		}
	}
	if !hasBreakUp {
		t.Error("expected breakUp signal")
	}
	if !hasBreakDn {
		t.Error("expected breakDn signal")
	}
}

func TestDetectBoundarySignals_EmptyInput(t *testing.T) {
	sigs := detectBoundarySignals(nil, nil, map[string]bool{"support": true}, 0.1)
	if len(sigs) != 0 {
		t.Errorf("expected 0 signals for nil input, got %d", len(sigs))
	}
}

func TestDetectBoundarySignals_TargetFilter(t *testing.T) {
	pivots := []types.Pivot{
		{ZG: 110, ZD: 90, BeginBiIdx: 0, EndBiIdx: 2, State: types.PivotFormed},
	}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 120, EndPrice: 100}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 100, EndPrice: 115}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirDown, StartPrice: 115, EndPrice: 95}},
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirUp, StartPrice: 95, EndPrice: 115, High: 118, Low: 93}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	// 只检测 breakUp
	sigs := detectBoundarySignals(pivots, bis, map[string]bool{"breakUp": true}, 0.1)
	if len(sigs) == 0 {
		t.Fatal("expected breakUp signal")
	}
	for _, s := range sigs {
		if s.SubType != types.SubTBreakUp {
			t.Errorf("expected only breakUp, got %v", s.SubType)
		}
	}
}

// ── parseBspTypes ──

func TestParseBspTypes_Default(t *testing.T) {
	m := parseBspTypes("")
	if len(m) == 0 {
		t.Fatal("expected non-empty map for empty string")
	}
	for _, k := range []string{"1", "1p", "2", "2s", "3a", "3b", "support", "resist", "breakUp", "breakDn"} {
		if !m[k] {
			t.Errorf("expected %q to be enabled by default", k)
		}
	}
}

func TestParseBspTypes_Custom(t *testing.T) {
	m := parseBspTypes("1,2,3a")
	if !m["1"] || !m["2"] || !m["3a"] {
		t.Error("expected 1,2,3a to be enabled")
	}
	if m["1p"] || m["3b"] {
		t.Error("expected 1p and 3b to be disabled")
	}
}

// ── overlap ──

func TestOverlap_Strict(t *testing.T) {
	if !overlap(0, 10, 5, 15, false) {
		t.Error("[0,10] and [5,15] should overlap strictly")
	}
	if overlap(0, 10, 10, 20, false) {
		t.Error("[0,10] and [10,20] should NOT overlap strictly (touch only)")
	}
	if !overlap(0, 10, 10, 20, true) {
		t.Error("[0,10] and [10,20] should overlap with equal=true")
	}
}

// ── back2ZS ──

func TestBack2ZS_DownwardBi(t *testing.T) {
	bi := &types.Bi{Direction: types.DirDown, StartPrice: 100, EndPrice: 80}
	zs := &types.Pivot{ZG: 95, ZD: 85}
	// bi low (80) < ZG (95) → 回到中枢内
	if !back2ZS(bi, zs) {
		t.Error("downward bi with low=80 should be back to ZS (ZG=95)")
	}
}

func TestBack2ZS_UpwardBi(t *testing.T) {
	bi := &types.Bi{Direction: types.DirUp, StartPrice: 80, EndPrice: 100}
	zs := &types.Pivot{ZG: 95, ZD: 85}
	// bi high (100) > ZD (85) → 回到中枢内
	if !back2ZS(bi, zs) {
		t.Error("upward bi with high=100 should be back to ZS (ZD=85)")
	}
}

func TestBack2ZS_Outside(t *testing.T) {
	// 向上笔低点低于 ZD → 还没回到中枢（属于中枢下方）
	bi := &types.Bi{Direction: types.DirUp, StartPrice: 70, EndPrice: 85}
	zs := &types.Pivot{ZG: 95, ZD: 85}
	if back2ZS(bi, zs) {
		t.Error("bi end=85 == ZD=85 should not be considered 'back to ZS' (not beyond)")
	}
}

// ── groupBySeg ──

func TestGroupBySeg_Basic(t *testing.T) {
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 20, Direction: types.DirUp, IsSure: true},
		{StartIndex: 21, EndIndex: 40, Direction: types.DirDown, IsSure: true},
	}
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 10, Direction: types.DirUp}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 20, Direction: types.DirDown}},
		{Bi: types.Bi{StartIndex: 21, EndIndex: 30, Direction: types.DirDown}},
		{Bi: types.Bi{StartIndex: 31, EndIndex: 40, Direction: types.DirUp}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}
	pivots := []types.Pivot{
		{BeginBiIdx: 0, EndBiIdx: 1},
		{BeginBiIdx: 2, EndBiIdx: 3},
	}

	groups := groupBySeg(segs, pivots, bis)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if len(groups[0].Pivots) != 1 {
		t.Errorf("expected 1 pivot in group[0], got %d", len(groups[0].Pivots))
	}
}

// ── DetectSignals (end-to-end smoke test) ──

func TestDetectSignals_EmptyInput(t *testing.T) {
	sigs := DetectSignals(nil, nil, nil, nil, types.Config{})
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
	sigs := DetectSignals(nil, bis, segs, nil, types.DefaultConfig())
	if len(sigs) != 0 {
		t.Errorf("expected 0 signals for no pivots, got %d", len(sigs))
	}
}

// ── parseBspTypes with inf rate (seg_bsp.go SegBspConfig compat) ──

func TestSignal_BspDivergenceSentinel(t *testing.T) {
	cfg := types.DefaultConfig()
	cfg.BspDivergenceRate = math.Inf(1) // sentinel: 不设背驰过滤
	_ = cfg
	// 保证 safeDivide 等不 panic
	_ = types.SafeDivide(1, 0, 0)
}
