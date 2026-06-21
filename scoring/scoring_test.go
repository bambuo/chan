package scoring

import (
	"testing"

	"github.com/bambuo/chan/types"
)

func TestScoreSignal_DefaultWeights(t *testing.T) {
	ctx := &ScoringContext{
		Signal: types.Signal{
			Type: types.BuyPoint1, SubType: types.SubT1,
			Index: 10, Price: 100.0, Strength: 0,
		},
		Deviations: []types.Deviation{
			{
				Level:       types.TrendDeviation,
				Direction:   types.DirDown,
				ForceBefore: 100,
				ForceAfter:  40,
				PriceHigh:   100,
			},
		},
		Pivots:          []types.Pivot{{ZD: 90, ZG: 110}},
		LiquidityData:   []float64{1000, 1100, 1200, 1300, 1400, 1500},
		MultiLevelCount: 2,
		ClosePrices:     []float64{95, 96, 97, 98, 99, 100},
	}
	// 关联偏差到信号
	ctx.Signal.Deviation = &ctx.Deviations[0]

	score, factors := ScoreSignal(ctx)
	if score <= 0 || score > 1 {
		t.Errorf("expected score in (0,1], got %f", score)
	}
	t.Logf("score=%.4f factors=%+v", score, factors)

	// 趋势背驰应有高分
	if score < 0.5 {
		t.Errorf("趋势背驰信号的评分应 >= 0.5, got %f", score)
	}
}

func TestScoreSignal_NilContext(t *testing.T) {
	score, factors := ScoreSignal(nil)
	if score != 0 {
		t.Errorf("expected 0 for nil ctx, got %f", score)
	}
	if factors != (ScoreFactors{}) {
		t.Errorf("expected empty factors for nil ctx, got %+v", factors)
	}
}

func TestScoreSignal_CustomWeights(t *testing.T) {
	w := types.ScoreWeights{Level: 1.0, Deviation: 0, Resonance: 0, Liquidity: 0, Position: 0}
	ctx := &ScoringContext{
		Signal: types.Signal{
			Type: types.BuyPoint1, SubType: types.SubT1, Index: 10, Price: 100,
			Deviation: &types.Deviation{Level: types.TrendDeviation, ForceBefore: 100, ForceAfter: 30},
		},
		Pivots:          []types.Pivot{{ZD: 90, ZG: 110}},
		LiquidityData:   []float64{1000, 1100, 1200, 1300, 1400, 1500},
		MultiLevelCount: 0,
		Weights:         &w,
	}
	score, _ := ScoreSignal(ctx)
	if score != 1.0 {
		t.Errorf("with Level weight=1.0 and trend deviation, expected 1.0, got %f", score)
	}
}

func TestScoreFactors_AllZeroInput(t *testing.T) {
	ctx := &ScoringContext{
		Signal:          types.Signal{Type: types.BuyPoint1, Index: 0, Price: 100},
		Pivots:          nil,
		LiquidityData:   nil,
		MultiLevelCount: 0,
		ClosePrices:     nil,
	}
	score, factors := ScoreSignal(ctx)
	t.Logf("empty input: score=%.4f factors=%+v", score, factors)
	if score < 0 || score > 1 {
		t.Errorf("score should be in [0,1], got %f", score)
	}
}
