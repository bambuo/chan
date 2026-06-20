package chanlun

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// generateBenchKlines 生成指定数量的振荡 K 线用于基准测试。
func generateBenchKlines(n int) []Kline {
	klines := make([]Kline, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range klines {
		phase := float64(i) * 0.4
		osc := 8.0 * math.Sin(phase)
		trend := float64(i) * 0.15
		mid := 100.0 + trend + osc
		klines[i] = Kline{
			Time:       DateTime{Time: t.Add(time.Duration(i) * time.Hour)},
			Open:       mid - 0.5,
			High:       mid + 2.0,
			Low:        mid - 1.5,
			Close:      mid + 0.3,
			BaseVolume: 1000,
		}
	}
	return klines
}

// BenchmarkEngine 基准测试：不同数据量下的完整流水线性能。
func BenchmarkEngine(b *testing.B) {
	sizes := []int{100, 500, 2000, 5000, 10000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			klines := generateBenchKlines(n)
			engine, _ := NewEngine(DefaultConfig())
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := engine.Process(klines)
				if err != nil {
					b.Fatalf("Process failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkEngine_Update 基准测试：增量更新性能。
func BenchmarkEngine_Update(b *testing.B) {
	// 先处理 10,000 根
	stream, _ := NewStreamEngine(DefaultConfig())
	klines := generateBenchKlines(10000)
	_, err := stream.Init(klines)
	if err != nil {
		b.Fatalf("initial Init: %v", err)
	}

	newKline := Kline{
		Time:       DateTime{Time: time.Now()},
		Open:       105,
		High:       108,
		Low:        103,
		Close:      106,
		BaseVolume: 1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream.AddKline(newKline)
	}
}

// BenchmarkEngine_AllSamePrice 基准测试：全相同价格（边界情况）。
func BenchmarkEngine_AllSamePrice(b *testing.B) {
	klines := make([]Kline, 5000)
	for i := range klines {
		klines[i] = Kline{High: 100, Low: 100, Close: 100, Open: 100, BaseVolume: 1000}
	}

	engine, _ := NewEngine(DefaultConfig())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Process(klines)
	}
}

// BenchmarkFractalDetection 基准测试：分型识别性能。
func BenchmarkFractalDetection(b *testing.B) {
	klines := generateBenchKlines(10000)
	merged := MergeKlines(klines)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		FindFractals(merged, 1)
	}
}

// BenchmarkBiConstruction 基准测试：笔构建性能。
func BenchmarkBiConstruction(b *testing.B) {
	klines := generateBenchKlines(10000)
	merged := MergeKlines(klines)
	fractals := FindFractals(merged, 1)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		BuildBis(merged, fractals, 1, 0)
	}
}

// BenchmarkSegmentBuild 基准测试：线段构建性能。
func BenchmarkSegmentBuild(b *testing.B) {
	klines := generateBenchKlines(10000)
	merged := MergeKlines(klines)
	fractals := FindFractals(merged, 1)
	bis := BuildBis(merged, fractals, 1, 0)
	mergedBis := MergeBis(bis)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		BuildSegments(mergedBis)
	}
}

// TestStress_LargeData 压力测试：50,000 根 K 线。
func TestStress_LargeData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large data stress test in short mode")
	}

	klines := generateBenchKlines(50000)
	start := time.Now()

	engine, err := NewEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	result, err := engine.Process(klines)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Process 50k failed: %v", err)
	}

	t.Logf("50,000 klines processed in %v (%.0f klines/sec)", elapsed, float64(50000)/elapsed.Seconds())
	t.Logf("Fractals: %d, Bis: %d, Segments: %d, Pivots: %d, Trends: %d",
		len(result.Fractals), len(result.Bis), len(result.Segments),
		len(result.Pivots), len(result.Trends))
}

// TestStress_Memory 内存压力测试：反复处理大量数据。
func TestStress_Memory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory stress test in short mode")
	}

	engine, _ := NewEngine(DefaultConfig())

	for i := 0; i < 10; i++ {
		klines := generateBenchKlines(10000)
		result, err := engine.Process(klines)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		_ = result
	}
	t.Log("10 iterations of 10k klines completed without memory issues")
}

// TestConfigValidation 压力测试：配置校验覆盖。
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{"default", DefaultConfig(), false},
		{"new bi standard", NewBiConfig(), false},
		{"invalid BiMinKLineCount", Config{BiMinKLineCount: 0, MACDFastPeriod: 12, MACDSlowPeriod: 26, MACDSignalPeriod: 9, DIFReturnThreshold: 0.15}, true},
		{"invalid MACD params", Config{BiMinKLineCount: 1, MACDFastPeriod: 30, MACDSlowPeriod: 26, MACDSignalPeriod: 9, DIFReturnThreshold: 0.15}, true},
		{"invalid force method", Config{BiMinKLineCount: 1, MACDFastPeriod: 12, MACDSlowPeriod: 26, MACDSignalPeriod: 9, DIFReturnThreshold: 0.15, DeviationForceMethod: "invalid"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestKlineValidation 压力测试：K线输入校验覆盖。
func TestKlineValidation(t *testing.T) {
	valid := Kline{High: 100, Low: 90, Close: 95, Open: 92, BaseVolume: 1000}
	t0 := valid
	t1 := valid
	t2 := valid
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t0.Time = DateTime{Time: baseTime}
	t1.Time = DateTime{Time: baseTime.Add(time.Hour)}
	t2.Time = DateTime{Time: baseTime.Add(2 * time.Hour)}

	tests := []struct {
		name    string
		klines  []Kline
		wantErr bool
	}{
		{"nil", nil, true},
		{"empty", []Kline{}, true},
		{"single", []Kline{valid}, true},
		{"two", []Kline{valid, valid}, true},
		{"three valid", []Kline{valid, valid, valid}, false},
		{"NaN price", []Kline{valid, {High: math.NaN(), Low: 90, Close: 95}, valid}, true},
		{"Inf price", []Kline{valid, {High: math.Inf(1), Low: 90, Close: 95}, valid}, true},
		{"High < Low", []Kline{valid, {High: 80, Low: 90, Close: 85}, valid}, true},
		{"negative volume", []Kline{valid, {High: 100, Low: 90, Close: 95, BaseVolume: -1}, valid}, true},
		{"NaN base volume", []Kline{valid, {High: 100, Low: 90, Close: 95, BaseVolume: math.NaN()}, valid}, true},
		{"Inf turnover", []Kline{valid, {High: 100, Low: 90, Close: 95, Turnover: math.Inf(1)}, valid}, true},
		{"negative trade count", []Kline{valid, {High: 100, Low: 90, Close: 95, TradeCount: -1}, valid}, true},
		{"non ascending time", []Kline{t0, t2, t1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKlines(tt.klines)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKlines() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
