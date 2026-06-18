package chanlun

import "fmt"

// ──────────────────────────────────────────────
// §10  多级别联立分析
// ──────────────────────────────────────────────
//
// 缠论的精髓在于多级别递归分析。
// 级别由走势的递归构筑定义，而非时间周期。
// 但在工程实现中，使用交易所 interval 作为级别的近似观察窗口。
//
// 级别递归链：
//   Level 1 (1F):  K线 → 笔 → 线段 → 中枢 → 走势类型
//   Level 2 (5F):  5F笔 = 1F线段 → 5F线段 = 1F走势类型
//   Level 3 (30F): 30F笔 = 5F线段 → 30F线段 = 5F走势类型
//   ...
//
// MultiLevelCoordinator 管理多个交易所 interval 的同步分析，
// 并执行跨级别的背驰验证（区间套）和信号共振检测。

// IntervalConfig 定义一个交易所 interval 的处理配置。
type IntervalConfig struct {
	Interval       string `json:"interval"`
	RelativeFactor int    `json:"relativeFactor"`
	Config         Config `json:"config"`
}

// LevelResult 存储单个交易所 interval 的分析结果。
type LevelResult struct {
	Interval string  `json:"interval"`
	Klines   []Kline `json:"klines,omitempty"`
	Result   *Result `json:"result,omitempty"`
}

// MultiLevelResult 包含多级别联立的完整分析结果。
type MultiLevelResult struct {
	Levels     []LevelResult          `json:"levels"`
	Resonance  int                    `json:"resonance"`
	Deviations []Deviation            `json:"deviations,omitempty"`
	Signals    []Signal               `json:"signals,omitempty"`
	Nesting    *IntervalNestingResult `json:"nesting,omitempty"`
}

// MultiLevelCoordinator 是多级别联立分析的协调器。
type MultiLevelCoordinator struct {
	intervals []IntervalConfig
}

// NewMultiLevelCoordinator 创建多级别联立协调器。
// intervals 按从高到低（大 interval → 小 interval）排序。
func NewMultiLevelCoordinator(intervals []IntervalConfig) *MultiLevelCoordinator {
	return &MultiLevelCoordinator{intervals: intervals}
}

// DefaultIntervals 返回默认的多级别 interval 配置（1h/30m/5m）。
func DefaultIntervals() []IntervalConfig {
	return []IntervalConfig{
		{
			Interval:       "1h",
			RelativeFactor: 12,
			Config:         DefaultConfig(),
		},
		{
			Interval:       "30m",
			RelativeFactor: 6,
			Config:         DefaultConfig(),
		},
		{
			Interval:       "5m",
			RelativeFactor: 1,
			Config:         DefaultConfig(),
		},
	}
}

// Analyse 对多个交易所 interval 的 Kline 数据执行多级别联立分析。
// data map 的 key 是交易所 interval（如 "1h", "30m", "5m"），
// value 是对应 interval 的 Kline 序列。
func (mc *MultiLevelCoordinator) Analyse(data map[string][]Kline) (*MultiLevelResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("chanlun multi: no data provided")
	}

	result := &MultiLevelResult{
		Levels: make([]LevelResult, 0, len(mc.intervals)),
	}

	// 第一步：逐级别独立运行完整分析
	for _, intervalCfg := range mc.intervals {
		klines, ok := data[intervalCfg.Interval]
		if !ok || len(klines) == 0 {
			continue
		}

		engine, err := NewEngine(intervalCfg.Config)
		if err != nil {
			continue
		}
		r, err := engine.Process(klines)
		if err != nil {
			// 某个级别数据不足时跳过，不影响其他级别
			continue
		}

		result.Levels = append(result.Levels, LevelResult{
			Interval: intervalCfg.Interval,
			Klines:   klines,
			Result:   r,
		})
	}

	if len(result.Levels) == 0 {
		return nil, fmt.Errorf("chanlun multi: no level produced results")
	}

	// 第二步：跨级别背驰共振检测
	result.Deviations = detectCrossLevelDeviation(result.Levels)

	// 第三步：区间套定位（从最高级别逐级下钻）
	result.Nesting = performMultiLevelNesting(result.Levels)

	// 第四步：生成跨级别确认的买卖点信号
	result.Signals = mergeMultiLevelSignals(result.Levels, result.Deviations)
	result.Resonance = countResonance(result.Signals)

	return result, nil
}

// detectCrossLevelDeviation 检测跨级别背驰共振。
// 当多个级别在同一时间段附近出现同向背驰时，产生共振信号。
func detectCrossLevelDeviation(levels []LevelResult) []Deviation {
	if len(levels) < 2 {
		return nil
	}

	resonantDeviations := make([]Deviation, 0)

	// 取最高级别（大周期）的背驰作为基准
	topLevel := levels[0]
	if len(topLevel.Result.Deviations) == 0 {
		return nil
	}

	// 对高级别的每个背驰，检查低级别是否有同向背驰
	for _, topDev := range topLevel.Result.Deviations {
		if topDev.SegmentAfter == nil {
			continue
		}

		// 计算高级别背驰的时间范围（K线索引）
		topStart := topDev.SegmentAfter.StartIndex
		topEnd := topDev.SegmentAfter.EndIndex

		// 在低级别中寻找同向背驰
		confirmed := false
		for _, level := range levels[1:] {
			for _, dev := range level.Result.Deviations {
				if dev.SegmentAfter == nil || dev.Direction != topDev.Direction {
					continue
				}

				// 检查低级别背驰的时间范围是否在高级别范围内
				devStart := dev.SegmentAfter.StartIndex
				devEnd := dev.SegmentAfter.EndIndex

				// 使用比例缩放来匹配不同时间周期的K线数量
				// 简化处理：低级别背驰位置应在高级别背驰范围内
				if devStart >= topStart && devEnd <= topEnd {
					confirmed = true
					break
				}
			}
		}

		if confirmed {
			resonantDeviations = append(resonantDeviations, topDev)
		}
	}

	return resonantDeviations
}

// performMultiLevelNesting 执行多级别区间套定位。
// 从最高级别开始，逐级下钻精确定位买卖点。
func performMultiLevelNesting(levels []LevelResult) *IntervalNestingResult {
	if len(levels) < 2 {
		return nil
	}

	// 构建区间套数据提供者
	provider := &MultiLevelDataProvider{
		Levels: make([]string, 0, len(levels)),
		Data:   make(map[string]LevelData),
	}

	for _, l := range levels {
		if l.Result == nil {
			continue
		}
		provider.Levels = append(provider.Levels, l.Interval)

		// 尝试从k线计算MACD
		if len(l.Klines) > 26 {
			macd, signal, hist, err := CalculateMACD(
				extractClosePrices(l.Klines),
				12, 26, 9,
			)
			if err == nil {
				provider.Data[l.Interval] = LevelData{
					Segments:   l.Result.Segments,
					MACD:       macd,
					MACDSignal: signal,
					MACDHist:   hist,
				}
			}
		}
	}

	if len(provider.Levels) < 2 {
		return nil
	}

	return PerformIntervalNesting(provider)
}

// mergeMultiLevelSignals 合并多级别信号。
// 保留跨级别确认的信号，并标记共振强度。
func mergeMultiLevelSignals(levels []LevelResult, crossDeviations []Deviation) []Signal {
	// 收集所有级别的信号
	signalMap := make(map[SignalType]Signal)

	for _, level := range levels {
		if level.Result == nil {
			continue
		}
		for _, sig := range level.Result.Signals {
			key := sig.Type
			existing, exists := signalMap[key]
			if !exists || sig.Strength > existing.Strength {
				signalMap[key] = sig
			}
		}
	}

	// 汇总
	signals := make([]Signal, 0, len(signalMap))
	for _, sig := range signalMap {
		// 如果背驰被跨级别确认，增强信号
		for _, dev := range crossDeviations {
			if sig.Deviation != nil && sig.Deviation.Direction == dev.Direction {
				sig.Strength = minFloat(sig.Strength+0.15, 1.0)
			}
		}
		signals = append(signals, sig)
	}

	return signals
}

// countResonance 统计共振信号数。
func countResonance(signals []Signal) int {
	if len(signals) == 0 {
		return 0
	}
	count := 0
	for _, s := range signals {
		if s.Strength >= 0.7 {
			count++
		}
	}
	return count
}

// extractClosePrices 从 K 线提取收盘价（与 engine.go 中的 extractClose 类似）。
func extractClosePrices(klines []Kline) []float64 {
	prices := make([]float64, len(klines))
	for i, c := range klines {
		prices[i] = c.Close
	}
	return prices
}
