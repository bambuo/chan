package force

import (
	"math"
	"testing"

	"github.com/bambuo/chan/types"
)

func TestCalcMetricDiffUsesMACDDif(t *testing.T) {
	bi := types.Bi{StartIndex: 1, EndIndex: 3, Direction: types.DirUp}
	macdHist := []float64{0, 100, 100, 100}
	macdDif := []float64{0, 1, 3, 2}

	got := CalcMetric(bi, Diff, false, macdHist, macdDif, nil, nil, nil)
	if got != 2 {
		t.Fatalf("Diff metric = %.2f, want 2.00 from macdDif range", got)
	}
}

// ── ParseMetric ──

func TestParseMetric_AllTypes(t *testing.T) {
	tests := []struct {
		input string
		want  MetricType
	}{
		{"area", Area}, {"peak", Peak}, {"full_area", FullArea},
		{"diff", Diff}, {"slope", Slope}, {"amp", Amp},
		{"amount", Amount}, {"volume", Volume}, {"volumn", Volume},
		{"amount_avg", AmountAvg}, {"volume_avg", VolumeAvg}, {"volumn_avg", VolumeAvg},
		{"rsi", Rsi}, {"unknown", Peak}, // default → Peak
	}
	for _, tt := range tests {
		if got := ParseMetric(tt.input); got != tt.want {
			t.Errorf("ParseMetric(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// ── calcMACDPeak ──

func TestCalcMACDPeak_Normal(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 4, Direction: types.DirUp}
	hist := []float64{0.1, 0.5, 0.3, 0.8, 0.2}
	got := calcMACDPeak(bi, hist)
	if got != 0.8 {
		t.Errorf("peak = %.4f, want 0.8", got)
	}
}

func TestCalcMACDPeak_DownDirection(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 4, Direction: types.DirDown}
	hist := []float64{-0.1, -0.5, -0.3, -0.8, -0.2}
	got := calcMACDPeak(bi, hist)
	if got != 0.8 {
		t.Errorf("peak = %.4f, want 0.8", got)
	}
}

func TestCalcMACDPeak_ZeroValues(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 2, Direction: types.DirUp}
	hist := []float64{0, 0, 0}
	got := calcMACDPeak(bi, hist)
	if got != epsilon {
		t.Errorf("peak for all-zero hist = %g, want epsilon (%g)", got, epsilon)
	}
}

func TestCalcMACDPeak_WrongSignIgnored(t *testing.T) {
	// Up bi but all negative MACD → should return epsilon
	bi := types.Bi{StartIndex: 0, EndIndex: 2, Direction: types.DirUp}
	hist := []float64{-1.0, -2.0, -3.0}
	got := calcMACDPeak(bi, hist)
	if got != epsilon {
		t.Errorf("peak for wrong-sign hist = %g, want epsilon", got)
	}
}

// ── calcHalfArea ──

func TestCalcHalfArea_Forward(t *testing.T) {
	bi := types.Bi{StartIndex: 1, EndIndex: 4, Direction: types.DirUp}
	hist := []float64{0, 0.5, 0.3, 0.2, 0.1}
	got := calcHalfArea(bi, false, hist)
	// sum of |0.5|+|0.3|+|0.2|+|0.1| = 1.1 + epsilon
	want := epsilon + 1.1
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("halfArea forward = %.6f, want %.6f", got, want)
	}
}

func TestCalcHalfArea_Reverse(t *testing.T) {
	bi := types.Bi{StartIndex: 1, EndIndex: 4, Direction: types.DirUp}
	hist := []float64{0, 0.5, 0.3, 0.2, 0.1}
	got := calcHalfArea(bi, true, hist)
	// reverse: from end=4, peakMacd=0.1, then scan backwards
	// 0.1 same sign → sum, 0.2 same sign → sum, 0.3 same → sum, 0.5 same → sum
	want := epsilon + 0.1 + 0.2 + 0.3 + 0.5
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("halfArea reverse = %.6f, want %.6f", got, want)
	}
}

func TestCalcHalfArea_AllZero(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 3, Direction: types.DirUp}
	hist := []float64{0, 0, 0, 0}
	got := calcHalfArea(bi, false, hist)
	if got != epsilon {
		t.Errorf("halfArea all-zero = %g, want epsilon", got)
	}
}

// ── calcFullArea ──

func TestCalcFullArea_Up(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 3, Direction: types.DirUp}
	hist := []float64{0.5, -0.2, 0.3, 0.1}
	got := calcFullArea(bi, hist)
	// Only positive values for DirUp: 0.5 + 0.3 + 0.1 = 0.9 + epsilon
	want := epsilon + 0.9
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("fullArea up = %.6f, want %.6f", got, want)
	}
}

func TestCalcFullArea_Down(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 3, Direction: types.DirDown}
	hist := []float64{-0.5, 0.2, -0.3, -0.1}
	got := calcFullArea(bi, hist)
	// Only negative values for DirDown: 0.5 + 0.3 + 0.1 = 0.9 + epsilon
	want := epsilon + 0.9
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("fullArea down = %.6f, want %.6f", got, want)
	}
}

// ── calcDiffRange ──

func TestCalcDiffRange_Normal(t *testing.T) {
	bi := types.Bi{StartIndex: 1, EndIndex: 4, Direction: types.DirUp}
	dif := []float64{0, 1.5, 3.0, 2.0, 0.5}
	got := calcDiffRange(bi, dif)
	// max=3.0, min=0.5, range=2.5
	if got != 2.5 {
		t.Errorf("diffRange = %.4f, want 2.5", got)
	}
}

func TestCalcDiffRange_EmptyRange(t *testing.T) {
	bi := types.Bi{StartIndex: 5, EndIndex: 5, Direction: types.DirUp}
	dif := []float64{1.0}
	got := calcDiffRange(bi, dif)
	// StartIndex=5 but len=1 → no valid data → return 0
	if got != 0 {
		t.Errorf("diffRange empty = %.4f, want 0", got)
	}
}

// ── calcSlope ──

func TestCalcSlope_Up(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 9, Direction: types.DirUp, StartPrice: 100, EndPrice: 110}
	got := calcSlope(bi)
	// (110-100)/110/10 = 10/110/10 ≈ 0.00909
	want := (110.0 - 100.0) / 110.0 / 10.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("slope up = %.8f, want %.8f", got, want)
	}
}

func TestCalcSlope_Down(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 9, Direction: types.DirDown, StartPrice: 110, EndPrice: 100}
	got := calcSlope(bi)
	// (110-100)/110/10
	want := (110.0 - 100.0) / 110.0 / 10.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("slope down = %.8f, want %.8f", got, want)
	}
}

func TestCalcSlope_ZeroEndPrice(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 100, EndPrice: 0}
	got := calcSlope(bi)
	if got != 0 {
		t.Errorf("slope with zero end = %g, want 0", got)
	}
}

// ── calcAmp ──

func TestCalcAmp_Up(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 100, EndPrice: 120}
	got := calcAmp(bi)
	if got != 0.2 {
		t.Errorf("amp up = %.4f, want 0.2", got)
	}
}

func TestCalcAmp_Down(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 100, EndPrice: 80}
	got := calcAmp(bi)
	if got != 0.2 {
		t.Errorf("amp down = %.4f, want 0.2", got)
	}
}

func TestCalcAmp_ZeroStart(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 0, EndPrice: 100}
	got := calcAmp(bi)
	if got != 0 {
		t.Errorf("amp zero start = %g, want 0", got)
	}
}

// ── calcTradeMetric ──

func TestCalcTradeMetric_Total(t *testing.T) {
	bi := types.Bi{StartIndex: 1, EndIndex: 3, Direction: types.DirUp}
	data := []float64{0, 100, 200, 300}
	got := calcTradeMetric(bi, data, false)
	if got != 600 {
		t.Errorf("trade total = %.2f, want 600", got)
	}
}

func TestCalcTradeMetric_Avg(t *testing.T) {
	bi := types.Bi{StartIndex: 1, EndIndex: 3, Direction: types.DirUp}
	data := []float64{0, 100, 200, 300}
	got := calcTradeMetric(bi, data, true)
	if got != 200 {
		t.Errorf("trade avg = %.2f, want 200", got)
	}
}

func TestCalcTradeMetric_NilData(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp}
	got := calcTradeMetric(bi, nil, false)
	if got != 0 {
		t.Errorf("trade nil data = %g, want 0", got)
	}
}

// ── CalcRSIMetric (public API) ──

func TestCalcRSIMetric_BasicUp(t *testing.T) {
	// 20 closes monotonically increasing → RSI should be high
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = 100 + float64(i)
	}
	bi := types.Bi{StartIndex: 14, EndIndex: 29, Direction: types.DirUp}
	got := CalcRSIMetric(bi, 14, closes)
	if got < 90 {
		t.Errorf("RSI for monotonically increasing = %.2f, expected > 90", got)
	}
}

func TestCalcRSIMetric_BasicDown(t *testing.T) {
	// 20 closes monotonically decreasing → RSI for down should be high (inverse)
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = 200 - float64(i)
	}
	bi := types.Bi{StartIndex: 14, EndIndex: 29, Direction: types.DirDown}
	got := CalcRSIMetric(bi, 14, closes)
	if got < 1 {
		t.Errorf("RSI for monotonically decreasing down = %.2f, expected > 1", got)
	}
}

func TestCalcRSIMetric_TooShort(t *testing.T) {
	closes := []float64{100, 101, 102}
	bi := types.Bi{StartIndex: 0, EndIndex: 2, Direction: types.DirUp}
	got := CalcRSIMetric(bi, 14, closes)
	if got != 0 {
		t.Errorf("RSI too short = %g, want 0", got)
	}
}

// ── CalcMetric dispatch ──

func TestCalcMetric_Dispatch(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 2, Direction: types.DirUp, StartPrice: 100, EndPrice: 110, High: 112, Low: 98}
	hist := []float64{0.5, 0.8, 0.3}
	dif := []float64{1.0, 2.0, 1.5}
	vol := []float64{100, 200, 300}
	turnover := []float64{1000, 2000, 3000}

	// Area
	if v := CalcMetric(bi, Area, false, hist, nil, nil, nil, nil); v <= epsilon {
		t.Errorf("Area = %g, expected > epsilon", v)
	}
	// Peak
	if v := CalcMetric(bi, Peak, false, hist, nil, nil, nil, nil); v <= epsilon {
		t.Errorf("Peak = %g, expected > epsilon", v)
	}
	// FullArea
	if v := CalcMetric(bi, FullArea, false, hist, nil, nil, nil, nil); v <= epsilon {
		t.Errorf("FullArea = %g, expected > epsilon", v)
	}
	// Diff
	if v := CalcMetric(bi, Diff, false, hist, dif, nil, nil, nil); v <= 0 {
		t.Errorf("Diff = %g, expected > 0", v)
	}
	// Slope
	if v := CalcMetric(bi, Slope, false, nil, nil, nil, nil, nil); v <= 0 {
		t.Errorf("Slope = %g, expected > 0", v)
	}
	// Amp
	if v := CalcMetric(bi, Amp, false, nil, nil, nil, nil, nil); v <= 0 {
		t.Errorf("Amp = %g, expected > 0", v)
	}
	// Volume
	if v := CalcMetric(bi, Volume, false, nil, nil, vol, nil, nil); v != 600 {
		t.Errorf("Volume = %g, want 600", v)
	}
	// Amount
	if v := CalcMetric(bi, Amount, false, nil, nil, nil, turnover, nil); v != 6000 {
		t.Errorf("Amount = %g, want 6000", v)
	}
	// VolumeAvg
	if v := CalcMetric(bi, VolumeAvg, false, nil, nil, vol, nil, nil); v != 200 {
		t.Errorf("VolumeAvg = %g, want 200", v)
	}
	// AmountAvg
	if v := CalcMetric(bi, AmountAvg, false, nil, nil, nil, turnover, nil); v != 2000 {
		t.Errorf("AmountAvg = %g, want 2000", v)
	}
}
