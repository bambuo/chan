package segment

import (
	"math"

	"github.com/bambuo/chan/types"
)

// CollectLeftSeg 收集线段尾部剩余笔形成不确定线段。
// 对齐 chan.py CSegListComm.collect_left_seg 逻辑。
func CollectLeftSeg(segments []types.Segment, bis []types.MergedBi, method string) []types.Segment {
	if len(bis) < 3 {
		return segments
	}
	if len(segments) == 0 {
		return collectFirstSeg(bis, method)
	}
	result := make([]types.Segment, len(segments))
	copy(result, segments)
	result = collectTailSegs(result, bis)
	return result
}

// collectFirstSeg 当没有任何已确认线段时，从全部笔推断第一段。
// 对应 Python: collect_first_seg (LEFT_SEG_METHOD.PEAK / ALL)
func collectFirstSeg(bis []types.MergedBi, method string) []types.Segment {
	if len(bis) < 1 {
		return nil
	}
	var segs []types.Segment
	if method == "peak" && len(bis) >= 3 {
		// 计算第一笔起始点和全局极值，决定方向
		firstBegin := bis[0].BeginVal()
		high := math.Inf(-1)
		low := math.Inf(1)
		for i := range bis {
			if bis[i].ZSHigh() > high {
				high = bis[i].ZSHigh()
			}
			if bis[i].ZSLow() < low {
				low = bis[i].ZSLow()
			}
		}
		if !math.IsInf(high, -1) && !math.IsInf(low, 1) {
			var peakIdx int
			var segDir types.Direction
			if math.Abs(high-firstBegin) >= math.Abs(low-firstBegin) {
				peakIdx = findPeakBiIdx(bis, true)
				segDir = types.DirUp
			} else {
				peakIdx = findPeakBiIdx(bis, false)
				segDir = types.DirDown
			}
			if peakIdx >= 0 {
				s := newUncertainSeg(bis, 0, peakIdx, segDir, "0seg_find_peak")
				segs = append(segs, s)
				// 收集剩余笔（对齐 Python: 即使是单笔也收集）
				remain := bis[peakIdx+1:]
				if len(remain) > 0 {
					tailSeg := newUncertainSeg(bis, peakIdx+1, peakIdx+len(remain), revertDir(segDir), "0seg_collect_remain")
					segs = append(segs, tailSeg)
				}
			}
		}
	} else if method == "all" {
		// 全部收集为一段
		lastIdx := len(bis) - 1
		segDir := types.DirUp
		if bis[lastIdx].EndVal() < bis[0].BeginVal() {
			segDir = types.DirDown
		}
		s := newUncertainSeg(bis, 0, lastIdx, segDir, "0seg_collect_all")
		segs = append(segs, s)
	}
	return segs
}

// collectTailSegs 收集最后一个线段之后的笔，形成尾部不确定线段。
// 对应 Python: collect_segs + collect_left_seg_peak_method + collect_left_as_seg
func collectTailSegs(segs []types.Segment, bis []types.MergedBi) []types.Segment {
	if len(segs) == 0 {
		return segs
	}
	lastSeg := &segs[len(segs)-1]
	lastEndBiIdx := findEndBiIdx(bis, lastSeg.EndIndex)
	if lastEndBiIdx < 0 || lastEndBiIdx+1 >= len(bis) {
		return segs
	}
	remain := bis[lastEndBiIdx+1:]
	// 对齐 Python: 即使只有 1 笔也允许收集为不确定线段
	if len(remain) == 0 {
		return segs
	}
	lastBi := remain[len(remain)-1]

	// 场景：最后一个线段方向与剩余笔的尾部比较
	if lastSeg.Direction == types.DirDown && lastBi.EndVal() <= lastSeg.EndVal() && len(remain) >= 3 {
		// 下降线段 + 尾部没有超过 → 找上升笔峰值
		if peakIdx := findPeakBiIdx(remain, true); peakIdx >= 0 {
			absIdx := lastEndBiIdx + 1 + peakIdx
			if absIdx-lastEndBiIdx >= 3 {
				s := newUncertainSeg(bis, lastEndBiIdx+1, absIdx, types.DirUp, "collectleft_find_high_force")
				segs = append(segs, s)
				// 递归收集
				return collectTailSegs(segs, bis)
			}
		}
	} else if lastSeg.Direction == types.DirUp && lastBi.EndVal() >= lastSeg.EndVal() && len(remain) >= 3 {
		// 上升线段 + 尾部没有低于 → 找下降笔峰值
		if peakIdx := findPeakBiIdx(remain, false); peakIdx >= 0 {
			absIdx := lastEndBiIdx + 1 + peakIdx
			if absIdx-lastEndBiIdx >= 3 {
				s := newUncertainSeg(bis, lastEndBiIdx+1, absIdx, types.DirDown, "collectleft_find_low_force")
				segs = append(segs, s)
				// 递归收集
				return collectTailSegs(segs, bis)
			}
		}
	}
	// 常规 peak 方法（需要 ≥3 笔）
	if len(remain) >= 3 {
		segs = collectLeftPeakMethod(segs, bis, lastEndBiIdx, remain)
	} else {
		// 对齐 Python: 剩余笔不足 3 笔时，直接用 collect_left_as_seg 收尾
		segs = collectLeftAsSeg(segs, bis, lastEndBiIdx, remain)
	}
	return segs
}

// collectLeftPeakMethod 对应 Python: collect_left_seg_peak_method + collect_left_as_seg
func collectLeftPeakMethod(segs []types.Segment, bis []types.MergedBi, lastEndBiIdx int, remain []types.MergedBi) []types.Segment {
	if len(remain) < 3 {
		return segs
	}
	lastSeg := &segs[len(segs)-1]
	peakIdx := -1
	var segDir types.Direction

	if lastSeg.Direction == types.DirDown {
		peakIdx = findPeakBiIdx(remain, true)
		segDir = types.DirUp
	} else {
		peakIdx = findPeakBiIdx(remain, false)
		segDir = types.DirDown
	}

	if peakIdx >= 0 {
		absPeak := lastEndBiIdx + 1 + peakIdx
		if absPeak-lastEndBiIdx >= 3 {
			s := newUncertainSeg(bis, lastEndBiIdx+1, absPeak, segDir, "collectleft_find_peak")
			segs = append(segs, s)
			// 递归继续
			return collectTailSegs(segs, bis)
		}
	}
	// 找不到峰值，用全部剩余笔收尾
	return collectLeftAsSeg(segs, bis, lastEndBiIdx, remain)
}

// collectLeftAsSeg 对应 Python: collect_left_as_seg
func collectLeftAsSeg(segs []types.Segment, bis []types.MergedBi, lastEndBiIdx int, remain []types.MergedBi) []types.Segment {
	if len(segs) == 0 {
		return segs
	}
	lastSeg := &segs[len(segs)-1]
	lastBi := remain[len(remain)-1]
	remainEndIdx := lastEndBiIdx + len(remain)

	if lastEndBiIdx+1 >= len(bis) {
		return segs
	}

	var endBiIdx int
	if lastSeg.Direction == lastBi.Direction {
		endBiIdx = remainEndIdx - 1
		if endBiIdx <= lastEndBiIdx {
			endBiIdx = remainEndIdx
		}
	} else {
		endBiIdx = remainEndIdx
	}
	if endBiIdx <= lastEndBiIdx {
		endBiIdx = lastEndBiIdx + 1
	}
	if endBiIdx >= len(bis) {
		endBiIdx = len(bis) - 1
	}

	// 确定方向
	segDir := lastSeg.Direction
	if segDir != bis[lastEndBiIdx+1].Direction {
		segDir = bis[lastEndBiIdx+1].Direction
	}

	s := newUncertainSeg(bis, lastEndBiIdx+1, endBiIdx, segDir, "collect_left_as_seg")
	segs = append(segs, s)
	return segs
}

// newUncertainSeg 创建一个不确定线段（is_sure=false）。
func newUncertainSeg(bis []types.MergedBi, startBiIdx, endBiIdx int, dir types.Direction, reason string) types.Segment {
	top := math.Inf(-1)
	bottom := math.Inf(1)
	for i := startBiIdx; i <= endBiIdx && i < len(bis); i++ {
		if bis[i].High > top {
			top = bis[i].High
		}
		if bis[i].Low < bottom {
			bottom = bis[i].Low
		}
	}
	biList := make([]types.MergedBi, 0, endBiIdx-startBiIdx+1)
	for i := startBiIdx; i <= endBiIdx && i < len(bis); i++ {
		biList = append(biList, bis[i])
	}
	return types.Segment{
		StartIndex:   bis[startBiIdx].StartIndex,
		EndIndex:     bis[endBiIdx].EndIndex,
		Direction:    dir,
		BiList:       biList,
		Top:          top,
		Bottom:       bottom,
		IsBroken:     false,
		BreakType:    types.BreakNone,
		ConfirmIndex: -1,
		IsSure:       false,
	}
}

// findPeakBiIdx 寻找上升(=峰值)/下降(=谷值)笔的索引。
// 对应 Python: FindPeakBi
func findPeakBiIdx(bis []types.MergedBi, isHigh bool) int {
	if isHigh {
		peakVal := math.Inf(-1)
		peakIdx := -1
		for i := range bis {
			bi := &bis[i]
			if bi.Direction == types.DirUp && bi.EndPrice >= peakVal {
				// 检查前前笔是否有更高的端点
				if i >= 2 && bis[i-2].Direction == types.DirUp && bis[i-2].EndPrice > bi.EndPrice {
					continue
				}
				peakVal = bi.EndPrice
				peakIdx = i
			}
		}
		return peakIdx
	}
	peakVal := math.Inf(1)
	peakIdx := -1
	for i := range bis {
		bi := &bis[i]
		if bi.Direction == types.DirDown && bi.EndPrice <= peakVal {
			if i >= 2 && bis[i-2].Direction == types.DirDown && bis[i-2].EndPrice < bi.EndPrice {
				continue
			}
			peakVal = bi.EndPrice
			peakIdx = i
		}
	}
	return peakIdx
}

// findEndBiIdx 在笔序列中查找指定 EndIndex 对应的笔索引。
func findEndBiIdx(bis []types.MergedBi, endIndex int) int {
	for i := range bis {
		if bis[i].EndIndex == endIndex {
			return i
		}
	}
	return -1
}

func revertDir(d types.Direction) types.Direction {
	if d == types.DirUp {
		return types.DirDown
	}
	if d == types.DirDown {
		return types.DirUp
	}
	return types.DirNone
}
