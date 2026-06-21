package deviation

import (
	"math"

	"github.com/bambuo/chan/force"
	"github.com/bambuo/chan/types"
)

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

// DetectDeviations 基于笔级中枢检测背驰。
func DetectDeviations(pivots []types.Pivot, macdHist, macdDif, volumes, turnovers, closes []float64, config types.Config) []types.Deviation {
	if len(pivots) == 0 {
		return nil
	}
	rate := config.BspDivergenceRate
	sentinel := math.IsInf(rate, 1) || rate > 100
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
		inM := force.CalcMetric(*zs.BiIn, metric, false, macdHist, macdDif, volumes, turnovers, closes)
		outM := force.CalcMetric(*zs.BiOut, metric, true, macdHist, macdDif, volumes, turnovers, closes)
		diver := false
		if sentinel {
			diver = true
		} else if inM > 1e-12 {
			diver = outM <= rate*inM
		}
		if !diver {
			continue
		}
		devs = append(devs, types.Deviation{
			Type: "bi_pivot", Level: types.BiDeviation, Direction: zs.BiOut.Direction,
			SegBeforeIdx: i, SegAfterIdx: i, PriceHigh: zs.BiOut.EndPrice,
			ForceBefore: inM, ForceAfter: outM,
			MACDAreaBefore: fullArea(*zs.BiIn, macdHist), MACDAreaAfter: fullArea(*zs.BiOut, macdHist),
			MACDDiffBefore: diffRange(*zs.BiIn, macdHist), MACDDiffAfter: diffRange(*zs.BiOut, macdHist),
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
	sentinel := math.IsInf(rate, 1) || rate > 100
	var devs []types.Deviation
	for _, trend := range trends {
		if len(trend.Pivots) < 2 {
			continue
		}
		bP := &pivots[len(pivots)-1]
		if bP.BiOut == nil {
			continue
		}
		var aBi *types.Bi
		for i := range pivots {
			if pivots[i].BiIn != nil {
				aBi = pivots[i].BiIn
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
		aM := force.CalcMetric(*aBi, metric, false, macdHist, macdDif, volumes, turnovers, closes)
		cM := force.CalcMetric(*cBi, metric, true, macdHist, macdDif, volumes, turnovers, closes)
		diver := false
		if sentinel {
			diver = true
		} else if aM > 1e-12 {
			diver = cM <= rate*aM
		}
		if !diver {
			continue
		}
		devs = append(devs, types.Deviation{
			Type: "bi_trend", Level: types.TrendDeviation, Direction: cBi.Direction,
			PriceHigh: cBi.EndPrice, ForceBefore: aM, ForceAfter: cM,
			MACDAreaBefore: fullArea(*aBi, macdHist), MACDAreaAfter: fullArea(*cBi, macdHist),
			MACDDiffBefore: diffRange(*aBi, macdHist), MACDDiffAfter: diffRange(*cBi, macdHist),
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
	sentinel := math.IsInf(rate, 1) || rate > 100
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
		inM := force.CalcMetric(*zs.BiIn, metric, false, macdHist, macdDif, volumes, turnovers, closes)
		outM := force.CalcMetric(*zs.BiOut, metric, true, macdHist, macdDif, volumes, turnovers, closes)
		diver := false
		if sentinel {
			diver = true
		} else if inM > 1e-12 {
			diver = outM <= rate*inM
		}
		if !diver {
			continue
		}
		devs = append(devs, types.Deviation{
			Type: "seg_pivot", Level: types.SegmentDeviation, Direction: zs.BiOut.Direction,
			SegBeforeIdx: i, SegAfterIdx: i, PriceHigh: zs.BiOut.EndPrice,
			ForceBefore: inM, ForceAfter: outM,
			MACDAreaBefore: fullArea(*zs.BiIn, macdHist), MACDAreaAfter: fullArea(*zs.BiOut, macdHist),
			MACDDiffBefore: diffRange(*zs.BiIn, macdHist), MACDDiffAfter: diffRange(*zs.BiOut, macdHist),
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

func fullArea(bi types.Bi, hist []float64) float64 {
	s := 1e-7
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(hist); i++ {
		if i < 0 {
			continue
		}
		v := hist[i]
		if (bi.Direction == types.DirDown && v < 0) || (bi.Direction == types.DirUp && v > 0) {
			s += math.Abs(v)
		}
	}
	return s
}

func diffRange(bi types.Bi, hist []float64) float64 {
	mx, mn := math.Inf(-1), math.Inf(1)
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(hist); i++ {
		if i < 0 {
			continue
		}
		v := hist[i]
		if v > mx {
			mx = v
		}
		if v < mn {
			mn = v
		}
	}
	if math.IsInf(mx, -1) || math.IsInf(mn, 1) {
		return 0
	}
	return mx - mn
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
