package deviation

import (
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
