package multi

import (
	"math"

	"github.com/bambuo/chan/bi"
	"github.com/bambuo/chan/deviation"
	"github.com/bambuo/chan/pivot"
	"github.com/bambuo/chan/segment"
	"github.com/bambuo/chan/signal"
	"github.com/bambuo/chan/trend"
	"github.com/bambuo/chan/types"
	"github.com/bambuo/talib"
)

// BuildMultiLevel 从基础级别的分析结果出发，使用高级别 K 线递归构筑更高级别结构。
//
// 核心原理（对齐 chan.py 多级别递归）：
//
//	L(n+1) 笔   = L(n) 线段
//	L(n+1) 线段 = L(n) 走势类型（工程近似：由 L(n+1) 笔构筑的线段）
//
// 参数：
//   - baseResult: 基础级别的完整分析结果（必须已计算 Segments）
//   - higherKlines: 高级别的 K 线序列（如 5m K 线，当基础级别为 1m 时）
//   - cfg: 配置（将使用其中的 SegAlgo、ZsAlgo 等参数）
func BuildMultiLevel(baseResult *types.Result, higherKlines []types.Kline, cfg types.Config) *MultiLevelResult {
	if baseResult == nil || len(baseResult.Segments) == 0 {
		return nil
	}
	if len(higherKlines) < 3 {
		return nil
	}

	// 步骤 1：将基础级别线段转换为高级别笔
	higherBis := segmentsToBis(baseResult.Segments, baseResult.MergedKlines, higherKlines)

	// 步骤 2：笔的包含处理
	var higherMergedBis []types.MergedBi
	if cfg.EnableBiInclusion && len(higherBis) >= 2 {
		higherMergedBis = bi.MergeBis(higherBis)
	} else {
		higherMergedBis = make([]types.MergedBi, len(higherBis))
		for i, b := range higherBis {
			higherMergedBis[i] = types.MergedBi{Bi: b, OriginalCount: 1}
		}
	}

	// 步骤 3：从合并笔构建线段
	higherSegs := segment.BuildWithConfig(higherMergedBis, cfg.SegAlgo, cfg.LeftSegMethod)

	// 步骤 4：中枢检测
	higherPivots := pivot.FindBiPivots(higherMergedBis, higherSegs, cfg)
	pivot.UpdateZSInSeg(higherMergedBis, higherPivots)

	// 步骤 5：走势类型分类
	higherTrends := trend.ClassifyTrends(higherPivots)

	// 步骤 6：背驰检测（需要高级别 K 线的 MACD）
	var higherDevs []types.Deviation
	closes := closesOf(higherKlines)
	if len(closes) > cfg.MACDSlowPeriod {
		r, err := talib.MACD(closes, cfg.MACDFastPeriod, cfg.MACDSlowPeriod, cfg.MACDSignalPeriod)
		if err == nil && r != nil {
			vols := volumesOf(higherKlines)
			turns := turnoversOf(higherKlines)
			higherDevs = deviation.DetectDeviations(higherPivots, r.Histogram, r.MACD,
				vols, turns, closes, cfg)
			td := deviation.DetectTrendDeviations(higherPivots, higherTrends,
				r.Histogram, r.MACD, vols, turns, closes, cfg)
			higherDevs = append(higherDevs, td...)
			trend.UpdateTrendsWithDeviations(higherTrends, td)
		}
	}

	// 步骤 7：买卖点检测
	higherSignals := signal.DetectSignals(higherPivots, higherMergedBis, higherSegs, higherDevs, cfg)

	// 步骤 8：计算共振数
	resonance := calcResonance(baseResult.Signals, higherSignals, cfg.ResonanceTolerance)

	return &MultiLevelResult{
		Levels: []LevelResult{
			{Result: baseResult},
			{Interval: "higher", Klines: higherKlines, Result: &types.Result{
				MergedKlines: higherKlines,
				Bis:          higherBis,
				MergedBis:    higherMergedBis,
				Segments:     higherSegs,
				Pivots:       higherPivots,
				Trends:       higherTrends,
				Deviations:   higherDevs,
				Signals:      higherSignals,
			}},
		},
		Resonance:  resonance,
		Deviations: higherDevs,
		Signals:    higherSignals,
	}
}

// segmentsToBis 将基础级别线段转换为高级别笔。
// 每个线段 → 一笔：方向由线段方向决定，高低点由线段范围决定。
// baseKlines 为基础级别的 K 线序列（合并后），用于时间戳映射到高级别 K 线索引。
func segmentsToBis(segs []types.Segment, baseKlines, higherKlines []types.Kline) []types.Bi {
	bis := make([]types.Bi, 0, len(segs))
	for _, seg := range segs {
		// 映射线段起止点到高级别 K 线索引（基于时间戳）
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
		// 若无法从 K 线获取高低点，使用线段本身的极值
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

// mapIndexByTime 基于时间戳将基础 K 线索引映射到高级别 K 线序列中的索引。
// 在 higherKlines 中查找第一个时间 >= baseKlines[baseIdx].Time 的位置。
// 若 baseIdx 越界则返回最近的边界索引；若高级别 K 线均早于目标时间，返回末尾。
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
		// 时间戳为空时退化为恒等映射（保持向后兼容）
		if baseIdx < 0 || baseIdx >= len(higherKlines) {
			if baseIdx < 0 {
				return 0
			}
			return len(higherKlines) - 1
		}
		return baseIdx
	}
	// 线性扫描（高级别 K 线通常远少于基础级别，性能可接受）
	for i, k := range higherKlines {
		if !k.Time.Before(target.Time) {
			return i
		}
	}
	return len(higherKlines) - 1
}

// calcResonance 计算两个级别的信号共振数。
// 同一时刻（同向）出现同向买卖点计为一次共振。
// tolerance 为价格差异容差（百分比，默认 0.01 = 1%）。
func calcResonance(baseSignals, higherSignals []types.Signal, tolerance float64) int {
	if len(baseSignals) == 0 || len(higherSignals) == 0 {
		return 0
	}
	if tolerance <= 0 {
		tolerance = 0.01 // 保底容差
	}
	count := 0
	for _, bs := range baseSignals {
		for _, hs := range higherSignals {
			// 简单共振判定：同向且价格接近
			if isSameDirection(bs.Type, hs.Type) {
				if bs.Price > 0 && hs.Price > 0 {
					diff := math.Abs(bs.Price-hs.Price) / bs.Price
					if diff < tolerance {
						count++
						break
					}
				}
			}
		}
	}
	return count
}

func isSameDirection(a, b types.SignalType) bool {
	return (a > 0 && b > 0) || (a < 0 && b < 0)
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
