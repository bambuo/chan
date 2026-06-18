package chanlun

// ──────────────────────────────────────────────
// Def 线段划分算法（简化版）
// ──────────────────────────────────────────────
//
// 移植自 chan.py SegListDef。
// 基于相邻笔的高低点关系判断线段方向变化。
// 方向反转时切割线段，逻辑简洁适合快速分析。

// defSegmentBuilder 实现 Def 简化线段划分算法。
type defSegmentBuilder struct{}

func (b *defSegmentBuilder) BuildSegments(bis []MergedBi) []Segment {
	if len(bis) < 3 {
		return nil
	}

	segments := make([]Segment, 0)
	var peakBi *MergedBi
	peakIdx := -1

	for idx := 0; idx < len(bis); idx++ {
		if idx < 2 {
			continue
		}

		bi := &bis[idx]
		preBi := &bis[idx-2]

		// 更新 peak：如果同向且更极端
		if peakBi != nil &&
			((bi.Direction == DirUp && peakBi.Direction == DirUp && bi.High >= peakBi.High) ||
				(bi.Direction == DirDown && peakBi.Direction == DirDown && bi.Low <= peakBi.Low)) {
			peakBi = bi
			peakIdx = idx
			continue
		}

		// 检查是否构成新线段（当前笔与隔两笔的比较）
		isUpSeg := bi.High > preBi.High
		isDownSeg := bi.Low < preBi.Low

		if (bi.Direction == DirUp && isUpSeg) || (bi.Direction == DirDown && isDownSeg) {
			if peakBi == nil {
				// 初始 peak
				if len(segments) == 0 || bi.Direction != segments[len(segments)-1].Direction {
					peakBi = bi
					peakIdx = idx
					continue
				}
			} else if peakBi.Direction != bi.Direction {
				// 方向反转：切割线段
				if idx-peakIdx > 2 {
					seg := buildDefSegment(bis, segments, peakIdx)
					if seg != nil {
						segments = append(segments, *seg)
					}
					peakBi = bi
					peakIdx = idx
					continue
				}
			}
		}
	}

	// 处理尾部不确定线段
	if peakBi != nil {
		seg := buildDefSegment(bis, segments, peakIdx)
		if seg != nil {
			seg.IsBroken = false // 不确定
			segments = append(segments, *seg)
		}
	}

	return segments
}

// buildDefSegment 从已有线段尾部到 peakIdx 构建新线段。
func buildDefSegment(bis []MergedBi, existingSegs []Segment, peakIdx int) *Segment {
	startIdx := 0
	if len(existingSegs) > 0 {
		lastSeg := existingSegs[len(existingSegs)-1]
		// 找到最后一笔的下一个位置
		for i, b := range bis {
			if b.EndIndex == lastSeg.EndIndex {
				startIdx = i + 1
				break
			}
		}
	}

	if startIdx > peakIdx || peakIdx >= len(bis) {
		return nil
	}

	biList := make([]MergedBi, 0, peakIdx-startIdx+1)
	top := bis[startIdx].High
	bottom := bis[startIdx].Low

	for i := startIdx; i <= peakIdx; i++ {
		biList = append(biList, bis[i])
		if bis[i].High > top {
			top = bis[i].High
		}
		if bis[i].Low < bottom {
			bottom = bis[i].Low
		}
	}

	if len(biList) < 3 {
		return nil
	}

	return &Segment{
		StartIndex: biList[0].StartIndex,
		EndIndex:   biList[len(biList)-1].EndIndex,
		Direction:  biList[0].Direction,
		BiList:     biList,
		Top:        top,
		Bottom:     bottom,
		IsBroken:   true,
		BreakType:  BreakStd,
	}
}
