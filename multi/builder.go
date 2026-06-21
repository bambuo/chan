package multi

import (
	"math"

	"github.com/bambuo/chan/bi"
	"github.com/bambuo/chan/deviation"
	"github.com/bambuo/chan/fractal"
	"github.com/bambuo/chan/kline"
	"github.com/bambuo/chan/pivot"
	"github.com/bambuo/chan/segment"
	"github.com/bambuo/chan/signal"
	"github.com/bambuo/chan/trend"
	"github.com/bambuo/chan/types"
	"github.com/bambuo/talib"
)

// ──────────────────────────────────────────────
// N 层链式递归构建
// ──────────────────────────────────────────────

// BuildMultiLevelChain 从最低级别到最高级别递归构建多级别缠论结构。
//
// levels 必须按时间周期从低到高排列，如 [{1m, klines1m}, {5m, klines5m}, {15m, klines15m}]。
// 至少需要 1 个级别；仅 1 个级别时返回单级别结果，LevelCount() 返回 1。
//
// 核心递归规则：
//
//	L(n+1) 笔   = L(n) 线段
//	L(n+1) 线段 = L(n) 走势类型（工程近似：由 L(n+1) 笔构筑的线段）
func BuildMultiLevelChain(levels []LevelInput, cfg types.Config) *MultiLevelResult {
	if len(levels) == 0 {
		return nil
	}

	results := make([]LevelResult, 0, len(levels))

	// Level 0: 最低级别 — 直接对原始 K 线做完整分析
	baseResult := runFullAnalysis(levels[0].Klines, cfg)
	if baseResult == nil {
		return nil
	}
	results = append(results, LevelResult{
		Interval: levels[0].Interval,
		Klines:   levels[0].Klines,
		Result:   baseResult,
	})

	// Level i>0: 从上一级别的线段递归构建
	for i := 1; i < len(levels); i++ {
		if len(levels[i].Klines) < 3 {
			break
		}
		prev := &results[i-1]
		nextResult := buildLevelFromLower(
			prev.Result,
			prev.Klines,
			levels[i].Klines,
			cfg,
		)
		if nextResult == nil {
			break
		}
		results = append(results, LevelResult{
			Interval: levels[i].Interval,
			Klines:   levels[i].Klines,
			Result:   nextResult,
		})
	}

	mr := &MultiLevelResult{
		Levels:    results,
		Resonance: calcTotalResonance(results, cfg.ResonanceTolerance),
	}

	if len(results) >= 2 {
		mr.Nesting = BuildIntervalNesting(mr)
	}

	return mr
}

// runFullAnalysis 对单级别 K 线执行完整缠论分析。
func runFullAnalysis(klines []types.Kline, cfg types.Config) *types.Result {
	if len(klines) < 3 {
		return nil
	}
	merged := kline.MergeKlines(klines, cfg.Inclusion)
	fractals := fractal.FindFractals(merged)
	if len(fractals) < 2 {
		return nil
	}
	bf := fractal.FilterForBi(fractals, cfg.BiMinKLineCount)
	bis := bi.BuildBis(merged, bf, cfg)

	var mergedBis []types.MergedBi
	if cfg.EnableBiInclusion && len(bis) >= 2 {
		mergedBis = bi.MergeBis(bis)
	} else {
		mergedBis = make([]types.MergedBi, len(bis))
		for i, b := range bis {
			mergedBis[i] = types.MergedBi{Bi: b, OriginalCount: 1}
		}
	}

	segs := segment.BuildWithConfig(mergedBis, cfg.SegAlgo, cfg.LeftSegMethod)
	pivots := pivot.FindBiPivots(mergedBis, segs, cfg)
	pivot.UpdateZSInSeg(mergedBis, pivots)
	trends := trend.ClassifyTrends(pivots)
	macdHist, macdDif := calcMACD(merged, cfg)

	var deviations []types.Deviation
	if macdHist != nil {
		closes := closesOf(merged)
		vols := volumesOf(merged)
		turns := turnoversOf(merged)
		deviations = deviation.DetectDeviations(pivots, macdHist, macdDif,
			vols, turns, closes, cfg)
		td := deviation.DetectTrendDeviations(pivots, trends,
			macdHist, macdDif, vols, turns, closes, cfg)
		deviations = append(deviations, td...)
		trend.UpdateTrendsWithDeviations(trends, td)
	}

	signals := signal.DetectSignals(pivots, mergedBis, segs, deviations, cfg, macdHist)

	return &types.Result{
		MergedKlines: merged,
		Fractals:     bf,
		Bis:          bis,
		MergedBis:    mergedBis,
		Segments:     segs,
		Pivots:       pivots,
		Trends:       trends,
		Deviations:   deviations,
		Signals:      signals,
	}
}

// buildLevelFromLower 从下一级别的结果构建上一级别的完整分析。
func buildLevelFromLower(
	lowerResult *types.Result,
	lowerKlines []types.Kline,
	currentKlines []types.Kline,
	cfg types.Config,
) *types.Result {
	if lowerResult == nil || len(lowerResult.Segments) == 0 {
		return nil
	}
	higherBis := segmentsToBis(lowerResult.Segments, lowerKlines, currentKlines)
	if len(higherBis) < 2 {
		return nil
	}

	var higherMergedBis []types.MergedBi
	if cfg.EnableBiInclusion {
		higherMergedBis = bi.MergeBis(higherBis)
	} else {
		higherMergedBis = make([]types.MergedBi, len(higherBis))
		for i, b := range higherBis {
			higherMergedBis[i] = types.MergedBi{Bi: b, OriginalCount: 1}
		}
	}

	higherSegs := segment.BuildWithConfig(higherMergedBis, cfg.SegAlgo, cfg.LeftSegMethod)
	higherPivots := pivot.FindBiPivots(higherMergedBis, higherSegs, cfg)
	pivot.UpdateZSInSeg(higherMergedBis, higherPivots)
	higherTrends := trend.ClassifyTrends(higherPivots)
	macdHist, macdDif := calcMACD(currentKlines, cfg)

	var higherDevs []types.Deviation
	if macdHist != nil {
		closes := closesOf(currentKlines)
		vols := volumesOf(currentKlines)
		turns := turnoversOf(currentKlines)
		higherDevs = deviation.DetectDeviations(higherPivots, macdHist, macdDif,
			vols, turns, closes, cfg)
		td := deviation.DetectTrendDeviations(higherPivots, higherTrends,
			macdHist, macdDif, vols, turns, closes, cfg)
		higherDevs = append(higherDevs, td...)
		trend.UpdateTrendsWithDeviations(higherTrends, td)
	}

	higherSignals := signal.DetectSignals(higherPivots, higherMergedBis,
		higherSegs, higherDevs, cfg, macdHist)

	return &types.Result{
		MergedKlines: kline.MergeKlines(currentKlines, cfg.Inclusion),
		Bis:          higherBis,
		MergedBis:    higherMergedBis,
		Segments:     higherSegs,
		Pivots:       higherPivots,
		Trends:       higherTrends,
		Deviations:   higherDevs,
		Signals:      higherSignals,
	}
}

// ──────────────────────────────────────────────
// 区间套实现
// ──────────────────────────────────────────────

// BuildIntervalNesting 从最高级别信号出发，逐级下钻确认背驰。
// 若某一级未能确认，链条在该级断裂。
func BuildIntervalNesting(mr *MultiLevelResult) *types.IntervalNestingResult {
	if mr == nil || len(mr.Levels) < 2 {
		return nil
	}

	levels := mr.Levels
	names := make([]string, len(levels))
	for i, l := range levels {
		names[i] = l.Interval
	}

	var confirmations []types.Deviation
	finalIndex := -1
	finalPrice := 0.0
	confirmed := 0

	highest := levels[len(levels)-1]
	topDevs := highestDeviations(highest.Result.Deviations)
	if len(topDevs) == 0 {
		return nil
	}

	anchor := topDevs[0]
	anchorIdx := anchorToIndex(anchor, highest.Result)
	confirmations = append(confirmations, anchor)
	confirmed = 1

	for i := len(levels) - 2; i >= 0; i-- {
		lower := &levels[i]
		higher := &levels[i+1]
		mappedIdx := mapHigherToLowerIndex(anchorIdx, higher.Klines, lower.Klines)
		found := findConfirmationDeviation(lower.Result.Deviations, anchor,
			mappedIdx, len(lower.Klines))
		if found == nil {
			break
		}
		confirmations = append(confirmations, *found)
		confirmed++
		anchor = *found
		anchorIdx = anchorToIndex(anchor, lower.Result)
		finalIndex = anchorIdx
		finalPrice = anchor.PriceHigh
	}

	if confirmed < 2 {
		return nil
	}

	return &types.IntervalNestingResult{
		Levels:        names,
		Confirmations: confirmations,
		FinalIndex:    finalIndex,
		FinalPrice:    finalPrice,
		Accuracy:      float64(confirmed) / float64(len(levels)),
	}
}

func anchorToIndex(dev types.Deviation, result *types.Result) int {
	if dev.SegmentAfter != nil {
		return dev.SegmentAfter.EndIndex
	}
	if dev.TrendIndex >= 0 && dev.TrendIndex < len(result.Trends) {
		return result.Trends[dev.TrendIndex].EndIndex
	}
	return 0
}

func highestDeviations(devs []types.Deviation) []types.Deviation {
	var out []types.Deviation
	for _, d := range devs {
		if d.Level == types.TrendDeviation {
			out = append(out, d)
		}
	}
	if len(out) > 0 {
		return out
	}
	if len(devs) > 0 {
		return devs
	}
	return nil
}

func findConfirmationDeviation(
	devs []types.Deviation,
	higherDev types.Deviation,
	mappedIdx int,
	lowerLen int,
) *types.Deviation {
	if len(devs) == 0 {
		return nil
	}
	start := max(0, mappedIdx-5)
	end := min(lowerLen-1, mappedIdx+5)
	for i := range devs {
		d := &devs[i]
		if d.Direction != higherDev.Direction {
			continue
		}
		if d.SegmentAfter != nil &&
			d.SegmentAfter.EndIndex >= start &&
			d.SegmentAfter.EndIndex <= end {
			return d
		}
	}
	return nil
}

func mapHigherToLowerIndex(higherIdx int, higherKlines, lowerKlines []types.Kline) int {
	if higherIdx < 0 || higherIdx >= len(higherKlines) {
		if len(lowerKlines) == 0 {
			return 0
		}
		return len(lowerKlines) - 1
	}
	target := higherKlines[higherIdx].Time
	if target.IsZero() {
		if len(higherKlines) <= 1 || len(lowerKlines) <= 1 {
			return min(higherIdx, len(lowerKlines)-1)
		}
		return higherIdx * len(lowerKlines) / len(higherKlines)
	}
	for i, k := range lowerKlines {
		if !k.Time.Before(target.Time) {
			return i
		}
	}
	return len(lowerKlines) - 1
}

// ──────────────────────────────────────────────
// 共振计算
// ──────────────────────────────────────────────

func calcTotalResonance(results []LevelResult, tolerance float64) int {
	if len(results) < 2 {
		return 0
	}
	total := 0
	for i := 0; i < len(results)-1; i++ {
		total += calcResonance(results[i].Result.Signals, results[i+1].Result.Signals, tolerance)
	}
	return total
}

func calcResonance(base, higher []types.Signal, tolerance float64) int {
	if len(base) == 0 || len(higher) == 0 {
		return 0
	}
	if tolerance <= 0 {
		tolerance = 0.01
	}
	count := 0
	for _, bs := range base {
		for _, hs := range higher {
			if (bs.Type > 0 && hs.Type > 0) || (bs.Type < 0 && hs.Type < 0) {
				if bs.Price > 0 && hs.Price > 0 {
					if math.Abs(bs.Price-hs.Price)/bs.Price < tolerance {
						count++
						break
					}
				}
			}
		}
	}
	return count
}

// ──────────────────────────────────────────────
// 辅助函数：线段→笔映射
// ──────────────────────────────────────────────

func segmentsToBis(segs []types.Segment, baseKlines, higherKlines []types.Kline) []types.Bi {
	bis := make([]types.Bi, 0, len(segs))
	for _, seg := range segs {
		startIdx := mapIndexByTime(seg.StartIndex, baseKlines, higherKlines)
		endIdx := mapIndexByTime(seg.EndIndex, baseKlines, higherKlines)
		if startIdx < 0 || endIdx < 0 || startIdx >= endIdx {
			continue
		}
		startPrice := seg.Top
		if seg.Direction == types.DirUp {
			startPrice = seg.Bottom
		}
		endPrice := seg.Bottom
		if seg.Direction == types.DirUp {
			endPrice = seg.Top
		}
		hi, lo := math.Inf(-1), math.Inf(1)
		for i := startIdx; i <= endIdx && i < len(higherKlines); i++ {
			if higherKlines[i].High > hi {
				hi = higherKlines[i].High
			}
			if higherKlines[i].Low < lo {
				lo = higherKlines[i].Low
			}
		}
		if math.IsInf(hi, -1) {
			hi = seg.Top
		}
		if math.IsInf(lo, 1) {
			lo = seg.Bottom
		}
		ln := endPrice - startPrice
		if ln < 0 {
			ln = -ln
		}
		kc := endIdx - startIdx + 1
		sl := 0.0
		if kc > 0 {
			sl = ln / float64(kc)
		}
		bis = append(bis, types.Bi{
			StartIndex: startIdx, EndIndex: endIdx,
			Direction:  seg.Direction,
			StartPrice: startPrice, EndPrice: endPrice,
			High: hi, Low: lo,
			Length: ln, Slope: sl, KLineCount: kc,
		})
	}
	return bis
}

func mapIndexByTime(baseIdx int, baseKlines, higherKlines []types.Kline) int {
	if baseIdx < 0 || baseIdx >= len(baseKlines) {
		if baseIdx < 0 {
			return 0
		}
		if len(higherKlines) == 0 {
			return -1
		}
		return len(higherKlines) - 1
	}
	target := baseKlines[baseIdx].Time
	if target.IsZero() {
		if baseIdx < 0 || baseIdx >= len(higherKlines) {
			if baseIdx < 0 {
				return 0
			}
			return len(higherKlines) - 1
		}
		return baseIdx
	}
	for i, k := range higherKlines {
		if !k.Time.Before(target.Time) {
			return i
		}
	}
	return len(higherKlines) - 1
}

// ──────────────────────────────────────────────
// 辅助函数：K 线数据提取
// ──────────────────────────────────────────────

func calcMACD(klines []types.Kline, cfg types.Config) (histogram, dif []float64) {
	closes := closesOf(klines)
	if len(closes) <= cfg.MACDSlowPeriod {
		return nil, nil
	}
	r, err := talib.MACD(closes, cfg.MACDFastPeriod, cfg.MACDSlowPeriod, cfg.MACDSignalPeriod)
	if err != nil || r == nil {
		return nil, nil
	}
	return r.Histogram, r.MACD
}

func closesOf(k []types.Kline) []float64 {
	p := make([]float64, len(k))
	for i, c := range k {
		p[i] = c.Close
	}
	return p
}

func volumesOf(k []types.Kline) []float64 {
	v := make([]float64, len(k))
	for i, c := range k {
		v[i] = c.BaseVolume
	}
	return v
}

func turnoversOf(k []types.Kline) []float64 {
	t := make([]float64, len(k))
	for i, c := range k {
		switch {
		case c.Turnover > 0:
			t[i] = c.Turnover
		case c.QuoteVolume > 0:
			t[i] = c.QuoteVolume
		default:
			t[i] = c.BaseVolume
		}
	}
	return t
}
