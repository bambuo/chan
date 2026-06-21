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
	SegAsBis []types.MergedBi // 线段转换后的「高级别笔」
	Merged   []types.MergedBi // 包含处理后的笔
	Segments []types.Segment  // 高级别线段（seg-of-seg）
	Pivots   []types.Pivot    // 高级别中枢
	Signals  []types.Signal   // 检测出的线段级信号
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
	segCfg := segBspConfig(cfg)
	curPivots := ctx.Pivots

	// 步骤 5b：计算线段级背驰（若 klines 数据可用则使用当前系列闭盘价）
	var segDevs []types.Deviation
	_ = segDevs // 占位 — 完整的偏差检测需传入 K 线级别的 MACD 数据,
	// 由上层 Analysis.DetectDeviations 完成后在 DetectSegSignals 调用处提供

	// 步骤 6：检测线段级买卖点（SegmentDeviation 偏差由上层传入或留空）
	ctx.Signals = DetectSignals(curPivots, ctx.Merged, ctx.Segments, nil, segCfg)

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
// 逻辑对齐 bi.MergeBis。
func mergeSegAsBis(bis []types.MergedBi) []types.MergedBi {
	if len(bis) < 2 {
		if len(bis) == 1 {
			return []types.MergedBi{{Bi: bis[0].Bi, OriginalCount: 1}}
		}
		return nil
	}
	merged := make([]types.MergedBi, 0, len(bis))
	cur := bis[0]
	for i := 1; i < len(bis); i++ {
		next := bis[i]
		// 同向合并
		if cur.Bi.Direction == next.Bi.Direction {
			if cur.Bi.IsUp() {
				if next.Bi.High <= cur.Bi.High && next.Bi.Low >= cur.Bi.Low {
					// next 被 cur 包含
					cur.OriginalCount++
					continue
				}
				if next.Bi.High > cur.Bi.High && next.Bi.Low < cur.Bi.Low {
					// cur 被 next 包含
					cur = next
					cur.OriginalCount++
					continue
				}
			} else {
				if next.Bi.Low >= cur.Bi.Low && next.Bi.High <= cur.Bi.High {
					cur.OriginalCount++
					continue
				}
				if next.Bi.Low < cur.Bi.Low && next.Bi.High > cur.Bi.High {
					cur = next
					cur.OriginalCount++
					continue
				}
			}
		}
		merged = append(merged, cur)
		cur = next
	}
	merged = append(merged, cur)
	return merged
}

// segBspConfig 从基础配置构造线段级 BSP 配置。
// 对齐 Python 的 seg_bs_point_conf 覆盖规则：
//   - macd_algo → "slope"（或用户指定的 SegBspMacdAlgo）
//   - bsp1_only_multibi_zs → SegBsp1OnlyMultiBiZs
//   - divergence_rate → SegBspDivergenceRate
func segBspConfig(base types.Config) types.Config {
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
