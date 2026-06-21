package signal

import (
	"github.com/bambuo/chan/pivot"
	"github.com/bambuo/chan/segment"
	"github.com/bambuo/chan/types"
)

// ── 线段级买卖点 ──
//
// 对齐 Python 的 seg_bs_point 设计：将已检测出的线段视为高级别的「笔」，
// 在此基础上重新走线段/中枢/信号检测流程，得到独立于笔级别的买卖点信号。

// SegSignalsCtx 保存线段级 BSP 检测所需的全部中间产物。
type SegSignalsCtx struct {
	SegAsBis   []types.MergedBi  // 线段转换后的「高级别笔」
	Merged     []types.MergedBi  // 包含处理后的笔
	Segments   []types.Segment   // 高级别线段（seg-of-seg）
	Pivots     []types.Pivot     // 高级别中枢
	Deviations []types.Deviation // 线段级背驰检测结果
	Signals    []types.Signal    // 检测出的线段级信号
}

// DetectSegSignals 在已检测出的线段基础上，执行线段级买卖点检测。
//
// 原理（对齐 Python seg_bs_point）：
//
//	segments → 高级别笔 → 包含处理 → 高级别线段 → 高级别中枢 → 信号
//
// 返回 SegSignalsCtx 包含完整的中间产物供调试/验证。
func DetectSegSignals(segs []types.Segment, cfg types.Config) *SegSignalsCtx {
	if len(segs) < 3 {
		return nil
	}

	ctx := &SegSignalsCtx{}

	// 步骤 1：线段 → 高级别笔
	ctx.SegAsBis = segmentsToMergedBis(segs)
	if len(ctx.SegAsBis) < 2 {
		return nil
	}

	// 步骤 2：笔的包含处理（对齐 bi.MergeBis）
	ctx.Merged = mergeSegAsBis(ctx.SegAsBis)

	// 步骤 3：从合并笔构建高级别线段（seg-of-seg）
	ctx.Segments = segment.BuildWithConfig(ctx.Merged, cfg.SegAlgo, cfg.LeftSegMethod)
	if len(ctx.Segments) < 1 {
		return nil
	}

	// 步骤 4：高级别中枢检测
	ctx.Pivots = pivot.FindBiPivots(ctx.Merged, ctx.Segments, cfg)
	pivot.UpdateZSInSeg(ctx.Merged, ctx.Pivots)

	// 步骤 5：构造线段级配置
	segCfg := SegBspConfig(cfg)
	curPivots := ctx.Pivots

	// 步骤 6：检测线段级买卖点（偏差由上层通过 ctx.Deviations 传入，
	// 见 analysis.go DetectSegSignals 中 segDevs 的存储和重新检测）
	ctx.Signals = DetectSignals(curPivots, ctx.Merged, ctx.Segments, ctx.Deviations, segCfg)

	return ctx
}

// segmentsToMergedBis 将线段列表转换为 MergedBi 序列（作为高级别的「笔」）。
//
// 映射规则：
//
//	Bi.StartIndex/EndIndex = Segment.StartIndex/EndIndex
//	Bi.Direction           = Segment.Direction
//	Bi.High/Low            = Segment.Top/Bottom
//	Bi.StartPrice/EndPrice = 按方向取 Top/Bottom
func segmentsToMergedBis(segs []types.Segment) []types.MergedBi {
	result := make([]types.MergedBi, 0, len(segs))
	for _, seg := range segs {
		var startPrice, endPrice float64
		switch seg.Direction {
		case types.DirUp:
			startPrice = seg.Bottom
			endPrice = seg.Top
		case types.DirDown:
			startPrice = seg.Top
			endPrice = seg.Bottom
		default:
			continue
		}

		amp := seg.Top - seg.Bottom
		if amp < 0 {
			amp = -amp
		}
		span := seg.EndIndex - seg.StartIndex
		if span <= 0 {
			span = 1
		}

		bi := types.Bi{
			StartIndex: seg.StartIndex,
			EndIndex:   seg.EndIndex,
			Direction:  seg.Direction,
			StartPrice: startPrice,
			EndPrice:   endPrice,
			High:       seg.Top,
			Low:        seg.Bottom,
			Length:     amp,
			Slope:      amp / float64(span),
			KLineCount: span,
		}
		result = append(result, types.MergedBi{Bi: bi, OriginalCount: 1})
	}
	return result
}

// mergeSegAsBis 对线段级笔做包含处理（合并相邻同向笔）。
// 逻辑对齐 bi.MergeBis，使用方向感知的合并方式。
func mergeSegAsBis(bis []types.MergedBi) []types.MergedBi {
	if len(bis) < 2 {
		if len(bis) == 1 {
			return []types.MergedBi{{Bi: bis[0].Bi, OriginalCount: 1}}
		}
		return nil
	}
	merged := make([]types.MergedBi, 0, len(bis))
	cur := bis[0]
	cur.MergedFrom = []int{0}
	for i := 1; i < len(bis); i++ {
		next := bis[i]
		if cur.Bi.Direction != next.Bi.Direction {
			merged = append(merged, cur)
			next.MergedFrom = []int{i}
			cur = next
			continue
		}
		// 同向包含处理
		if segBiContain(next.Bi, cur.Bi) {
			// next 被 cur 包含 → 保持 cur，更新计数
			cur.OriginalCount++
			cur.MergedFrom = append(cur.MergedFrom, i)
			continue
		}
		if segBiContain(cur.Bi, next.Bi) {
			// cur 被 next 包含 → 用 next 替换 cur
			dir := segBiDir(merged, next.Bi)
			m := segMergePair(next.Bi, cur.Bi, dir)
			cur = types.MergedBi{Bi: m, OriginalCount: cur.OriginalCount + 1}
			cur.MergedFrom = append(cur.MergedFrom, i)
			continue
		}
		merged = append(merged, cur)
		next.MergedFrom = []int{i}
		cur = next
	}
	merged = append(merged, cur)
	return merged
}

// segBiContain 检查 a 是否包含 b（a 的区间完全覆盖 b）。
func segBiContain(a, b types.Bi) bool {
	return a.High >= b.High && a.Low <= b.Low
}

// segBiDir 从已合并的笔序列推断当前合并方向。
func segBiDir(merged []types.MergedBi, cur types.Bi) types.Direction {
	for i := len(merged) - 1; i >= 1; i-- {
		p, c := merged[i-1].Bi, merged[i].Bi
		if !segBiContain(p, c) && !segBiContain(c, p) {
			if c.High > p.High && c.Low > p.Low {
				return types.DirUp
			}
			if c.High < p.High && c.Low < p.Low {
				return types.DirDown
			}
		}
	}
	if cur.IsUp() {
		return types.DirUp
	}
	return types.DirDown
}

// segMergePair 按方向合并两支同向笔。
func segMergePair(a, b types.Bi, dir types.Direction) types.Bi {
	m := a
	m.EndIndex = b.EndIndex
	m.EndPrice = b.EndPrice
	m.KLineCount = m.EndIndex - m.StartIndex + 1
	switch dir {
	case types.DirUp:
		m.High = maxF(a.High, b.High)
		m.Low = maxF(a.Low, b.Low)
	case types.DirDown:
		m.High = minF(a.High, b.High)
		m.Low = minF(a.Low, b.Low)
	default:
		if a.IsUp() {
			m.High = maxF(a.High, b.High)
			m.Low = maxF(a.Low, b.Low)
		} else {
			m.High = minF(a.High, b.High)
			m.Low = minF(a.Low, b.Low)
		}
	}
	m.Length = m.EndPrice - m.StartPrice
	if m.Length < 0 {
		m.Length = -m.Length
	}
	if m.KLineCount > 0 {
		m.Slope = m.Length / float64(m.KLineCount)
	}
	return m
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// SegBspConfig 从基础配置构造线段级 BSP 配置。
// 对齐 Python 的 seg_bs_point_conf 覆盖规则：
//   - macd_algo → "slope"（或用户指定的 SegBspMacdAlgo）
//   - bsp1_only_multibi_zs → SegBsp1OnlyMultiBiZs
//   - divergence_rate → SegBspDivergenceRate
func SegBspConfig(base types.Config) types.Config {
	cfg := base
	cfg.BspMacdAlgo = "slope"
	if base.SegBspMacdAlgo != "" {
		cfg.BspMacdAlgo = base.SegBspMacdAlgo
	}
	cfg.BspMacdAlgoBuy = base.BspMacdAlgoBuy
	cfg.BspMacdAlgoSell = base.BspMacdAlgoSell
	cfg.Bsp1OnlyMultiBiZs = base.SegBsp1OnlyMultiBiZs
	cfg.BspDivergenceRate = base.SegBspDivergenceRate
	if base.SegBspType != "" {
		cfg.BspType = base.SegBspType
	}
	// 线段级偏差检测依靠价格斜率，不依赖 MACD
	// 跳过 MACD 相关的 deviation 检测
	return cfg
}
