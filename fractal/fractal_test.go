package fractal

import (
	"testing"

	"github.com/bambuo/chan/types"
)

func TestValidateFractals_Empty(t *testing.T) {
	w := ValidateFractals(nil, nil)
	if len(w) != 0 {
		t.Errorf("expected no warnings for empty input, got %d", len(w))
	}
}

func TestValidateFractals_Alternating(t *testing.T) {
	raw := []types.Fractal{
		{Type: types.BottomFractal, Index: 2},
		{Type: types.TopFractal, Index: 5},
		{Type: types.BottomFractal, Index: 8},
		{Type: types.TopFractal, Index: 11},
	}
	w := ValidateFractals(raw, raw)
	for _, s := range w {
		t.Errorf("unexpected warning: %s", s)
	}
}

func TestValidateFractals_Conflict(t *testing.T) {
	raw := []types.Fractal{
		{Type: types.TopFractal, Index: 5},
		{Type: types.BottomFractal, Index: 5}, // 同索引冲突
	}
	w := ValidateFractals(raw, raw)
	hasConflict := false
	for _, s := range w {
		if contains(s, "冲突") {
			hasConflict = true
		}
	}
	if !hasConflict {
		t.Error("expected conflict warning, got none")
	}
}

func TestValidateFractals_NonAlternatingRaw(t *testing.T) {
	// 原始分型中连续两个顶 → ValidateFractals 应报告
	raw := []types.Fractal{
		{Type: types.TopFractal, Index: 3},
		{Type: types.TopFractal, Index: 7},
	}
	w := ValidateFractals(raw, raw)
	hasWarn := false
	for _, s := range w {
		if contains(s, "同向") {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Error("expected non-alternating warning for raw same-type fractals")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
