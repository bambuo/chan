package chanlun

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// generateBenchCandles 生成指定数量的振荡 K 线用于基准测试。
func generateBenchCandles(n int) []Candle {
	candles := make([]Candle, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range candles {
		phase := float64(i) * 0.4
		osc := 8.0 * math.Sin(phase)
		trend := float64(i) * 0.15
		mid := 100.0 + trend + osc
		candles[i] = Candle{
			Time:   t.Add(time.Duration(i) * time.Hour),
			Open:   mid - 0.5,
			High:   mid + 2.0,
			Low:    mid - 1.5,
			Close:  mid + 0.3,
			Volume: 1000,
		}
	}
	return candles
}

// BenchmarkEngine 基准测试：不同数据量下的完整流水线性能。
func BenchmarkEngine(b *testing.B) {
	sizes := []int{100, 500, 2000, 5000, 10000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			candles := generateBenchCandles(n)
			engine, _ := NewEngine(DefaultConfig())
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := engine.Process(candles)
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
	engine, _ := NewEngine(DefaultConfig())
	candles := generateBenchCandles(10000)
	_, err := engine.Process(candles)
	if err != nil {
		b.Fatalf("initial Process: %v", err)
	}

	newCandle := Candle{
		Time:   time.Now(),
		Open:   105,
		High:   108,
		Low:    103,
		Close:  106,
		Volume: 1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Update(newCandle)
		if err != nil {
			b.Fatalf("Update failed: %v", err)
		}
	}
}

// BenchmarkEngine_AllSamePrice 基准测试：全相同价格（边界情况）。
func BenchmarkEngine_AllSamePrice(b *testing.B) {
	candles := make([]Candle, 5000)
	for i := range candles {
		candles[i] = Candle{High: 100, Low: 100, Close: 100, Open: 100, Volume: 1000}
	}

	engine, _ := NewEngine(DefaultConfig())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Process(candles)
	}
}

// BenchmarkFractalDetection 基准测试：分型识别性能。
func BenchmarkFractalDetection(b *testing.B) {
	candles := generateBenchCandles(10000)
	merged := MergeCandles(candles)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		FindFractals(merged, 1)
	}
}

// BenchmarkBiConstruction 基准测试：笔构建性能。
func BenchmarkBiConstruction(b *testing.B) {
	candles := generateBenchCandles(10000)
	merged := MergeCandles(candles)
	fractals := FindFractals(merged, 1)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		BuildBis(merged, fractals, 1, 0)
	}
}

// BenchmarkSegmentBuild 基准测试：线段构建性能。
func BenchmarkSegmentBuild(b *testing.B) {
	candles := generateBenchCandles(10000)
	merged := MergeCandles(candles)
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

	candles := generateBenchCandles(50000)
	start := time.Now()

	engine, err := NewEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	result, err := engine.Process(candles)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Process 50k failed: %v", err)
	}

	t.Logf("50,000 candles processed in %v (%.0f candles/sec)", elapsed, float64(50000)/elapsed.Seconds())
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
		candles := generateBenchCandles(10000)
		result, err := engine.Process(candles)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		_ = result
	}
	t.Log("10 iterations of 10k candles completed without memory issues")
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

// TestCandleValidation 压力测试：K线输入校验覆盖。
func TestCandleValidation(t *testing.T) {
	valid := Candle{High: 100, Low: 90, Close: 95, Open: 92, Volume: 1000}

	tests := []struct {
		name    string
		candles []Candle
		wantErr bool
	}{
		{"nil", nil, true},
		{"empty", []Candle{}, true},
		{"single", []Candle{valid}, true},
		{"two", []Candle{valid, valid}, true},
		{"three valid", []Candle{valid, valid, valid}, false},
		{"NaN price", []Candle{valid, {High: math.NaN(), Low: 90, Close: 95}, valid}, true},
		{"Inf price", []Candle{valid, {High: math.Inf(1), Low: 90, Close: 95}, valid}, true},
		{"High < Low", []Candle{valid, {High: 80, Low: 90, Close: 85}, valid}, true},
		{"negative volume", []Candle{valid, {High: 100, Low: 90, Close: 95, Volume: -1}, valid}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCandles(tt.candles)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCandles() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
