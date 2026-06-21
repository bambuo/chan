package pivot

import (
	"math"

	"github.com/bambuo/chan/types"
)

// FindBiPivots 从笔序列和线段序列构建笔级中枢。
// 支持 zs_algo: normal / over_seg / auto（对齐 chan.py CZSList.cal_bi_zs）。
func FindBiPivots(bis []types.MergedBi, segments []types.Segment, config types.Config) []types.Pivot {
	return findPivots(bis, segments, config)
}

// UpdateZSInSeg 回填笔级中枢的 BiIn/BiOut/BiList 字段。
func UpdateZSInSeg(bis []types.MergedBi, pivots []types.Pivot) {
	for i := range pivots {
		zs := &pivots[i]
		if zs.BeginBiIdx < 0 || zs.EndBiIdx < 0 || zs.BeginBiIdx >= len(bis) {
			continue
		}
		if zs.BeginBiIdx > 0 {
			biIn := bis[zs.BeginBiIdx-1].Bi
			zs.BiIn = &biIn
		}
		if zs.EndBiIdx+1 < len(bis) {
			biOut := bis[zs.EndBiIdx+1].Bi
			zs.BiOut = &biOut
		}
		if zs.EndBiIdx >= zs.BeginBiIdx {
			zs.BiList = make([]types.Bi, 0, zs.EndBiIdx-zs.BeginBiIdx+1)
			for j := zs.BeginBiIdx; j <= zs.EndBiIdx && j < len(bis); j++ {
				zs.BiList = append(zs.BiList, bis[j].Bi)
			}
		}
	}
}

type builder struct {
	config      types.Config
	zsLst       []types.Pivot
	freeItemLst []int
	bis         []types.MergedBi
	segments    []types.Segment
}

func findPivots(bis []types.MergedBi, segments []types.Segment, config types.Config) []types.Pivot {
	if len(bis) < 3 {
		return nil
	}
	b := &builder{config: config, bis: bis, segments: segments}

	switch config.ZsAlgo {
	case "over_seg":
		return b.findPivotsOverSeg()
	case "auto":
		return b.findPivotsAuto()
	default: // "normal"
		return b.findPivotsNormal()
	}
}

// ──────────────────────────────
// normal 模式 — 逐线段构建中枢
// ──────────────────────────────

func (b *builder) findPivotsNormal() []types.Pivot {
	for _, seg := range b.segments {
		segBiLst := b.collectSegBis(seg)
		if len(segBiLst) == 0 {
			continue
		}
		b.freeItemLst = b.freeItemLst[:0]
		b.addRange(segBiLst, seg.Direction, seg.IsSure)
	}
	if len(b.segments) > 0 {
		last := b.segments[len(b.segments)-1]
		b.freeItemLst = b.freeItemLst[:0]
		remain := b.collectAfter(last.EndIndex)
		rev := types.DirUp
		if last.Direction == types.DirUp {
			rev = types.DirDown
		}
		b.addRange(remain, rev, false)
	}
	if len(b.segments) == 0 && len(b.bis) >= 3 {
		b.freeItemLst = b.freeItemLst[:0]
		b.addRange(b.bis[:], types.DirDown, false)
	}
	return b.zsLst
}

// ──────────────────────────────
// over_seg 模式 — 跨线段连续构建中枢
// ──────────────────────────────

func (b *builder) findPivotsOverSeg() []types.Pivot {
	if len(b.bis) < 3 {
		return nil
	}
	b.freeItemLst = b.freeItemLst[:0]
	startBiIdx := 0
	if len(b.zsLst) > 0 {
		startBiIdx = b.zsLst[len(b.zsLst)-1].EndBiIdx + 1
	}
	for i := startBiIdx; i < len(b.bis); i++ {
		b.updateOverSeg(&b.bis[i], i)
	}
	return b.zsLst
}

// updateOverSeg 对应 Python: CZSList.update_overseg_zs
func (b *builder) updateOverSeg(bi *types.MergedBi, biIdx int) {
	// 尝试延伸最后一个中枢
	if len(b.zsLst) > 0 && len(b.freeItemLst) == 0 {
		last := &b.zsLst[len(b.zsLst)-1]
		// 如果当前笔与上一个中枢的末端笔相邻且在范围内 → 尝试延伸
		if biIdx-last.EndBiIdx <= 1 && b.inRange(last, bi) {
			if pivotTryAddToEnd(last, bi, biIdx) {
				return
			}
		}
		// 如果当前笔在中枢范围内且相邻 → 直接返回（已在中枢内）
		if biIdx-last.EndBiIdx <= 1 && b.inRange(last, bi) {
			return
		}
	}
	// 不能延伸，添加到自由列表尝试构建新中枢
	b.addFree(biIdx, true)
}

// inRange 检查笔是否在中枢范围内（对应 Python: CZS.in_range）
func (b *builder) inRange(zs *types.Pivot, bi *types.MergedBi) bool {
	return types.HasOverlap(zs.ZD, zs.ZG, bi.ZSLow(), bi.ZSHigh(), false)
}

// pivotTryAddToEnd 尝试将笔延伸到最后一个中枢（对应 Python: CZS.try_add_to_end）
func pivotTryAddToEnd(p *types.Pivot, bi *types.MergedBi, biIdx int) bool {
	if !p.IsOneBiZs() && !types.HasOverlap(p.ZD, p.ZG, bi.ZSLow(), bi.ZSHigh(), false) {
		return false
	}
	if p.IsOneBiZs() {
		// 单笔中枢需要重新计算范围
		beg := p.BeginBiIdx
		if beg >= 0 && beg < len(p.BiList) {
			zg, zd := types.CalcZSRangeFromUnits([]types.ZSUnit{&p.BiList[beg], bi})
			p.ZG, p.ZD = zg, zd
		}
	}
	p.EndBiIdx = biIdx
	p.EndIndex = bi.EndIndex
	p.OverlapCount++
	if bi.ZSHigh() > p.PeakHigh {
		p.PeakHigh = bi.ZSHigh()
		p.GG = bi.ZSHigh()
	}
	if bi.ZSLow() < p.PeakLow {
		p.PeakLow = bi.ZSLow()
		p.DD = bi.ZSLow()
	}
	if p.State == types.PivotFormed {
		p.State = types.PivotExtending
	}
	return true
}

// ──────────────────────────────
// auto 模式 — 自适应切换 normal / over_seg
// ──────────────────────────────

func (b *builder) findPivotsAuto() []types.Pivot {
	if len(b.bis) < 3 {
		return nil
	}
	sureSegAppeared := false
	existSureSeg := b.hasSureSegment()

	for _, seg := range b.segments {
		if seg.IsSure {
			sureSegAppeared = true
		}
		if !b.segNeedCal(seg) {
			continue
		}
		segBiLst := b.collectSegBis(seg)
		if len(segBiLst) == 0 {
			continue
		}
		if seg.IsSure || (!sureSegAppeared && existSureSeg) {
			// 确定线段 → 使用 normal 模式
			b.freeItemLst = b.freeItemLst[:0]
			b.addRange(segBiLst, seg.Direction, seg.IsSure)
		} else {
			// 不确定线段 → 切换到 over_seg 模式
			b.freeItemLst = b.freeItemLst[:0]
			startBiIdx := 0
			if len(seg.BiList) > 0 {
				startBiIdx = b.findBiIdxByStart(seg.BiList[0].StartIndex)
			}
			if startBiIdx < 0 {
				startBiIdx = b.findBiIdxByEnd(seg.StartIndex)
				if startBiIdx < 0 {
					startBiIdx = 0
				}
			}
			for i := startBiIdx; i < len(b.bis); i++ {
				b.updateOverSeg(&b.bis[i], i)
			}
			break // 切换到 over_seg 后不再处理后续线段
		}
	}
	// 如果一直在 normal 模式且还有未处理的尾部笔
	if len(b.segments) > 0 {
		last := b.segments[len(b.segments)-1]
		if b.endBiIdx(last) < len(b.bis)-1 {
			b.freeItemLst = b.freeItemLst[:0]
			remain := b.collectAfter(last.EndIndex)
			rev := types.DirUp
			if last.Direction == types.DirUp {
				rev = types.DirDown
			}
			b.addRange(remain, rev, false)
		}
	}
	return b.zsLst
}

// ──────────────────────────────
// 通用方法
// ──────────────────────────────

func (b *builder) collectSegBis(seg types.Segment) []types.MergedBi {
	var r []types.MergedBi
	for i := range b.bis {
		bi := &b.bis[i]
		if bi.StartIndex >= seg.StartIndex && bi.EndIndex <= seg.EndIndex {
			r = append(r, *bi)
		}
	}
	return r
}

func (b *builder) collectAfter(end int) []types.MergedBi {
	var r []types.MergedBi
	for i := range b.bis {
		if b.bis[i].StartIndex > end {
			r = append(r, b.bis[i])
		}
	}
	return r
}

func (b *builder) addRange(lst []types.MergedBi, segDir types.Direction, sure bool) {
	cnt := 0
	for i := range lst {
		bi := &lst[i]
		if bi.Direction == segDir {
			continue
		}
		biIdx := b.findIdx(bi)
		if cnt < 1 {
			b.addFree(biIdx, sure)
			cnt++
		} else {
			b.update(biIdx, sure)
		}
	}
}

func (b *builder) findIdx(t *types.MergedBi) int {
	for i := range b.bis {
		if b.bis[i].StartIndex == t.StartIndex && b.bis[i].EndIndex == t.EndIndex {
			return i
		}
	}
	return -1
}

func (b *builder) update(biIdx int, sure bool) {
	added := false
	if len(b.freeItemLst) == 0 && biIdx >= 0 && biIdx < len(b.bis) {
		added = b.addToEnd(biIdx)
	}
	if added {
		b.tryCombine()
		return
	}
	b.addFree(biIdx, sure)
}

func (b *builder) addFree(biIdx int, sure bool) {
	if biIdx < 0 || biIdx >= len(b.bis) {
		return
	}
	if len(b.freeItemLst) > 0 && biIdx == b.freeItemLst[len(b.freeItemLst)-1] {
		b.freeItemLst = b.freeItemLst[:len(b.freeItemLst)-1]
	}
	b.freeItemLst = append(b.freeItemLst, biIdx)
	zs := b.tryConstruct(b.freeItemLst, sure)
	if zs != nil && zs.BeginBiIdx > 0 {
		b.zsLst = append(b.zsLst, *zs)
		b.freeItemLst = b.freeItemLst[:0]
		b.tryCombine()
	}
}

func (b *builder) tryConstruct(lst []int, sure bool) *types.Pivot {
	if len(lst) == 0 {
		return nil
	}
	// over_seg 模式特殊处理
	if b.config.ZsAlgo == "over_seg" || b.config.ZsAlgo == "auto" {
		return b.tryConstructOverSeg(lst, sure)
	}
	// normal 模式
	l := lst
	if !b.config.OneBiZs {
		if len(l) == 1 {
			return nil
		}
		l = l[len(l)-2:]
	}
	return b.buildPivotFromIdxs(l, sure)
}

// tryConstructOverSeg 对应 Python: CZSList.try_construct_zs(zs_algo="over_seg")
func (b *builder) tryConstructOverSeg(lst []int, sure bool) *types.Pivot {
	if len(lst) < 3 {
		return nil
	}
	l := lst[len(lst)-3:]
	// 检查第一个元素方向是否与父线段一致
	if len(b.segments) > 0 && b.isSameDirAsParentSeg(&b.bis[l[0]], l[0]) {
		l = l[1:]
		if len(l) < 2 {
			return nil
		}
	}
	return b.buildPivotFromIdxs(l, sure)
}

// isSameDirAsParentSeg 检查笔的方向是否与其所在线段方向一致
func (b *builder) isSameDirAsParentSeg(bi *types.MergedBi, biIdx int) bool {
	for _, seg := range b.segments {
		if bi.StartIndex >= seg.StartIndex && bi.EndIndex <= seg.EndIndex {
			return bi.Direction == seg.Direction
		}
	}
	return false
}

func (b *builder) buildPivotFromIdxs(idxs []int, sure bool) *types.Pivot {
	if len(idxs) == 0 {
		return nil
	}
	units := make([]types.ZSUnit, 0, len(idxs))
	for _, idx := range idxs {
		if idx < 0 || idx >= len(b.bis) {
			return nil
		}
		units = append(units, &b.bis[idx])
	}
	zg, zd := types.CalcZSRangeFromUnits(units)
	if !(zg > zd) {
		return nil
	}
	return b.buildPivot(idxs, zg, zd, sure)
}

func (b *builder) buildPivot(idxs []int, zg, zd float64, sure bool) *types.Pivot {
	beg, end := idxs[0], idxs[len(idxs)-1]
	peakHigh, peakLow := math.Inf(-1), math.Inf(1)
	for _, idx := range idxs {
		bi := &b.bis[idx]
		if bi.ZSHigh() > peakHigh {
			peakHigh = bi.ZSHigh()
		}
		if bi.ZSLow() < peakLow {
			peakLow = bi.ZSLow()
		}
	}
	return &types.Pivot{
		StartIndex: b.bis[beg].StartIndex, EndIndex: b.bis[end].EndIndex,
		ZG: zg, ZD: zd, GG: peakHigh, DD: peakLow,
		PeakHigh: peakHigh, PeakLow: peakLow,
		OverlapCount: len(idxs), SourceLevel: "bi",
		State: types.PivotFormed, Direction: b.bis[beg].Direction,
		BeginBiIdx: beg, EndBiIdx: end, IsSure: sure,
	}
}

func (b *builder) addToEnd(biIdx int) bool {
	if len(b.zsLst) == 0 || biIdx < 0 || biIdx >= len(b.bis) {
		return false
	}
	last := &b.zsLst[len(b.zsLst)-1]
	bi := &b.bis[biIdx]
	if !types.HasOverlap(last.ZD, last.ZG, bi.ZSLow(), bi.ZSHigh(), false) {
		return false
	}
	if last.BeginBiIdx == last.EndBiIdx {
		beg := &b.bis[last.BeginBiIdx]
		zg, zd := types.CalcZSRangeFromUnits([]types.ZSUnit{beg, bi})
		last.ZG, last.ZD = zg, zd
	}
	last.EndBiIdx = biIdx
	last.EndIndex = bi.EndIndex
	last.OverlapCount++
	if bi.ZSHigh() > last.PeakHigh {
		last.PeakHigh = bi.ZSHigh()
		last.GG = bi.ZSHigh()
	}
	if bi.ZSLow() < last.PeakLow {
		last.PeakLow = bi.ZSLow()
		last.DD = bi.ZSLow()
	}
	if last.State == types.PivotFormed {
		last.State = types.PivotExtending
	}
	return true
}

func (b *builder) tryCombine() {
	if !b.config.ZsCombine {
		return
	}
	for len(b.zsLst) >= 2 {
		if !combineTwo(&b.zsLst[len(b.zsLst)-2], &b.zsLst[len(b.zsLst)-1], b.config.ZsCombineMode) {
			break
		}
		b.zsLst = b.zsLst[:len(b.zsLst)-1]
	}
}

func combineTwo(zs1, zs2 *types.Pivot, mode string) bool {
	if zs2.BeginBiIdx == zs2.EndBiIdx {
		return false
	}
	if zs2.BeginBiIdx < zs1.EndBiIdx || zs2.BeginBiIdx-zs1.EndBiIdx > 2 {
		return false
	}
	var overlap bool
	if mode == "peak" {
		overlap = types.HasOverlap(zs1.PeakLow, zs1.PeakHigh, zs2.PeakLow, zs2.PeakHigh, false)
	} else {
		overlap = types.HasOverlap(zs1.ZD, zs1.ZG, zs2.ZD, zs2.ZG, true)
	}
	if !overlap {
		return false
	}
	doCombine(zs1, zs2)
	return true
}

func doCombine(zs1, zs2 *types.Pivot) {
	if len(zs1.SubPivots) == 0 {
		s := *zs1
		zs1.SubPivots = append(zs1.SubPivots, s)
	}
	zs1.SubPivots = append(zs1.SubPivots, *zs2)
	if zs2.ZD < zs1.ZD {
		zs1.ZD = zs2.ZD
	}
	if zs2.ZG > zs1.ZG {
		zs1.ZG = zs2.ZG
	}
	if zs2.PeakLow < zs1.PeakLow {
		zs1.PeakLow = zs2.PeakLow
		zs1.DD = zs2.DD
	}
	if zs2.PeakHigh > zs1.PeakHigh {
		zs1.PeakHigh = zs2.PeakHigh
		zs1.GG = zs2.GG
	}
	zs1.EndIndex = zs2.EndIndex
	zs1.EndBiIdx = zs2.EndBiIdx
	zs1.BiOut = zs2.BiOut
	zs1.OverlapCount += zs2.OverlapCount
}

// ── 辅助方法 ──

func (b *builder) hasSureSegment() bool {
	for i := range b.segments {
		if b.segments[i].IsSure {
			return true
		}
	}
	return false
}

func (b *builder) segNeedCal(seg types.Segment) bool {
	// 始终需要计算（简化处理）
	return true
}

func (b *builder) endBiIdx(seg types.Segment) int {
	for i := range b.bis {
		if b.bis[i].EndIndex == seg.EndIndex {
			return i
		}
	}
	return -1
}

func (b *builder) findBiIdxByStart(startIndex int) int {
	for i := range b.bis {
		if b.bis[i].StartIndex == startIndex {
			return i
		}
	}
	return -1
}

func (b *builder) findBiIdxByEnd(endIndex int) int {
	for i := range b.bis {
		if b.bis[i].EndIndex == endIndex {
			return i
		}
	}
	return -1
}
