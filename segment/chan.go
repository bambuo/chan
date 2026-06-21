package segment

import (
	"math"

	"github.com/bambuo/chan/types"
)

// buildChan 使用特征序列分形（Eigen FX）方法构建线段。
// 对齐 chan.py CSegListChan.cal_seg_sure + treat_fx_eigen。
//
// 核心思路：双 EigenFX — upEigen 负责上升线段（处理下降笔），
// downEigen 负责下降线段（处理上升笔），自适应确定第一段方向。
func buildChan(bis []types.MergedBi) []types.Segment {
	if len(bis) < 3 {
		return nil
	}
	var segs []types.Segment
	idx := 0
	for idx < len(bis) {
		s, n := tryBuildSeg(bis, idx)
		if s == nil {
			idx = n
			continue
		}
		segs = append(segs, *s)
		idx = n
	}
	return segs
}

// tryBuildSeg 从 startBiIdx 开始尝试构建一个线段。
// 返回线段及下一个待处理的笔索引。
//
// 对应 Python: cal_seg_sure 中 for bi in bi_lst[begin_idx:] 循环 + treat_fx_eigen
func tryBuildSeg(bis []types.MergedBi, start int) (*types.Segment, int) {
	upEigen := newEigenFX(types.DirUp)     // 上升线段 ← 处理下降笔
	downEigen := newEigenFX(types.DirDown) // 下降线段 ← 处理上升笔
	segDir := types.DirNone

	for i := start; i < len(bis); i++ {
		bi := bis[i]

		// ── 选择对应的 EigenFX ──
		var fxEigen *eigenFX
		var result eigenFXBreakResult

		if bi.Direction == types.DirDown && segDir != types.DirUp {
			result, _ = upEigen.add(bi, bis, i)
			if result != eigenNoBreak {
				fxEigen = upEigen
			}
		} else if bi.Direction == types.DirUp && segDir != types.DirDown {
			result, _ = downEigen.add(bi, bis, i)
			if result != eigenNoBreak {
				fxEigen = downEigen
			}
		}

		// ── 自适应确定第一段方向（对应 Python 核心逻辑）──
		if segDir == types.DirNone {
			if upEigen.HasElement(1) && bi.Direction == types.DirDown {
				// up_eigen 形成第二元素 + 当前是下降笔 → 方向为 DOWN
				segDir = types.DirDown
				downEigen.clear()
			} else if downEigen.HasElement(1) && bi.Direction == types.DirUp {
				// down_eigen 形成第二元素 + 当前是上升笔 → 方向为 UP
				segDir = types.DirUp
				upEigen.clear()
			}
			// 如果方向被设定但对应 eigen 丢失了 element[1] → 重置
			if !upEigen.HasElement(1) && segDir == types.DirDown && bi.Direction == types.DirDown {
				segDir = types.DirNone
			} else if !downEigen.HasElement(1) && segDir == types.DirUp && bi.Direction == types.DirUp {
				segDir = types.DirNone
			}
		}

		// ── EigenFX 形成分形 → 检查线段是否结束 ──
		if fxEigen != nil {
			shouldEnd, isSure := fxEigen.canBeEnd(bis, i)
			// 对应 Python: if _test in [True, None] → 构建线段
			if shouldEnd || (!shouldEnd && !isSure) {
				endBiIdx := fxEigen.getPeakIdx()
				if endBiIdx < start {
					endBiIdx = start
				}
				if endBiIdx > i {
					endBiIdx = i
				}

				isTrue := shouldEnd && isSure && fxEigen.allBiIsSure()
				seg := buildSegFromRange(bis, start, endBiIdx, segDir, isTrue, fxEigen)

				// 验证线段有效性，防止首段方向与首尾值异常
				// 对应 Python: if not add_new_seg → retry from end_bi_idx+1
				if ok := validateSeg(seg, bis, start, endBiIdx, segDir); !ok {
					return nil, endBiIdx + 1
				}

				next := endBiIdx + 1
				return &seg, next
			}
			// 对应 Python: cfg.BspType 的 False 分支 → 从第二元素开始重算
			if !shouldEnd {
				segDir = types.DirNone
			}
		}
	}
	return nil, len(bis)
}

// buildSegFromRange 从笔范围构造 Segment 对象。
func buildSegFromRange(bis []types.MergedBi, startBiIdx, endBiIdx int, dir types.Direction, isSure bool, fx *eigenFX) types.Segment {
	biList := make([]types.MergedBi, 0, endBiIdx-startBiIdx+1)
	top := math.Inf(-1)
	bottom := math.Inf(1)
	for i := startBiIdx; i <= endBiIdx && i < len(bis); i++ {
		biList = append(biList, bis[i])
		if bis[i].High > top {
			top = bis[i].High
		}
		if bis[i].Low < bottom {
			bottom = bis[i].Low
		}
	}
	seg := types.Segment{
		StartIndex: bis[startBiIdx].StartIndex,
		EndIndex:   bis[endBiIdx].EndIndex,
		Direction:  dir,
		BiList:     biList,
		Top:        top,
		Bottom:     bottom,
		IsSure:     isSure,
	}
	// 检查破坏类型
	if endBiIdx+1 < len(bis) {
		nextBi := bis[endBiIdx+1]
		if nextBi.Direction != dir {
			if fx != nil && fx.IsGapElement(1) {
				seg.IsBroken = true
				seg.BreakType = types.BreakStroke
				idx := fx.getPeakIdx()
				if idx >= 0 && idx < len(bis) {
					seg.ConfirmIndex = bis[idx].EndIndex
				} else {
					seg.ConfirmIndex = nextBi.EndIndex
				}
			} else {
				seg.IsBroken = true
				seg.BreakType = types.BreakStd
				seg.ConfirmIndex = nextBi.EndIndex
			}
		}
	}
	return seg
}

// validateSeg 验证线段有效性（对齐 Python CSeg.check）。
// Python 对 sure 线段检查方向和长度，对 unsure 只做方向检查。
func validateSeg(seg types.Segment, bis []types.MergedBi, startBiIdx, endBiIdx int, segDir types.Direction) bool {
	if endBiIdx-startBiIdx < 1 {
		// 至少需要 2 笔才能构成线段
		return false
	}
	if seg.IsSure {
		// 确定线段必须满足方向一致性
		firstBi := bis[startBiIdx]
		lastBi := bis[endBiIdx]
		if segDir == types.DirDown {
			if firstBi.BeginVal() <= lastBi.EndVal() {
				return false
			}
		} else {
			if firstBi.BeginVal() >= lastBi.EndVal() {
				return false
			}
		}
		// 足够长度：Python 要求 end_bi.idx - start_bi.idx >= 2 (至少3笔)
		if endBiIdx-startBiIdx < 2 {
			return false
		}
	}
	return true
}

// allBiIsSure 检查 eigenFX 中所有涉及笔是否确定。
// 对应 Python: CEigenFX.all_bi_is_sure
func (fx *eigenFX) allBiIsSure() bool {
	// 当前简化实现：只要存在分形即为确定
	return fx.lastEvidentBiSure
}
