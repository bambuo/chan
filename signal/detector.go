package signal

import (
	"math"
	"strconv"

	"github.com/bambuo/chan/features"
	"github.com/bambuo/chan/types"
)

// DetectSignals 基于笔级中枢和笔序列检测全部买卖点。
func DetectSignals(pivots []types.Pivot, bis []types.MergedBi, segments []types.Segment,
	deviations []types.Deviation, config types.Config) []types.Signal {
	if len(segments) == 0 || len(bis) == 0 {
		return nil
	}
	targets := parseBspTypes(config.BspType)
	groups := groupBySeg(segments, pivots, bis)
	bsp1Map := make(map[int]bool)
	bsp1IdxMap := make(map[int]int) // seg.EndIndex → T1 signal K-line index
	bspAllMap := make(map[int]bool)
	var signals []types.Signal
	for gi := range groups {
		g := &groups[gi]
		if sig := detectT1(g, bis, deviations, config, targets); sig != nil {
			signals = append(signals, *sig)
			bsp1Map[sig.Index] = true
			bsp1IdxMap[sig.Index] = sig.Index
			bspAllMap[sig.Index] = true
		}
	}
	for gi := range groups {
		g := &groups[gi]
		signals = append(signals, detectT2(g, groups, bis, bsp1Map, bsp1IdxMap, bspAllMap, config, targets)...)
	}
	for gi := range groups {
		g := &groups[gi]
		signals = append(signals, detectT3(g, groups, gi, bis, bsp1Map, bsp1IdxMap, bspAllMap, config, targets)...)
	}
	// 中枢边界信号
	signals = append(signals, detectBoundarySignals(pivots, bis, targets, config.BoundaryToleranceRatio)...)
	return signals
}

type segGroup struct {
	Seg    types.Segment
	Pivots []types.Pivot
}

func groupBySeg(segs []types.Segment, pivots []types.Pivot, bis []types.MergedBi) []segGroup {
	g := make([]segGroup, len(segs))
	for i := range segs {
		g[i] = segGroup{Seg: segs[i]}
	}
	for pi := range pivots {
		zs := &pivots[pi]
		if zs.BeginBiIdx < 0 || zs.BeginBiIdx >= len(bis) {
			continue
		}
		start := bis[zs.BeginBiIdx].StartIndex
		for si := range segs {
			if start >= segs[si].StartIndex && start <= segs[si].EndIndex {
				g[si].Pivots = append(g[si].Pivots, *zs)
				break
			}
		}
	}
	return g
}

func (g *segGroup) firstMultiBiZS() *types.Pivot {
	for i := range g.Pivots {
		if !g.Pivots[i].IsOneBiZs() {
			return &g.Pivots[i]
		}
	}
	return nil
}

func (g *segGroup) lastMultiBiZS() *types.Pivot {
	for i := len(g.Pivots) - 1; i >= 0; i-- {
		if !g.Pivots[i].IsOneBiZs() {
			return &g.Pivots[i]
		}
	}
	return nil
}

func (g *segGroup) multiCnt() int {
	c := 0
	for i := range g.Pivots {
		if !g.Pivots[i].IsOneBiZs() {
			c++
		}
	}
	return c
}

// ── T1 / T1P ──

func detectT1(g *segGroup, bis []types.MergedBi, devs []types.Deviation, cfg types.Config, targets map[string]bool) *types.Signal {
	seg := g.Seg
	endBiIdx := findEndIdx(bis, seg.EndIndex)
	if endBiIdx < 0 {
		return nil
	}
	if len(g.Pivots) > 0 {
		last := &g.Pivots[len(g.Pivots)-1]
		if !last.IsOneBiZs() && last.BiIn != nil {
			// 对齐 Python: 检查 BiOut 是否到达线段末端
			reachesEnd := false
			if last.BiOut != nil && last.BiOut.EndIndex >= seg.EndIndex {
				reachesEnd = true
			}
			if !reachesEnd && len(last.BiList) > 0 {
				lastBi := &last.BiList[len(last.BiList)-1]
				if lastBi.EndIndex >= seg.EndIndex {
					reachesEnd = true
				}
			}
			inIdx := findStartIdx(bis, last.BiIn.StartIndex)
			if reachesEnd && endBiIdx-inIdx > 2 {
				return treatT1(g, last, bis, devs, cfg, targets)
			}
		}
	}
	return treatT1P(g, bis, cfg, targets)
}

func treatT1(g *segGroup, last *types.Pivot, bis []types.MergedBi, devs []types.Deviation, cfg types.Config, targets map[string]bool) *types.Signal {
	seg := g.Seg
	endBiIdx := findEndIdx(bis, seg.EndIndex)
	if endBiIdx < 0 {
		return nil
	}
	endBi := &bis[endBiIdx].Bi
	if cfg.Bsp1Peak && last.BiOut != nil && !outIsPeak(last, endBiIdx) {
		return nil
	}
	diver := false
	for i := range devs {
		d := &devs[i]
		if d.Direction == seg.Direction && d.PriceHigh == endBi.EndPrice {
			diver = true
			break
		}
	}
	if !diver && last.BiIn != nil && last.BiOut != nil {
		diver = outBreak(endBi, last.ZG, last.ZD)
	}
	if !diver {
		return nil
	}
	cnt := len(g.Pivots)
	if cfg.Bsp1OnlyMultiBiZs {
		cnt = g.multiCnt()
	}
	if cfg.BspMinZsCnt > 0 && cnt < cfg.BspMinZsCnt {
		return nil
	}
	if !targets["1"] {
		return nil
	}
	buy := seg.Direction == types.DirDown
	st := types.BuyPoint1
	sub := types.SubT1
	if !buy {
		st = types.SellPoint1
	}
	// 从偏差中提取背驰率
	dr := math.Inf(1)
	for i := range devs {
		d := &devs[i]
		if d.Direction == seg.Direction && d.PriceHigh == endBi.EndPrice {
			if d.ForceBefore > 1e-12 {
				dr = d.ForceAfter / d.ForceBefore
			}
			break
		}
	}
	return &types.Signal{Type: st, SubType: sub, Level: "线段级别",
		Index: seg.EndIndex, Price: endBi.EndPrice, Strength: 0.7,
		Features: features.Merge(features.Common(*endBi), features.T1(dr, g.multiCnt())),
	}
}

func treatT1P(g *segGroup, bis []types.MergedBi, cfg types.Config, targets map[string]bool) *types.Signal {
	seg := g.Seg
	lastIdx := findEndIdx(bis, seg.EndIndex)
	if lastIdx < 0 || lastIdx < 2 {
		return nil
	}
	last, pre := &bis[lastIdx].Bi, &bis[lastIdx-2].Bi
	if last.Direction != seg.Direction {
		return nil
	}
	if last.IsDown() && last.ZSLow() > pre.ZSLow() {
		return nil
	}
	if last.IsUp() && last.ZSHigh() < pre.ZSHigh() {
		return nil
	}
	rate := cfg.BspDivergenceRate
	if !(math.IsInf(rate, 1) || rate > 100) {
		if pre.Amp() > 1e-12 && last.Amp() > rate*pre.Amp() {
			return nil
		}
	}
	if !targets["1p"] {
		return nil
	}
	buy := seg.Direction == types.DirDown
	st := types.BuyPoint1
	sub := types.SubT1P
	if !buy {
		st = types.SellPoint1
	}
	return &types.Signal{Type: st, SubType: sub, Level: "笔级别",
		Index: seg.EndIndex, Price: last.EndPrice, Strength: 0.5,
		Features: features.Merge(features.Common(*last), features.T1P(rate, last.Amp())),
	}
}

func outIsPeak(zs *types.Pivot, _ int) bool {
	if zs.BiOut == nil || len(zs.BiList) == 0 {
		return false
	}
	for i := range zs.BiList {
		bi := &zs.BiList[i]
		if (zs.BiOut.IsDown() && bi.ZSLow() < zs.BiOut.ZSLow()) ||
			(zs.BiOut.IsUp() && bi.ZSHigh() > zs.BiOut.ZSHigh()) {
			return false
		}
	}
	return true
}

func outBreak(out *types.Bi, zg, zd float64) bool {
	if out == nil {
		return false
	}
	if out.IsDown() {
		return out.ZSLow() < zd
	}
	return out.IsUp() && out.ZSHigh() > zg
}

func findEndIdx(bis []types.MergedBi, end int) int {
	for i := range bis {
		if bis[i].EndIndex == end {
			return i
		}
	}
	return -1
}

func findStartIdx(bis []types.MergedBi, start int) int {
	for i := range bis {
		if bis[i].StartIndex == start {
			return i
		}
	}
	return -1
}

// ── T2 / T2S ──

func detectT2(g *segGroup, groups []segGroup, bis []types.MergedBi,
	bsp1Map map[int]bool, bsp1IdxMap map[int]int, _ map[int]bool, cfg types.Config, targets map[string]bool) []types.Signal {
	seg := g.Seg
	b1Idx := findEndIdx(bis, seg.EndIndex)
	if b1Idx < 0 || b1Idx+2 >= len(bis) {
		return nil
	}
	if cfg.Bsp2Follow1 && !bsp1Map[seg.EndIndex] {
		return nil
	}
	relIdx := -1
	if cfg.Bsp2Follow1 {
		if idx, ok := bsp1IdxMap[seg.EndIndex]; ok {
			relIdx = idx
		}
	}
	breakBi := &bis[b1Idx+1].Bi
	b2Bi := &bis[b1Idx+2].Bi
	var sigs []types.Signal
	ret := math.Inf(1)
	if breakBi.Amp() > 1e-12 {
		ret = b2Bi.Amp() / breakBi.Amp()
	}
	b2Ok := ret <= cfg.BspMaxBs2Rate
	if b2Ok && targets["2"] {
		buy := b2Bi.IsDown()
		st := types.BuyPoint2
		if !buy {
			st = types.SellPoint2
		}
		sigs = append(sigs, types.Signal{Type: st, SubType: types.SubT2,
			Level: "线段级别", Index: b2Bi.StartIndex, Price: b2Bi.EndPrice, Strength: 0.6,
			RelatedBSP1Index: relIdx,
			Features: features.Merge(features.Common(*b2Bi),
				features.T2(ret, breakBi.Amp(), b2Bi.Amp())),
		})
	} else if cfg.Bsp2sFollow2 && !b2Ok {
		return sigs
	}
	if !targets["2s"] {
		return sigs
	}
	sigs = append(sigs, detectT2S(g, groups, bis, b1Idx, b2Bi, breakBi, relIdx, cfg, targets)...)
	return sigs
}

func detectT2S(g *segGroup, _ []segGroup, bis []types.MergedBi,
	b1Idx int, b2Bi, breakBi *types.Bi, relIdx int, cfg types.Config, targets map[string]bool) []types.Signal {
	var sigs []types.Signal
	bias := 2
	var lo, hi *float64
	b2Idx := b1Idx + 2
	for b2Idx+bias < len(bis) {
		// 对齐 Python: max_bsp2s_lv 限制搜索层级
		if cfg.BspMaxBs2sLv != nil && bias/2 > *cfg.BspMaxBs2sLv {
			break
		}
		sBi := &bis[b2Idx+bias].Bi
		if sBi.StartIndex > g.Seg.EndIndex {
			break
		}
		if bias == 2 {
			if !overlap(b2Bi.ZSLow(), b2Bi.ZSHigh(), sBi.ZSLow(), sBi.ZSHigh(), false) {
				break
			}
			l, h := b2Bi.ZSLow(), b2Bi.ZSHigh()
			if sBi.ZSLow() > l {
				l = sBi.ZSLow()
			}
			if sBi.ZSHigh() < h {
				h = sBi.ZSHigh()
			}
			lo, hi = &l, &h
		} else if lo != nil && hi != nil {
			if !overlap(*lo, *hi, sBi.ZSLow(), sBi.ZSHigh(), false) {
				break
			}
		}
		if (sBi.IsDown() && sBi.ZSLow() < breakBi.ZSLow()) ||
			(sBi.IsUp() && sBi.ZSHigh() > breakBi.ZSHigh()) {
			break
		}
		r := math.Abs(sBi.EndVal() - breakBi.EndVal())
		if breakBi.Amp() > 1e-12 {
			r /= breakBi.Amp()
		}
		if r > cfg.BspMaxBs2Rate {
			break
		}
		buy := sBi.IsDown()
		st := types.BuyPoint2
		if !buy {
			st = types.SellPoint2
		}
		sigs = append(sigs, types.Signal{Type: st, SubType: types.SubT2S,
			Level: "线段级别", Index: sBi.StartIndex, Price: sBi.EndPrice, Strength: 0.5,
			RelatedBSP1Index: relIdx,
			Features: features.Merge(features.Common(*sBi),
				features.T2S(r, breakBi.Amp(), sBi.Amp(), bias/2)),
		})
		bias += 2
	}
	return sigs
}

// ── T3A / T3B ──

func detectT3(g *segGroup, groups []segGroup, gi int, bis []types.MergedBi,
	bsp1Map map[int]bool, bsp1IdxMap map[int]int, _ map[int]bool, cfg types.Config, targets map[string]bool) []types.Signal {
	seg := g.Seg
	b1Idx := findEndIdx(bis, seg.EndIndex)
	if b1Idx < 0 {
		return nil
	}
	if cfg.Bsp3Follow1 && !bsp1Map[seg.EndIndex] {
		return nil
	}
	relIdx := -1
	if cfg.Bsp3Follow1 {
		if idx, ok := bsp1IdxMap[seg.EndIndex]; ok {
			relIdx = idx
		}
	}
	if !targets["3a"] && !targets["3b"] {
		return nil
	}
	var ng *segGroup
	if gi+1 < len(groups) {
		ng = &groups[gi+1]
	}
	var sigs []types.Signal
	if ng != nil && targets["3a"] {
		sigs = append(sigs, treatT3A(g, ng, gi, bis, b1Idx, relIdx, cfg, targets)...)
	}
	if targets["3b"] && bsp1Map[seg.EndIndex] {
		sigs = append(sigs, treatT3B(g, ng, gi, bis, b1Idx, relIdx, cfg, targets)...)
	}
	return sigs
}

func treatT3A(_, ng *segGroup, _ int, bis []types.MergedBi,
	b1Idx int, relIdx int, cfg types.Config, targets map[string]bool) []types.Signal {
	first := ng.firstMultiBiZS()
	if first == nil {
		return nil
	}
	if cfg.StrictBsp3 && first.BiIn != nil {
		if findStartIdx(bis, first.BiIn.StartIndex) != b1Idx+1 {
			return nil
		}
	}
	var multi []types.Pivot
	for i := range ng.Pivots {
		if !ng.Pivots[i].IsOneBiZs() {
			multi = append(multi, ng.Pivots[i])
		}
	}
	mc := cfg.Bsp3aMaxZsCnt
	if mc <= 0 {
		mc = 1
	}
	var sigs []types.Signal
	for zi, zs := range multi {
		if zi >= mc {
			break
		}
		if zs.BiOut == nil {
			break
		}
		outIdx := findStartIdx(bis, zs.BiOut.StartIndex)
		if outIdx+1 >= len(bis) || outIdx < 0 {
			break
		}
		b3 := &bis[outIdx+1].Bi
		if b3.Direction == ng.Seg.Direction {
			break
		}
		if back2ZS(b3, &zs) {
			continue
		}
		if cfg.Bsp3Peak && !breakPeak(b3, &zs) {
			continue
		}
		buy := b3.IsDown()
		st := types.BuyPoint3
		if !buy {
			st = types.SellPoint3
		}
		sigs = append(sigs, types.Signal{Type: st, SubType: types.SubT3A,
			Level: "线段级别", Index: b3.StartIndex, Price: b3.EndPrice, Strength: 0.65,
			RelatedBSP1Index: relIdx,
			Features: features.Merge(features.Common(*b3),
				features.T3(zs.ZG, zs.ZD, b3.Amp())),
		})
	}
	return sigs
}

func treatT3B(g, ng *segGroup, _ int, bis []types.MergedBi,
	b1Idx int, relIdx int, cfg types.Config, _ map[string]bool) []types.Signal {
	cmp := g.lastMultiBiZS()
	if cmp == nil {
		return nil
	}
	if cfg.StrictBsp3 {
		if cmp.BiOut == nil {
			return nil
		}
		if findStartIdx(bis, cmp.BiOut.StartIndex) != b1Idx {
			return nil
		}
	}
	var sigs []types.Signal
	for idx := b1Idx + 2; idx < len(bis); idx += 2 {
		b3 := &bis[idx].Bi
		if ng != nil && b3.StartIndex >= ng.Seg.StartIndex {
			break
		}
		if back2ZS(b3, cmp) {
			continue
		}
		buy := b3.IsDown()
		st := types.BuyPoint3
		if !buy {
			st = types.SellPoint3
		}
		sigs = append(sigs, types.Signal{Type: st, SubType: types.SubT3B,
			Level: "线段级别", Index: b3.StartIndex, Price: b3.EndPrice, Strength: 0.6,
			RelatedBSP1Index: relIdx,
			Features: features.Merge(features.Common(*b3),
				features.T3(cmp.ZG, cmp.ZD, b3.Amp())),
		})
		break
	}
	return sigs
}

func back2ZS(bi *types.Bi, zs *types.Pivot) bool {
	return (bi.IsDown() && bi.ZSLow() < zs.ZG) || (bi.IsUp() && bi.ZSHigh() > zs.ZD)
}

func breakPeak(bi *types.Bi, zs *types.Pivot) bool {
	return (bi.IsDown() && bi.ZSHigh() >= zs.PeakHigh) || (bi.IsUp() && bi.ZSLow() <= zs.PeakLow)
}

func parseBspTypes(s string) map[string]bool {
	r := make(map[string]bool)
	if s == "" {
		for _, t := range []string{"1", "1p", "2", "2s", "3a", "3b", "support", "resist", "breakUp", "breakDn"} {
			r[t] = true
		}
		return r
	}
	for _, p := range split(s) {
		r[p] = true
	}
	return r
}

func split(s string) []string {
	var r []string
	cur := ""
	for _, c := range s {
		if c == ',' {
			if cur != "" {
				r = append(r, cur)
			}
			cur = ""
		} else if c != ' ' {
			cur += string(c)
		}
	}
	if cur != "" {
		r = append(r, cur)
	}
	return r
}

func overlap(l1, h1, l2, h2 float64, eq bool) bool {
	if eq {
		return h2 >= l1 && h1 >= l2
	}
	return h2 > l1 && h1 > l2
}

// ── 中枢边界信号（对齐 index.html 的 support/resist/breakUp/breakDn）──

// detectBoundarySignals 检测中枢边界的支撑/压力/突破/跌破信号。
// 遍历每个非单笔中枢，检查其后的笔与 ZG/ZD 的交互关系。
// toleranceRatio 为 ZG/ZD 容差比例（中枢高度的百分比，默认 0.1 = 10%）。
func detectBoundarySignals(pivots []types.Pivot, bis []types.MergedBi, targets map[string]bool, toleranceRatio float64) []types.Signal {
	if !targets["support"] && !targets["resist"] && !targets["breakUp"] && !targets["breakDn"] {
		return nil
	}
	var sigs []types.Signal
	seen := make(map[string]bool) // "type:biIdx" 去重

	for pi := range pivots {
		zs := &pivots[pi]
		if zs.IsOneBiZs() {
			continue // 跳过多笔中枢
		}
		tolerance := (zs.ZG - zs.ZD) * toleranceRatio
		if tolerance <= 0 {
			tolerance = zs.ZG * 0.01 // 保底容差
		}

		// 从中枢结束笔之后开始扫描
		startScan := zs.EndBiIdx + 1
		if startScan < 0 || startScan >= len(bis) {
			continue
		}
		for i := startScan; i < len(bis); i++ {
			bi := &bis[i].Bi
			biLow := bi.ZSLow()
			biHigh := bi.ZSHigh()

			// 支撑：向下笔的低点接近 ZD（在容差范围内）
			if targets["support"] && bi.IsDown() {
				if biLow >= zs.ZD-tolerance && biLow <= zs.ZD+tolerance {
					key := "support:" + strconv.Itoa(i)
					if !seen[key] {
						seen[key] = true
						sigs = append(sigs, types.Signal{
							Type: types.BuyPoint2, SubType: types.SubTSupport,
							Level: "中枢边界", Index: bi.StartIndex, Price: biLow, Strength: 0.4,
							Features: features.Common(*bi),
						})
					}
				}
			}

			// 压力：向上笔的高点接近 ZG（在容差范围内）
			if targets["resist"] && bi.IsUp() {
				if biHigh >= zs.ZG-tolerance && biHigh <= zs.ZG+tolerance {
					key := "resist:" + strconv.Itoa(i)
					if !seen[key] {
						seen[key] = true
						sigs = append(sigs, types.Signal{
							Type: types.SellPoint2, SubType: types.SubTResist,
							Level: "中枢边界", Index: bi.StartIndex, Price: biHigh, Strength: 0.4,
							Features: features.Common(*bi),
						})
					}
				}
			}

			// 突破：向上笔的低点 < ZG 且高点 > ZG（从下方上穿）
			if targets["breakUp"] && bi.IsUp() {
				if biLow < zs.ZG && biHigh > zs.ZG {
					key := "breakUp:" + strconv.Itoa(i)
					if !seen[key] {
						seen[key] = true
						sigs = append(sigs, types.Signal{
							Type: types.BuyPoint3, SubType: types.SubTBreakUp,
							Level: "中枢边界", Index: bi.StartIndex, Price: zs.ZG, Strength: 0.5,
							Features: features.Common(*bi),
						})
					}
				}
			}

			// 跌破：向下笔的高点 > ZD 且低点 < ZD（从上方下穿）
			if targets["breakDn"] && bi.IsDown() {
				if biHigh > zs.ZD && biLow < zs.ZD {
					key := "breakDn:" + strconv.Itoa(i)
					if !seen[key] {
						seen[key] = true
						sigs = append(sigs, types.Signal{
							Type: types.SellPoint3, SubType: types.SubTBreakDn,
							Level: "中枢边界", Index: bi.StartIndex, Price: zs.ZD, Strength: 0.5,
							Features: features.Common(*bi),
						})
					}
				}
			}
		}
	}
	return sigs
}
