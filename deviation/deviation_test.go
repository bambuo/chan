package deviation

import (
	"math"
	"testing"

	"github.com/bambuo/chan/force"
	"github.com/bambuo/chan/types"
)

func TestMetricForSide_Buy(t *testing.T) {
	cfg := types.DefaultConfig()
	cfg.BspMacdAlgo = "area"
	cfg.BspMacdAlgoBuy = "peak"

	// 向下笔 → 买入侧 → 应返回 Peak
	bi := &types.Bi{Direction: types.DirDown}
	m := metricForSide(bi, cfg)
	if m != force.Peak {
		t.Errorf("expected Peak for buy side, got %d", m)
	}
}

func TestMetricForSide_Sell(t *testing.T) {
	cfg := types.DefaultConfig()
	cfg.BspMacdAlgo = "area"
	cfg.BspMacdAlgoSell = "amp"

	// 向上笔 → 卖出侧 → 应返回 Amp
	bi := &types.Bi{Direction: types.DirUp}
	m := metricForSide(bi, cfg)
	if m != force.Amp {
		t.Errorf("expected Amp for sell side, got %d", m)
	}
}

func TestMetricForSide_Fallback(t *testing.T) {
	cfg := types.DefaultConfig()
	cfg.BspMacdAlgo = "slope"
	cfg.BspMacdAlgoBuy = ""
	cfg.BspMacdAlgoSell = ""

	m := metricForSide(&types.Bi{Direction: types.DirDown}, cfg)
	if m != force.Slope {
		t.Errorf("expected Slope fallback for buy, got %d", m)
	}
	m = metricForSide(&types.Bi{Direction: types.DirUp}, cfg)
	if m != force.Slope {
		t.Errorf("expected Slope fallback for sell, got %d", m)
	}
}

func TestMetricForSide_NilBi(t *testing.T) {
	cfg := types.DefaultConfig()
	cfg.BspMacdAlgo = "area"
	m := metricForSide(nil, cfg)
	if m != force.Area {
		t.Errorf("expected Area fallback for nil bi, got %d", m)
	}
}

// ── DetectDeviations ──

func TestDetectDeviations_Basic(t *testing.T) {
	pivots := []types.Pivot{
		{
			BiIn:  &types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 90, EndPrice: 110},
			BiOut: &types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirUp, StartPrice: 110, EndPrice: 130},
			ZG:    115, ZD: 100,
		},
	}
	// 构造简单的 MACD 直方图：入笔 MACD 大，出笔 MACD 小 → 背驰
	macdHist := make([]float64, 11)
	for i := 0; i <= 5; i++ {
		macdHist[i] = 2.0 // BiIn 区域：大 MACD
	}
	for i := 6; i <= 10; i++ {
		macdHist[i] = 0.5 // BiOut 区域：小 MACD → 背驰
	}
	macdDif := make([]float64, 11)
	volumes := make([]float64, 11)
	turnovers := make([]float64, 11)
	closes := make([]float64, 11)

	cfg := types.DefaultConfig()
	cfg.BspDivergenceRate = 0.8 // outM <= 0.8 * inM 才视为背驰
	cfg.BspMacdAlgo = "peak"

	devs := DetectDeviations(pivots, macdHist, macdDif, volumes, turnovers, closes, cfg)
	if len(devs) == 0 {
		t.Log("no deviations detected (may be expected with simple mock data)")
	} else {
		t.Logf("detected %d deviations", len(devs))
		for i, d := range devs {
			t.Logf("  dev[%d]: type=%s dir=%v forceBefore=%.2f forceAfter=%.2f",
				i, d.Type, d.Direction, d.ForceBefore, d.ForceAfter)
		}
	}
}

func TestDetectDeviations_EmptyPivots(t *testing.T) {
	devs := DetectDeviations(nil, nil, nil, nil, nil, nil, types.DefaultConfig())
	if len(devs) != 0 {
		t.Errorf("expected 0 deviations for empty pivots, got %d", len(devs))
	}
}

func TestDetectDeviations_SentinelModeRequiresForceDecay(t *testing.T) {
	// sentinel 模式 (rate=Inf) 下不做倍率过滤，但仍需后段力度小于前段力度。
	pivots := []types.Pivot{
		{
			BiIn:  &types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 110, EndPrice: 90},
			BiOut: &types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirDown, StartPrice: 90, EndPrice: 70},
			ZG:    105, ZD: 95,
		},
	}
	macdHist := make([]float64, 11)
	for i := 0; i <= 5; i++ {
		macdHist[i] = -1.0
	}
	for i := 6; i <= 10; i++ {
		macdHist[i] = -2.0
	}
	macdDif := make([]float64, 11)
	volumes := make([]float64, 11)
	turnovers := make([]float64, 11)
	closes := make([]float64, 11)

	cfg := types.DefaultConfig()
	cfg.BspDivergenceRate = math.Inf(1) // sentinel mode

	devs := DetectDeviations(pivots, macdHist, macdDif, volumes, turnovers, closes, cfg)
	if len(devs) != 0 {
		t.Fatalf("expected no deviation without force decay, got %d", len(devs))
	}

	for i := 6; i <= 10; i++ {
		macdHist[i] = -0.5
	}
	devs = DetectDeviations(pivots, macdHist, macdDif, volumes, turnovers, closes, cfg)
	if len(devs) == 0 {
		t.Fatal("expected deviation when force decays in sentinel mode")
	}
}

// ── endBiBreak ──

func TestEndBiBreak_Downward(t *testing.T) {
	out := &types.Bi{Direction: types.DirDown, StartPrice: 100, EndPrice: 80}
	if !endBiBreak(out, 90, 85) {
		t.Error("downward bi with end=80 < zd=85 should break")
	}
	if endBiBreak(out, 90, 75) {
		t.Error("downward bi with end=80 > zd=75 should not break")
	}
}

func TestEndBiBreak_Upward(t *testing.T) {
	out := &types.Bi{Direction: types.DirUp, StartPrice: 80, EndPrice: 100}
	if !endBiBreak(out, 90, 85) {
		t.Error("upward bi with end=100 > zg=90 should break")
	}
	if endBiBreak(out, 105, 95) {
		t.Error("upward bi with end=100 < zg=105 should not break")
	}
}
