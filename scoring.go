package chanlun

// ──────────────────────────────────────────────
// §11  信号强度评分
// ──────────────────────────────────────────────
//
// 综合五项因子，输出 0~1 的置信度评分：
//   | 因子         | 权重 | 说明                          |
//   |-------------|------|-------------------------------|
//   | 级别大小     | 0.30 | 级别越大，信号越可靠            |
//   | 背驰力度     | 0.25 | 力度差异越大，信号越可靠         |
//   | 多级别共振   | 0.20 | 多个级别同时出信号加分           |
//   | 流动性配合   | 0.15 | 成交额/盘口/滑点支持信号时加分    |
//   | 中枢位置     | 0.10 | 中枢边界附近的信号更可靠          |

// ScoringContext 为信号评分提供上下文。
type ScoringContext struct {
	Signal          Signal      `json:"signal"`
	Deviations      []Deviation `json:"deviations,omitempty"`
	Pivots          []Pivot     `json:"pivots,omitempty"`
	LiquidityData   []float64   `json:"liquidityData,omitempty"`
	MultiLevelCount int         `json:"multiLevelCount"`
	ClosePrices     []float64   `json:"closePrices,omitempty"`
}

// ScoreFactors 包含各项评分因子。
type ScoreFactors struct {
	LevelScore     float64 `json:"levelScore"`
	DeviationScore float64 `json:"deviationScore"`
	ResonanceScore float64 `json:"resonanceScore"`
	LiquidityScore float64 `json:"liquidityScore"`
	PositionScore  float64 `json:"positionScore"`
}

// ScoreSignal 对单个信号进行综合评分。
func ScoreSignal(ctx *ScoringContext) (float64, ScoreFactors) {
	if ctx == nil {
		return 0, ScoreFactors{}
	}

	factors := ScoreFactors{}

	// 1. 级别因子 (权重 0.30)
	factors.LevelScore = scoreLevel(ctx.Signal.Level, ctx.Signal.Deviation)

	// 2. 背驰力度因子 (权重 0.25)
	factors.DeviationScore = scoreDeviationForce(ctx.Signal.Deviation)

	// 3. 多周期共振因子 (权重 0.20)
	factors.ResonanceScore = scoreResonance(ctx.MultiLevelCount)

	// 4. 流动性因子 (权重 0.15)
	factors.LiquidityScore = scoreLiquidity(ctx.Signal, ctx.LiquidityData)

	// 5. 中枢位置因子 (权重 0.10)
	factors.PositionScore = scorePosition(ctx.Signal, ctx.Pivots)

	// 加权总分
	total := factors.LevelScore*0.30 +
		factors.DeviationScore*0.25 +
		factors.ResonanceScore*0.20 +
		factors.LiquidityScore*0.15 +
		factors.PositionScore*0.10

	return total, factors
}

// scoreLevel 评估级别因子的可靠性。
func scoreLevel(level string, dev *Deviation) float64 {
	if dev == nil {
		return 0.3
	}
	switch dev.Level {
	case TrendDeviation:
		return 1.0 // 走势背驰，最可靠
	case SegmentDeviation:
		return 0.7 // 线段背驰，中等可靠
	case BiDeviation:
		return 0.3 // 笔背驰，信号较弱
	default:
		return 0.3
	}
}

// scoreDeviationForce 评估背驰力度差异。
func scoreDeviationForce(dev *Deviation) float64 {
	if dev == nil {
		return 0
	}

	// 力度差异比例
	forceDiff := dev.ForceBefore - dev.ForceAfter
	if forceDiff <= 0 || dev.ForceBefore <= 0 {
		return 0
	}

	ratio := forceDiff / dev.ForceBefore
	if ratio >= 0.5 {
		return 1.0 // 力度减半以上，信号极强
	}
	if ratio >= 0.3 {
		return 0.8
	}
	if ratio >= 0.1 {
		return 0.5
	}
	return 0.2
}

// scoreResonance 评估多周期共振强度。
func scoreResonance(count int) float64 {
	switch {
	case count >= 3:
		return 1.0 // 三个级别共振，极强
	case count == 2:
		return 0.7 // 两个级别共振
	case count == 1:
		return 0.3 // 单级别信号
	default:
		return 0
	}
}

// scoreLiquidity 评估流动性配合程度。
func scoreLiquidity(signal Signal, liquidity []float64) float64 {
	if len(liquidity) == 0 || signal.Index < 0 || signal.Index >= len(liquidity) {
		return 0.5 // 无流动性数据时返回中性
	}

	idx := signal.Index
	window := 5 // 取前后各 5 根 K 线的平均成交量

	signalVol := liquidity[idx]

	// 前 window 根的平均成交量
	sumBefore := 0.0
	count := 0
	for i := maxInt(0, idx-window); i < idx; i++ {
		sumBefore += liquidity[i]
		count++
	}
	if count == 0 {
		return 0.5
	}
	avgBefore := sumBefore / float64(count)

	if avgBefore <= 0 {
		return 0.5
	}

	ratio := signalVol / avgBefore
	if ratio >= 1.5 {
		return 1.0 // 放量 50%+
	}
	if ratio >= 1.2 {
		return 0.8 // 放量 20%+
	}
	if ratio >= 1.0 {
		return 0.6 // 略微放量
	}
	return 0.3 // 缩量，可靠性降低
}

// scorePosition 评估信号在中枢附近的位置可靠性。
// 中枢边界附近的信号更可靠。
func scorePosition(signal Signal, pivots []Pivot) float64 {
	if len(pivots) == 0 {
		return 0.5
	}

	switch signal.Type {
	case BuyPoint1, BuyPoint2, BuyPoint3:
		// 买点应在中枢下沿（ZD）附近或下方
		for _, p := range pivots {
			if signal.Price <= p.ZD {
				return 1.0
			}
			if signal.Price <= p.ZD+(p.ZG-p.ZD)*0.3 {
				return 0.8
			}
		}
	case SellPoint1, SellPoint2, SellPoint3:
		// 卖点应在中枢上沿（ZG）附近或上方
		for _, p := range pivots {
			if signal.Price >= p.ZG {
				return 1.0
			}
			if signal.Price >= p.ZG-(p.ZG-p.ZD)*0.3 {
				return 0.8
			}
		}
	}

	return 0.5
}
