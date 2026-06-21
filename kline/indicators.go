package kline

import (
	"math"

	"github.com/bambuo/chan/types"
	"github.com/bambuo/talib"
)

// CalcIndicators 对 K 线序列计算技术指标（BOLL/RSI/KDJ/MA）。
// 底层使用 talib 库，结果与 TradingView / Python TA-Lib 对齐。
func CalcIndicators(klines []types.Kline, cfg types.Config) {
	if len(klines) == 0 {
		return
	}
	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	// ── 均线 MA (SMA) ──
	if len(cfg.MeanMetrics) > 0 {
		for _, n := range cfg.MeanMetrics {
			if n <= 0 {
				continue
			}
			ma, err := talib.SMA(closes, n)
			if err != nil {
				continue
			}
			for i := range klines {
				if i < n-1 || math.IsNaN(ma[i]) {
					continue
				}
				if klines[i].MA == nil {
					klines[i].MA = make(map[int]float64)
				}
				klines[i].MA[n] = ma[i]
			}
		}
	}

	// ── 布林带 BOLL ──
	if cfg.CalBoll {
		bb, err := talib.BBands(closes, cfg.BollN, 2.0, 2.0, talib.MASMA)
		if err == nil {
			for i := range klines {
				if i < cfg.BollN-1 || math.IsNaN(bb.Middle[i]) {
					continue
				}
				klines[i].BOLL = &types.BOLLValue{
					Upper: bb.Upper[i],
					Mid:   bb.Middle[i],
					Lower: bb.Lower[i],
				}
			}
		}
	}

	// ── RSI ──
	if cfg.CalRsi {
		rsi, err := talib.RSI(closes, cfg.RsiCycle)
		if err == nil {
			for i := range klines {
				if i < cfg.RsiCycle || math.IsNaN(rsi[i]) {
					continue
				}
				klines[i].RSI = &rsi[i]
			}
		}
	}

	// ── KDJ（基于 talib STOCH，对齐主流实现）──
	//   K = SMA of Raw %K (slowKPeriod=3)
	//   D = SMA of K  (slowDPeriod=3)
	//   J = 3*K - 2*D
	if cfg.CalKdj {
		stoch, err := talib.STOCH(highs, lows, closes, cfg.KdjCycle, 3, 3, talib.MASMA)
		if err == nil {
			for i := range klines {
				if i < cfg.KdjCycle+5 || math.IsNaN(stoch.K[i]) || math.IsNaN(stoch.D[i]) {
					continue
				}
				j := 3*stoch.K[i] - 2*stoch.D[i]
				klines[i].KDJ = &types.KDJValue{K: stoch.K[i], D: stoch.D[i], J: j}
			}
		}
	}
}
