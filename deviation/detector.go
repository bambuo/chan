package deviation

import (
	"math"

	"github.com/bambuo/chan/force"
	"github.com/bambuo/chan/types"
)

// isSentinel 判断背驰率是否为哨兵值（关闭背驰率过滤）。
func isSentinel(rate float64) bool {
	return math.IsInf(rate, 1) || rate > types.DivergenceSentinelThreshold
}

// metricForSide 根据偏差方向选择买/卖侧的力度算法。
// 向下笔→买侧，向上笔→卖侧。
func metricForSide(bi *types.Bi, config types.Config) force.MetricType {
	if bi == nil {
		return force.ParseMetric(config.BspMacdAlgo)
	}
	if bi.IsDown() {
		// 买入侧：使用 BspMacdAlgoBuy
		algo := config.BspMacdAlgoBuy
		if algo == "" {
			algo = config.BspMacdAlgo
		}
		return force.ParseMetric(algo)
	}
	// 卖出侧：使用 BspMacdAlgoSell
	algo := config.BspMacdAlgoSell
	if algo == "" {
		algo = config.BspMacdAlgo
	}
	return force.ParseMetric(algo)
}

// isDivergence 判定力度是否构成背驰。
// 纯函数：sentinel 关闭倍率过滤，但仍要求后段力度小于前段力度；否则要求 outM <= rate*inM 且 inM>0。
// 对齐 Python ZS.py:171-174 的 divergence_rate > 100 “保送”逻辑（sentinel 关闭比率过滤）。
func isDivergence(inM, outM, rate float64) bool {
	if inM <= types.ForceEpsilon {
		return false
	}
	if isSentinel(rate) {
		return outM < inM
	}
	return outM <= rate*inM
}

// calcForce 根据配置选择力度计算的底层算法。
// 当 metric 为 Rsi 时使用 Config.RsiCycle 作为周期，否则回退到 CalcMetric。
// 纯函数，不修改任何状态。
func calcForce(bi types.Bi, metric force.MetricType, isReverse bool,
	macdHist, macdDif, volumes, turnovers, closes []float64, cfg types.Config) float64 {
	if metric == force.Rsi {
		period := cfg.RsiCycle
		if period <= 0 {
			period = 14 // 兜底标准周期
		}
		return force.CalcRSIMetric(bi, period, closes)
	}
	return force.CalcMetric(bi, metric, isReverse, macdHist, macdDif, volumes, turnovers, closes)
}

// DetectDeviations 基于笔级中枢检测背驰。
func DetectDeviations(pivots []types.Pivot, macdHist, macdDif, volumes, turnovers, closes []float64, config types.Config) []types.Deviation {
	if len(pivots) == 0 {
		return nil
	}
	rate := config.BspDivergenceRate
	var devs []types.Deviation
	for i := range pivots {
		zs := &pivots[i]
		if zs.BiIn == nil || zs.BiOut == nil {
			continue
		}
		if !endBiBreak(zs.BiOut, zs.ZG, zs.ZD) {
			continue
		}
		// 按方向选择买/卖算法
		metric := metricForSide(zs.BiOut, config)
		inM := calcForce(*zs.BiIn, metric, false, macdHist, macdDif, volumes, turnovers, closes, config)
		outM := calcForce(*zs.BiOut, metric, true, macdHist, macdDif, volumes, turnovers, closes, config)
		if !isDivergence(inM, outM, rate) {
			continue
		}
		devs = append(devs, types.Deviation{
			Type: "bi_pivot", Level: types.BiDeviation, Direction: zs.BiOut.Direction,
			SegBeforeIdx: i, SegAfterIdx: i, PriceHigh: zs.BiOut.EndPrice,
			ForceBefore: inM, ForceAfter: outM,
			MACDAreaBefore: force.CalcMetric(*zs.BiIn, force.FullArea, false, macdHist, macdDif, volumes, turnovers, closes),
			MACDAreaAfter:  force.CalcMetric(*zs.BiOut, force.FullArea, true, macdHist, macdDif, volumes, turnovers, closes),
			MACDDiffBefore: force.CalcMetric(*zs.BiIn, force.Diff, false, macdHist, macdDif, volumes, turnovers, closes),
			MACDDiffAfter:  force.CalcMetric(*zs.BiOut, force.Diff, true, macdHist, macdDif, volumes, turnovers, closes),
		})
	}
	return devs
}

// DetectTrendDeviations 基于笔级中枢和走势检测趋势背驰（a+A+b+B+c）。
func DetectTrendDeviations(pivots []types.Pivot, trends []types.Trend,
	macdHist, macdDif, volumes, turnovers, closes []float64, config types.Config) []types.Deviation {
	if len(trends) == 0 || len(pivots) == 0 {
		return nil
	}
	rate := config.BspDivergenceRate
	var devs []types.Deviation
	for ti, trend := range trends {
		tp := trend.Pivots
		if len(tp) < 2 {
			continue
		}
		// 取趋势的最后一个中枢作为 B（而非全局 pivots 最后一个）
		bP := &tp[len(tp)-1]
		if bP.BiOut == nil {
			continue
		}
		// 取趋势的第一个中枢的 BiIn 作为 a（而非全局第一个有 BiIn 的中枢）
		var aBi *types.Bi
		for i := range tp {
			if tp[i].BiIn != nil {
				aBi = tp[i].BiIn
				break
			}
		}
		if aBi == nil {
			continue
		}
		cBi := bP.BiOut
		td := trendDir(trend.Type)
		if aBi.Direction != td || cBi.Direction != td {
			continue
		}
		if !endBiBreak(cBi, bP.ZG, bP.ZD) {
			continue
		}
		// 按方向选择买/卖算法
		metric := metricForSide(cBi, config)
		aM := calcForce(*aBi, metric, false, macdHist, macdDif, volumes, turnovers, closes, config)
		cM := calcForce(*cBi, metric, true, macdHist, macdDif, volumes, turnovers, closes, config)
		if !isDivergence(aM, cM, rate) {
			continue
		}
		devs = append(devs, types.Deviation{
			Type: "bi_trend", Level: types.TrendDeviation, Direction: cBi.Direction,
			TrendIndex: ti,
			PriceHigh:  cBi.EndPrice, ForceBefore: aM, ForceAfter: cM,
			MACDAreaBefore: force.CalcMetric(*aBi, force.FullArea, false, macdHist, macdDif, volumes, turnovers, closes),
			MACDAreaAfter:  force.CalcMetric(*cBi, force.FullArea, true, macdHist, macdDif, volumes, turnovers, closes),
			MACDDiffBefore: force.CalcMetric(*aBi, force.Diff, false, macdHist, macdDif, volumes, turnovers, closes),
			MACDDiffAfter:  force.CalcMetric(*cBi, force.Diff, true, macdHist, macdDif, volumes, turnovers, closes),
		})
	}
	return devs
}

// DetectSegmentDeviations 基于线段级中枢检测线段级背驰。
// 逻辑与 DetectDeviations 一致，但作用于线段级中枢和线段（作为高级别笔）。
func DetectSegmentDeviations(segPivots []types.Pivot, segs []types.MergedBi,
	macdHist, macdDif, volumes, turnovers, closes []float64, config types.Config) []types.Deviation {
	if len(segPivots) == 0 {
		return nil
	}
	rate := config.BspDivergenceRate
	var devs []types.Deviation
	for i := range segPivots {
		zs := &segPivots[i]
		if zs.BiIn == nil || zs.BiOut == nil {
			continue
		}
		if !endBiBreak(zs.BiOut, zs.ZG, zs.ZD) {
			continue
		}
		metric := metricForSide(zs.BiOut, config)
		inM := calcForce(*zs.BiIn, metric, false, macdHist, macdDif, volumes, turnovers, closes, config)
		outM := calcForce(*zs.BiOut, metric, true, macdHist, macdDif, volumes, turnovers, closes, config)
		if !isDivergence(inM, outM, rate) {
			continue
		}
		devs = append(devs, types.Deviation{
			Type: "seg_pivot", Level: types.SegmentDeviation, Direction: zs.BiOut.Direction,
			SegBeforeIdx: i, SegAfterIdx: i, PriceHigh: zs.BiOut.EndPrice,
			ForceBefore: inM, ForceAfter: outM,
			MACDAreaBefore: force.CalcMetric(*zs.BiIn, force.FullArea, false, macdHist, macdDif, volumes, turnovers, closes),
			MACDAreaAfter:  force.CalcMetric(*zs.BiOut, force.FullArea, true, macdHist, macdDif, volumes, turnovers, closes),
			MACDDiffBefore: force.CalcMetric(*zs.BiIn, force.Diff, false, macdHist, macdDif, volumes, turnovers, closes),
			MACDDiffAfter:  force.CalcMetric(*zs.BiOut, force.Diff, true, macdHist, macdDif, volumes, turnovers, closes),
		})
	}
	return devs
}

func endBiBreak(out *types.Bi, zg, zd float64) bool {
	if out == nil {
		return false
	}
	if out.IsDown() {
		return out.ZSLow() < zd
	}
	return out.IsUp() && out.ZSHigh() > zg
}

func trendDir(t types.TrendType) types.Direction {
	switch t {
	case types.TrendUp:
		return types.DirUp
	case types.TrendDown:
		return types.DirDown
	default:
		return types.DirNone
	}
}
