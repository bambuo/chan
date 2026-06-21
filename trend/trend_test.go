package trend

import (
	"testing"
	"time"

	"github.com/bambuo/chan/types"
)

func TestCalcTrendMetrics(t *testing.T) {
	klines := make([]types.Kline, 20)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range klines {
		klines[i] = types.Kline{
			Time:       types.DateTime{Time: base.Add(time.Duration(i) * time.Hour)},
			Open:       float64(100 + i),
			High:       float64(101 + i),
			Low:        float64(99 + i),
			Close:      float64(100 + i),
			BaseVolume: float64(100),
		}
	}

	trends := []types.Trend{
		{StartIndex: 0, EndIndex: 9, Pivots: []types.Pivot{{}}},
		{StartIndex: 10, EndIndex: 19, Pivots: []types.Pivot{{}}},
	}

	CalcTrendMetrics(trends, klines, []int{5, 10})
	for i, tr := range trends {
		if tr.Metrics == nil {
			t.Errorf("trend[%d]: Metrics is nil", i)
			continue
		}
		// 检查均值
		m5, ok := tr.Metrics[5]
		if !ok {
			t.Errorf("trend[%d]: Metrics[5] missing", i)
		} else if m5 <= 0 {
			t.Errorf("trend[%d]: Metrics[5]=%.2f should be > 0", i, m5)
		}
		// 检查 max (5*10+1=51)
		mx, ok := tr.Metrics[51]
		if !ok {
			t.Errorf("trend[%d]: Metrics[51] missing", i)
		} else if mx <= 0 {
			t.Errorf("trend[%d]: Metrics[51]=%.2f should be > 0", i, mx)
		}
		// 检查 min (5*10+2=52)
		mn, ok := tr.Metrics[52]
		if !ok {
			t.Errorf("trend[%d]: Metrics[52] missing", i)
		} else if mn <= 0 {
			t.Errorf("trend[%d]: Metrics[52]=%.2f should be > 0", i, mn)
		}
	}
}

func TestCalcTrendMetrics_EmptyInput(t *testing.T) {
	CalcTrendMetrics(nil, nil, nil) // should not panic
	CalcTrendMetrics(nil, []types.Kline{}, []int{5})
}

func TestCalcTrendMetrics_NoMetrics(t *testing.T) {
	trends := []types.Trend{
		{StartIndex: 0, EndIndex: 5, Pivots: []types.Pivot{{}}},
	}
	klines := make([]types.Kline, 10)
	CalcTrendMetrics(trends, klines, nil)
	if trends[0].Metrics != nil {
		t.Error("expected nil Metrics when no metrics requested")
	}
}
