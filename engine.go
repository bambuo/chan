package chanlun

import (
	"fmt"

	"github.com/bambuo/talib"
)

// ──────────────────────────────────────────────
// §9  算法流程总览
// ──────────────────────────────────────────────
//
// Engine 是缠论算法的核心引擎，编排完整的 12 步流水线。
//
// 满足 §12 全部设计约束：
//   - 确定性：同一输入序列产生完全相同输出
//   - 实时性：O(1) 增量更新（engine_inc.go）
//   - 可配置：Config 控制全部参数
//   - 可观测：Result 包含所有中间步骤的输出

// Engine 是缠论算法的主引擎。
type Engine struct {
	config Config
}

// NewEngine 创建一个新的缠论引擎实例。
func NewEngine(config Config) (*Engine, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("chanlun: %w", err)
	}
	return &Engine{config: config}, nil
}

// processInternal 执行完整 12 步算法流水线（无锁，供内部调用）。
func (e *Engine) processInternal(klines []Kline) (*Result, error) {
	if err := ValidateKlines(klines); err != nil {
		return nil, err
	}

	// 步骤 1: K 线包含处理
	merged := MergeKlines(klines)
	if len(merged) < 3 {
		return nil, fmt.Errorf("chanlun: too few klines after inclusion: %d", len(merged))
	}

	// 步骤 2: 客观分型识别 + 成笔分型筛选
	fractals := FindFractals(merged, 1)
	biFractals := FilterFractalsForBi(fractals, e.config.BiMinKLineCount)

	// 步骤 3: 笔的构建
	var bis []Bi
	if len(biFractals) >= 2 {
		bis = BuildBis(merged, biFractals, e.config.BiMinKLineCount, e.config.NewBiMinPriceRatio)
	}

	// 步骤 4: 笔的包含处理
	var mergedBis []MergedBi
	if len(bis) >= 2 {
		if e.config.EnableBiInclusion {
			mergedBis = MergeBis(bis)
		} else {
			mergedBis = make([]MergedBi, len(bis))
			for i, b := range bis {
				mergedBis[i] = MergedBi{Bi: b, OriginalCount: 1}
			}
		}
	}

	// 步骤 5+6: 线段划分
	segments := BuildSegments(mergedBis)

	// 步骤 7: 中枢识别
	pivots := FindPivots(segments)

	// 步骤 8: 走势类型分类
	trends := ClassifyTrends(pivots)

	// 步骤 9: 多级别联立（预留）

	// 步骤 10: 背驰检测
	var deviations []Deviation
	var trendDeviations []Deviation
	if len(merged) > e.config.MACDSlowPeriod {
		// MACD 在已合并 K 线上计算，保持索引空间与 segments/fractals/bis 一致
		closePrices := extractClose(merged)
		macdResult, err := talib.MACD(closePrices, e.config.MACDFastPeriod,
			e.config.MACDSlowPeriod, e.config.MACDSignalPeriod)
		if err == nil && macdResult != nil {
			deviations = DetectDeviations(segments, macdResult.MACD, macdResult.Signal, macdResult.Histogram)
			trendDeviations = DetectTrendDeviations(segments, pivots, trends,
				macdResult.MACD, macdResult.Signal, macdResult.Histogram)
		}
	}
	allDeviations := append(deviations, trendDeviations...)

	// 用背驰检测结果更新走势完成状态（走势必完美定理）
	UpdateTrendsWithDeviations(trends, trendDeviations)

	// 步骤 11: 买卖点判定
	signals := DetectSignals(trends, allDeviations, pivots, segments)

	// 步骤 12: 信号强度评分
	liquidityData := extractLiquidity(merged)
	closePrices := extractClose(merged)
	for i := range signals {
		score, _ := ScoreSignal(&ScoringContext{
			Signal:          signals[i],
			Deviations:      allDeviations,
			Pivots:          pivots,
			MultiLevelCount: 1,
			LiquidityData:   liquidityData,
			ClosePrices:     closePrices,
		})
		signals[i].Strength = score
	}

	return &Result{
		MergedKlines: merged,
		Fractals:     fractals,
		BiFractals:   biFractals,
		Bis:          bis,
		MergedBis:    mergedBis,
		Segments:     segments,
		Pivots:       pivots,
		Trends:       trends,
		Deviations:   allDeviations,
		Signals:      signals,
	}, nil
}

// extractClose 从 K 线序列中提取收盘价序列。
func extractClose(klines []Kline) []float64 {
	prices := make([]float64, len(klines))
	for i, c := range klines {
		prices[i] = c.Close
	}
	return prices
}

// extractLiquidity 从 Kline 序列中提取更能代表流动性的成交额口径。
func extractLiquidity(klines []Kline) []float64 {
	volumes := make([]float64, len(klines))
	for i, c := range klines {
		switch {
		case c.Turnover > 0:
			volumes[i] = c.Turnover
		case c.QuoteVolume > 0:
			volumes[i] = c.QuoteVolume
		default:
			volumes[i] = c.BaseVolume
		}
	}
	return volumes
}
