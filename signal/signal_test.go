package signal

import (
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
	sigs := detectBoundarySignals(pivots, bis, targets)
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
	sigs := detectBoundarySignals(nil, nil, map[string]bool{"support": true})
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
	sigs := detectBoundarySignals(pivots, bis, map[string]bool{"breakUp": true})
	if len(sigs) == 0 {
		t.Fatal("expected breakUp signal")
	}
	for _, s := range sigs {
		if s.SubType != types.SubTBreakUp {
			t.Errorf("expected only breakUp, got %v", s.SubType)
		}
	}
}
