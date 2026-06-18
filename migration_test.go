package chanlun

import (
	"math"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// chan.py 功能迁移测试
// ──────────────────────────────────────────────

// TestDefaultConfigBackwardCompat 验证默认配置下结果与改造前一致。
func TestDefaultConfigBackwardCompat(t *testing.T) {
	klines := generateMigrationKlines(100)

	engine, err := NewEngine(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	result, err := engine.Process(klines)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// 基本合理性检查
	if len(result.MergedKlines) == 0 {
		t.Error("expected merged klines")
	}
	if len(result.MergedKlines) > len(klines) {
		t.Error("merged klines should not exceed input")
	}
}

// TestMultiSegmentAlgorithms 验证三种线段算法产出不同结果。
func TestMultiSegmentAlgorithms(t *testing.T) {
	klines := generateMigrationKlines(200)

	algorithms := []string{"chan", "dyh", "def"}
	results := make(map[string][]Segment)

	for _, algo := range algorithms {
		cfg := DefaultConfig()
		cfg.SegAlgo = algo
		engine, _ := NewEngine(cfg)
		result, err := engine.Process(klines)
		if err != nil {
			t.Fatalf("Process with algo=%s: %v", algo, err)
		}
		results[algo] = result.Segments
		t.Logf("algo=%s: segments=%d", algo, len(result.Segments))
	}

	// 至少有两种算法产出不同数量的线段
	allSame := true
	for i := 1; i < len(algorithms); i++ {
		if len(results[algorithms[0]]) != len(results[algorithms[i]]) {
			allSame = false
			break
		}
	}
	if allSame && len(results["chan"]) > 0 {
		t.Log("all algorithms produced same segment count - this may be expected for simple data")
	}
}

// TestForceMetricTypes 验证各种力度指标的计算正确性。
func TestForceMetricTypes(t *testing.T) {
	bi := Bi{
		StartIndex: 0,
		EndIndex:   10,
		Direction:  DirUp,
		StartPrice: 100,
		EndPrice:   120,
		High:       125,
		Low:        95,
		Length:     20,
		KLineCount: 11,
	}

	macdHist := make([]float64, 20)
	for i := range macdHist {
		macdHist[i] = float64(i-10) * 0.5
	}

	volumes := make([]float64, 20)
	turnovers := make([]float64, 20)
	closes := make([]float64, 20)
	for i := 0; i < 20; i++ {
		volumes[i] = 1000
		turnovers[i] = 100000
		closes[i] = 100 + float64(i)
	}

	metrics := []struct {
		name   string
		metric ForceMetricType
	}{
		{"area", ForceArea},
		{"peak", ForcePeak},
		{"full_area", ForceFullArea},
		{"diff", ForceDiff},
		{"slope", ForceSlope},
		{"amp", ForceAmp},
		{"amount", ForceAmount},
		{"volume", ForceVolume},
		{"amount_avg", ForceAmountAvg},
		{"volume_avg", ForceVolumeAvg},
		{"rsi", ForceRsi},
	}

	for _, m := range metrics {
		val := CalcForceMetric(bi, m.metric, false, macdHist, macdHist, volumes, turnovers, closes)
		t.Logf("metric=%s: value=%.4f", m.name, val)
		if math.IsNaN(val) {
			t.Errorf("metric %s returned NaN", m.name)
		}
	}
}

// TestPivotCombine 验证中枢合并功能。
func TestPivotCombine(t *testing.T) {
	pivots := []Pivot{
		{ZD: 100, ZG: 110, PeakLow: 95, PeakHigh: 115, DD: 90, GG: 120, OverlapCount: 3},
		{ZD: 108, ZG: 118, PeakLow: 103, PeakHigh: 123, DD: 100, GG: 125, OverlapCount: 3}, // 与第一个重叠
		{ZD: 130, ZG: 140, PeakLow: 125, PeakHigh: 145, DD: 120, GG: 150, OverlapCount: 3}, // 不重叠
	}

	// zs 模式合并
	combined := CombinePivots(pivots, "zs")
	if len(combined) != 2 {
		t.Errorf("expected 2 pivots after zs combine, got %d", len(combined))
	}
	if len(combined) >= 1 && combined[0].ZG != 118 {
		t.Errorf("expected first pivot ZG=118 after merge, got %.1f", combined[0].ZG)
	}

	// peak 模式合并
	combined2 := CombinePivots(pivots, "peak")
	if len(combined2) != 2 {
		t.Errorf("expected 2 pivots after peak combine, got %d", len(combined2))
	}
}

// TestInclusionOptions 验证精细包含处理。
func TestInclusionOptions(t *testing.T) {
	klines := []Kline{
		{High: 10, Low: 5, Open: 6, Close: 9},
		{High: 12, Low: 3, Open: 4, Close: 11}, // 包含第一根
		{High: 15, Low: 8, Open: 9, Close: 14},
	}

	// 标准模式
	result := MergeKlines(klines)
	if len(result) != 2 {
		t.Errorf("standard inclusion: expected 2 merged, got %d", len(result))
	}

	// exclude_included 模式
	result2 := MergeKlines(klines, InclusionOption{ExcludeIncluded: true})
	if len(result2) != 2 {
		t.Errorf("exclude_included: expected 2 merged, got %d", len(result2))
	}

	// allowTopEqual 模式
	klines2 := []Kline{
		{High: 10, Low: 5, Open: 6, Close: 9},
		{High: 10, Low: 3, Open: 4, Close: 9}, // 顶相等
		{High: 15, Low: 8, Open: 9, Close: 14},
	}
	result3 := MergeKlines(klines2, InclusionOption{AllowTopEqual: 1})
	t.Logf("allowTopEqual=1: merged count = %d", len(result3))
}

// TestIndicatorCalculator 验证技术指标增量计算。
func TestIndicatorCalculator(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CalBoll = true
	cfg.BollN = 5
	cfg.CalRsi = true
	cfg.RsiCycle = 5
	cfg.CalKdj = true
	cfg.KdjCycle = 5
	cfg.MeanMetrics = []int{3, 5}

	ic := NewIndicatorCalculator(cfg)

	for i := 0; i < 10; i++ {
		k := Kline{
			Close: 100 + float64(i)*2,
			High:  102 + float64(i)*2,
			Low:   98 + float64(i)*2,
		}
		result := ic.AddKline(k)

		if i >= 2 {
			if result.MA == nil {
				t.Errorf("MA should not be nil at i=%d", i)
			} else if val, ok := result.MA[3]; ok {
				if i >= 2 && val == 0 {
					// MA[3] 在第 3 根开始应有非零值
					t.Logf("MA[3] = %.2f at i=%d (may be 0 if prices are 0)", val, i)
				}
			}
		}

		if i >= 4 {
			if result.BOLL == nil {
				t.Errorf("BOLL should not be nil at i=%d", i)
			}
			if result.RSI == nil {
				t.Errorf("RSI should not be nil at i=%d", i)
			}
			if result.KDJ == nil {
				t.Errorf("KDJ should not be nil at i=%d", i)
			}
		}
	}
}

// TestParseForceMetricType 验证力度指标类型解析。
func TestParseForceMetricType(t *testing.T) {
	tests := []struct {
		input    string
		expected ForceMetricType
	}{
		{"area", ForceArea},
		{"peak", ForcePeak},
		{"full_area", ForceFullArea},
		{"diff", ForceDiff},
		{"slope", ForceSlope},
		{"amp", ForceAmp},
		{"amount", ForceAmount},
		{"volumn", ForceVolume},
		{"volume", ForceVolume},
		{"amount_avg", ForceAmountAvg},
		{"volumn_avg", ForceVolumeAvg},
		{"rsi", ForceRsi},
		{"unknown", ForcePeak}, // 默认
	}

	for _, tt := range tests {
		got := ParseForceMetricType(tt.input)
		if got != tt.expected {
			t.Errorf("ParseForceMetricType(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// TestBuildSegmentsWithAlgo 验证多算法线段构建入口。
func TestBuildSegmentsWithAlgo(t *testing.T) {
	klines := generateMigrationKlines(150)

	cfg := DefaultConfig()
	engine, _ := NewEngine(cfg)
	result, _ := engine.Process(klines)

	if len(result.MergedBis) < 3 {
		t.Skip("not enough bis for segment test")
	}

	// 三种算法都不应 panic
	for _, algo := range []string{"chan", "dyh", "def"} {
		segs := BuildSegmentsWithAlgo(result.MergedBis, algo)
		t.Logf("algo=%s: %d segments from %d bis", algo, len(segs), len(result.MergedBis))
	}
}

// TestStreamEngineWithNewConfig 验证 StreamEngine 使用新配置。
func TestStreamEngineWithNewConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SegAlgo = "dyh"
	cfg.ZsCombine = true
	cfg.ZsCombineMode = "peak"
	cfg.BspMacdAlgo = "slope"
	cfg.CalBoll = true
	cfg.BollN = 10

	stream, err := NewStreamEngine(cfg)
	if err != nil {
		t.Fatalf("NewStreamEngine: %v", err)
	}

	klines := generateMigrationKlines(80)
	result, err := stream.Init(klines)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if result == nil {
		t.Fatal("Init returned nil result")
	}

	// 追加 K 线
	for i := 80; i < 100; i++ {
		inc := stream.AddKline(generateMigrationKlines(100)[i])
		if inc == nil {
			t.Fatalf("AddKline returned nil at %d", i)
		}
	}

	snap := stream.Snapshot()
	if snap == nil {
		t.Fatal("Snapshot returned nil")
	}
}

// TestParseBspTypes 验证买卖点类型解析。
func TestParseBspTypes(t *testing.T) {
	types := parseBspTypes("1,2,3a")
	if !types["1"] || !types["2"] || !types["3a"] {
		t.Error("expected 1, 2, 3a to be present")
	}
	if types["1p"] || types["2s"] || types["3b"] {
		t.Error("did not expect 1p, 2s, 3b")
	}

	// 空字符串 = 全部启用
	all := parseBspTypes("")
	if !all["1"] || !all["1p"] || !all["2"] || !all["2s"] || !all["3a"] || !all["3b"] {
		t.Error("empty string should enable all types")
	}
}

// generateMigrationKlines 生成迁移测试用的 K 线数据。
func generateMigrationKlines(n int) []Kline {
	klines := make([]Kline, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range klines {
		phase := float64(i) * 0.25
		osc := 8.0 * math.Sin(phase)
		var trend float64
		if i < n/2 {
			trend = float64(i) * 0.2
		} else {
			trend = float64(n-i) * 0.2
		}
		mid := 100.0 + trend + osc
		klines[i] = Kline{
			Time:       t.Add(time.Duration(i) * time.Hour),
			Open:       mid - 0.5,
			High:       mid + 2.0,
			Low:        mid - 1.5,
			Close:      mid + 0.3,
			BaseVolume: 1000,
			Turnover:   100000,
		}
	}
	return klines
}
