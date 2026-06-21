package multi

import (
	"testing"
	"time"

	"github.com/bambuo/chan/types"
)

func makeKline(t time.Time, o, h, l, c float64) types.Kline {
	return types.Kline{
		Time: types.DateTime{Time: t},
		Open: o, High: h, Low: l, Close: c,
		BaseVolume: 1000,
	}
}

func TestMapIndexByTime_Basic(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	baseKlines := []types.Kline{
		makeKline(baseTime, 100, 102, 99, 101),
		makeKline(baseTime.Add(1*time.Minute), 101, 103, 100, 102),
		makeKline(baseTime.Add(2*time.Minute), 102, 104, 101, 103),
		makeKline(baseTime.Add(3*time.Minute), 103, 105, 102, 104),
		makeKline(baseTime.Add(4*time.Minute), 104, 106, 103, 105),
	}
	higherKlines := []types.Kline{
		makeKline(baseTime, 100, 102, 99, 101),                      // 5m bar covering [0,5) min
		makeKline(baseTime.Add(5*time.Minute), 105, 107, 104, 106),  // 5m bar covering [5,10) min
		makeKline(baseTime.Add(10*time.Minute), 110, 112, 109, 111), // 5m bar covering [10,15) min
	}

	// baseIdx=0 → time=0:00 → higher[0] (0:00)
	if idx := mapIndexByTime(0, baseKlines, higherKlines); idx != 0 {
		t.Errorf("expected idx=0 for base[0], got %d", idx)
	}
	// baseIdx=4 → time=0:04 → higher[1] (0:05 = first >= 0:04)
	if idx := mapIndexByTime(4, baseKlines, higherKlines); idx != 1 {
		t.Errorf("expected idx=1 for base[4] (time=0:04→higher[1]=0:05), got %d", idx)
	}
	// baseIdx=5 → out of range → 返回 higherKlines 末尾索引 2
	if idx := mapIndexByTime(5, baseKlines, higherKlines); idx != 2 {
		t.Errorf("expected idx=2 for out-of-range base[5], got %d", idx)
	}
}

func TestMapIndexByTime_OutOfRange(t *testing.T) {
	baseKlines := []types.Kline{
		makeKline(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 100, 102, 99, 101),
	}
	higherKlines := []types.Kline{
		makeKline(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 100, 102, 99, 101),
	}

	// 负数索引 → 返回 0
	if idx := mapIndexByTime(-1, baseKlines, higherKlines); idx != 0 {
		t.Errorf("expected idx=0 for baseIdx=-1, got %d", idx)
	}
	// 越界索引 → 返回末尾
	if idx := mapIndexByTime(100, baseKlines, higherKlines); idx != len(higherKlines)-1 {
		t.Errorf("expected idx=%d for baseIdx=100, got %d", len(higherKlines)-1, idx)
	}
}

func TestMapIndexByTime_EmptyTime(t *testing.T) {
	// 空时间应退化为恒等映射
	baseKlines := []types.Kline{
		{Open: 100, High: 102, Low: 99, Close: 101},
		{Open: 101, High: 103, Low: 100, Close: 102},
	}
	higherKlines := []types.Kline{
		{Open: 100, High: 102, Low: 99, Close: 101},
		{Open: 101, High: 103, Low: 100, Close: 102},
	}
	if idx := mapIndexByTime(1, baseKlines, higherKlines); idx != 1 {
		t.Errorf("with empty times, expected identity idx=1, got %d", idx)
	}
}

func TestCalcResonance_Basic(t *testing.T) {
	baseSignals := []types.Signal{
		{Type: types.BuyPoint1, SubType: types.SubT1, Index: 10, Price: 100},
	}
	higherSignals := []types.Signal{
		{Type: types.BuyPoint1, SubType: types.SubT1, Index: 10, Price: 100.5}, // 0.5% diff
		{Type: types.SellPoint1, SubType: types.SubT1, Index: 20, Price: 110},  // 反向
	}
	// tolerance=0.01 (1%) → 100 vs 100.5 → diff=0.005 < 0.01 → 共振
	cnt := calcResonance(baseSignals, higherSignals, 0.01)
	if cnt != 1 {
		t.Errorf("expected 1 resonance, got %d", cnt)
	}

	// tolerance=0.001 (0.1%) → diff=0.005 > 0.001 → 不共振
	cnt = calcResonance(baseSignals, higherSignals, 0.001)
	if cnt != 0 {
		t.Errorf("expected 0 resonance with tight tolerance, got %d", cnt)
	}
}

func TestCalcResonance_EmptyInput(t *testing.T) {
	if cnt := calcResonance(nil, nil, 0.01); cnt != 0 {
		t.Errorf("expected 0 for nil input, got %d", cnt)
	}
}

func TestSegmentsToBis_Basic(t *testing.T) {
	// 基础 K 线覆盖 0-9 分钟（10 根 1m K 线）
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	baseKlines := make([]types.Kline, 10)
	for i := range baseKlines {
		baseKlines[i] = makeKline(baseTime.Add(time.Duration(i)*time.Minute),
			100+float64(i), 102+float64(i), 99+float64(i), 101+float64(i))
	}
	// 高级别 K 线：两根 5m K 线，分别覆盖 [0,5) 和 [5,10)
	higherKlines := []types.Kline{
		makeKline(baseTime, 100, 106, 99, 105),
		makeKline(baseTime.Add(5*time.Minute), 105, 111, 104, 110),
	}

	// 覆盖基础 K 线 [0,4] 的向上线段 → 映射到 higher[0]
	segs := []types.Segment{
		{
			StartIndex: 0, EndIndex: 4,
			Direction: types.DirUp,
			Top:       106, Bottom: 99,
			IsSure: true,
		},
	}

	bis := segmentsToBis(segs, baseKlines, higherKlines)
	if len(bis) != 1 {
		t.Fatalf("expected 1 bi, got %d", len(bis))
	}
	if bis[0].Direction != types.DirUp {
		t.Errorf("expected DirUp, got %v", bis[0].Direction)
	}
	// startIdx=0 (base[0]=0:00 → higher[0]=0:00)
	// endIdx=0 (base[4]=0:04 → higher[0]=0:00 之后无匹配 → fallback=0)
	// → startIdx=0 >= endIdx=0 → 跳过?
	// 实际: base[4]=0:04, higher[0]=0:00 (<), higher[1]=0:05 (>=) → endIdx=1
	// → startIdx=0 < endIdx=1 → 不跳过 ✓
	_ = bis[0]
	t.Logf("bi mapped to higher[%d..%d]", bis[0].StartIndex, bis[0].EndIndex)
}
