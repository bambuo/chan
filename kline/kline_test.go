package kline

import (
	"testing"
	"time"

	"github.com/bambuo/chan/types"
)

func TestAggregateKlines_Basic(t *testing.T) {
	base := time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC)
	src := make([]types.Kline, 5)
	for i := 0; i < 5; i++ {
		src[i] = types.Kline{
			Time:       types.DateTime{Time: base.Add(time.Duration(i) * time.Minute)},
			Open:       float64(100 + i),
			High:       float64(101 + i),
			Low:        float64(99 + i),
			Close:      float64(100 + i),
			BaseVolume: float64(100 + i),
		}
	}
	agg, err := AggregateKlines(src, "5m")
	if err != nil {
		t.Fatalf("AggregateKlines failed: %v", err)
	}
	if len(agg) != 1 {
		t.Fatalf("expected 1 aggregated kline, got %d", len(agg))
	}
	if agg[0].Open != 100 {
		t.Errorf("Open: got %.0f, want 100", agg[0].Open)
	}
	if agg[0].High != 105 {
		t.Errorf("High: got %.0f, want 105", agg[0].High)
	}
	if agg[0].Low != 99 {
		t.Errorf("Low: got %.0f, want 99", agg[0].Low)
	}
	if agg[0].Close != 104 {
		t.Errorf("Close: got %.0f, want 104", agg[0].Close)
	}
	if agg[0].BaseVolume != 510 {
		t.Errorf("Volume: got %.0f, want 510", agg[0].BaseVolume)
	}
}

func TestMergeKlines_DirectionUsesMergedResult(t *testing.T) {
	// 验证方向基于合并结果序列而非原始序列（对齐 chan.py 合并后 KLC 比较）。
	// k0+k1 向上合并为 {H:10, L:5}，方向保持 DirUp；
	// k2={H:9, L:5.5} 被包含，向上合并为 {H:10, L:5.5}。
	klines := []types.Kline{
		{High: 10, Low: 5, Open: 8, Close: 7},
		{High: 10, Low: 4, Open: 7, Close: 5},
		{High: 9, Low: 5.5, Open: 6, Close: 6},
	}

	merged := MergeKlines(klines)
	if len(merged) != 1 {
		t.Fatalf("expected all klines to merge into 1 item, got %d", len(merged))
	}
	if merged[0].High != 10 || merged[0].Low != 5.5 {
		t.Fatalf("merged range = [%.2f, %.2f], want [10.00, 5.50]", merged[0].High, merged[0].Low)
	}
}

func TestAggregateKlines_MultiBucket(t *testing.T) {
	base := time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC)
	src := make([]types.Kline, 12)
	for i := 0; i < 12; i++ {
		src[i] = types.Kline{
			Time:       types.DateTime{Time: base.Add(time.Duration(i) * time.Minute)},
			Open:       float64(100 + i),
			High:       float64(102 + i),
			Low:        float64(98 + i),
			Close:      float64(101 + i),
			BaseVolume: float64(100),
		}
	}
	agg, err := AggregateKlines(src, "5m")
	if err != nil {
		t.Fatalf("AggregateKlines failed: %v", err)
	}
	if len(agg) != 3 {
		t.Fatalf("expected 3 aggregated klines, got %d", len(agg))
	}
	if agg[0].Time.Format("15:04") != "09:30" {
		t.Errorf("first bucket time: got %s, want 09:30", agg[0].Time.Format("15:04"))
	}
	if agg[1].Time.Format("15:04") != "09:35" {
		t.Errorf("second bucket time: got %s, want 09:35", agg[1].Time.Format("15:04"))
	}
}

func TestAggregateKlines_EmptyInput(t *testing.T) {
	_, err := AggregateKlines(nil, "5m")
	if err == nil {
		t.Error("expected error for nil input")
	}
}

func TestAggregateAll(t *testing.T) {
	base := time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC)
	src := make([]types.Kline, 10)
	for i := 0; i < 10; i++ {
		src[i] = types.Kline{
			Time:       types.DateTime{Time: base.Add(time.Duration(i) * time.Minute)},
			Open:       float64(100),
			High:       float64(105),
			Low:        float64(95),
			Close:      float64(102),
			BaseVolume: float64(100),
		}
	}
	m, err := AggregateAll(src, "5m", "10m")
	if err != nil {
		t.Fatalf("AggregateAll failed: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(m))
	}
	if len(m["5m"]) != 2 {
		t.Errorf("5m: expected 2 klines, got %d", len(m["5m"]))
	}
	if len(m["10m"]) != 1 {
		t.Errorf("10m: expected 1 kline, got %d", len(m["10m"]))
	}
}

func TestParsePeriodMinutes(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"1m", 1}, {"5m", 5}, {"15m", 15}, {"30m", 30},
		{"1h", 60}, {"2h", 120}, {"4h", 240}, {"6h", 360},
		{"1d", 1440}, {"1w", 10080},
	}
	for _, c := range cases {
		got, err := ParsePeriodMinutes(c.input)
		if err != nil {
			t.Errorf("ParsePeriodMinutes(%q): unexpected error: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParsePeriodMinutes(%q) = %d, want %d", c.input, got, c.want)
		}
	}
	if _, err := ParsePeriodMinutes("invalid"); err == nil {
		t.Error("expected error for invalid period")
	}
}
