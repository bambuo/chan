package scoring

import "github.com/bambuo/chan/types"

// positionLowRatio 买入信号位于中枢 ZD 附近的比率阈值。
const positionLowRatio = 0.3

// ScoringContext 为信号评分提供上下文。
type ScoringContext struct {
	Signal          types.Signal
	Deviations      []types.Deviation
	Pivots          []types.Pivot
	LiquidityData   []float64
	MultiLevelCount int
	ClosePrices     []float64
	Weights         *types.ScoreWeights // 评分权重，nil 时使用默认值
}

// ScoreFactors 包含各项评分因子。
type ScoreFactors struct {
	LevelScore     float64
	DeviationScore float64
	ResonanceScore float64
	LiquidityScore float64
	PositionScore  float64
}

// ScoreSignal 对单个信号进行综合评分，返回 0~1 的置信度。
func ScoreSignal(ctx *ScoringContext) (float64, ScoreFactors) {
	if ctx == nil {
		return 0, ScoreFactors{}
	}
	w := defaultWeights()
	if ctx.Weights != nil {
		w = *ctx.Weights
	}
	var f ScoreFactors
	f.LevelScore = levelScore(ctx.Signal.Level, ctx.Signal.Deviation)
	f.DeviationScore = devForceScore(ctx.Signal.Deviation)
	f.ResonanceScore = resonanceScore(ctx.MultiLevelCount)
	f.LiquidityScore = liquidityScore(ctx.Signal, ctx.LiquidityData)
	f.PositionScore = positionScore(ctx.Signal, ctx.Pivots)
	total := f.LevelScore*w.Level + f.DeviationScore*w.Deviation +
		f.ResonanceScore*w.Resonance + f.LiquidityScore*w.Liquidity +
		f.PositionScore*w.Position
	return total, f
}

func defaultWeights() types.ScoreWeights {
	return types.ScoreWeights{
		Level:     0.30,
		Deviation: 0.25,
		Resonance: 0.20,
		Liquidity: 0.15,
		Position:  0.10,
	}
}

func levelScore(_ string, dev *types.Deviation) float64 {
	if dev == nil {
		return 0.3
	}
	switch dev.Level {
	case types.TrendDeviation:
		return 1.0
	case types.SegmentDeviation:
		return 0.7
	default:
		return 0.3
	}
}

func devForceScore(dev *types.Deviation) float64 {
	if dev == nil || dev.ForceBefore <= 0 {
		return 0
	}
	diff := dev.ForceBefore - dev.ForceAfter
	if diff <= 0 {
		return 0
	}
	r := diff / dev.ForceBefore
	switch {
	case r >= 0.5:
		return 1.0
	case r >= 0.3:
		return 0.8
	case r >= 0.1:
		return 0.5
	default:
		return 0.2
	}
}

func resonanceScore(n int) float64 {
	switch {
	case n >= 3:
		return 1.0
	case n == 2:
		return 0.7
	case n == 1:
		return 0.3
	default:
		return 0
	}
}

// liquidityScore 评估信号发生位置的流动性强度。
// data 为合并后 K 线的流动性序列（成交额/成交量），sig.Index 是合并 K 线索引空间
// （笔/线段的 StartIndex/EndIndex 均基于合并后 K 线，故索引空间对齐）。
// 越界或数据不足时返回中性值 0.5，避免掩盖真实差异。
func liquidityScore(sig types.Signal, data []float64) float64 {
	if len(data) == 0 || sig.Index < 0 || sig.Index >= len(data) {
		return 0.5
	}
	idx := sig.Index
	vol := data[idx]
	sum := 0.0
	cnt := 0
	for i := max(0, idx-5); i < idx; i++ {
		sum += data[i]
		cnt++
	}
	if cnt == 0 {
		return 0.5
	}
	avg := sum / float64(cnt)
	if avg <= 0 {
		return 0.5
	}
	r := vol / avg
	switch {
	case r >= 1.5:
		return 1.0
	case r >= 1.2:
		return 0.8
	case r >= 1.0:
		return 0.6
	default:
		return 0.3
	}
}

func positionScore(sig types.Signal, pivots []types.Pivot) float64 {
	if len(pivots) == 0 {
		return 0.5
	}
	// 只评估关联中枢（若有）或最后一个中枢，避免任意中枢误打分
	var p *types.Pivot
	if sig.Pivot != nil {
		p = sig.Pivot
	} else {
		p = &pivots[len(pivots)-1]
	}
	height := p.ZG - p.ZD
	switch sig.Type {
	case types.BuyPoint1, types.BuyPoint2, types.BuyPoint3:
		if sig.Price <= p.ZD {
			return 1.0
		}
		if height > 0 && sig.Price <= p.ZD+height*positionLowRatio {
			return 0.8
		}
	case types.SellPoint1, types.SellPoint2, types.SellPoint3:
		if sig.Price >= p.ZG {
			return 1.0
		}
		if height > 0 && sig.Price >= p.ZG-height*positionLowRatio {
			return 0.8
		}
	}
	return 0.5
}
