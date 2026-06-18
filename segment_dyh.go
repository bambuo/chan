package chanlun

import "math"

// ──────────────────────────────────────────────
// DYH 线段划分算法
// ──────────────────────────────────────────────
//
// 移植自 chan.py SegListDYH。
// 基于 situation1/situation2 两种模式判定线段端点。
//
// situation1: 当前笔比前前笔更不极端，下一笔反向确认
// situation2: 当前笔比前前笔更极端，下一笔深度确认

// dyhSegmentBuilder 实现 DYH 线段划分算法。
type dyhSegmentBuilder struct{}

func (b *dyhSegmentBuilder) BuildSegments(bis []MergedBi) []Segment {
	if len(bis) < 3 {
		return nil
	}

	segments := make([]Segment, 0)
	nextBeginBi := bis[0]

	for idx := 0; idx < len(bis); idx++ {
		if idx+2 >= len(bis) || idx < 2 {
			continue
		}

		curBi := bis[idx]
		nextBi := bis[idx+2]
		preBi := bis[idx-2]

		// 方向约束：新线段起始方向必须与前一结束笔方向不同
		if len(segments) > 0 && curBi.Direction != segments[len(segments)-1].Direction {
			// 检查：方向相同时跳过（DYH 要求 curBi.dir == 上一线段结束笔方向 才跳过）
			// 但这里 curBi.Direction != 上一线段结束方向 说明可以开始新线段
			_ = curBi
		}

		// 重叠检查
		if curBi.Direction == DirDown && bis[idx-1].High < nextBeginBi.Low {
			continue
		}
		if curBi.Direction == DirUp && bis[idx-1].Low > nextBeginBi.High {
			continue
		}

		// 间距检查：至少 4 笔间隔
		if len(segments) > 0 {
			lastSeg := segments[len(segments)-1]
			if idx-lastSeg.endBiIndex(bis) < 4 {
				continue
			}
		}

		// situation1 或 situation2 判定
		if dyhSituation1(curBi.Bi, nextBi.Bi, preBi.Bi) || dyhSituation2(curBi.Bi, nextBi.Bi, preBi.Bi) {
			seg := buildDyhSegment(bis, nextBeginBi, bis[idx-1])
			if seg != nil {
				segments = append(segments, *seg)
			}
			nextBeginBi = curBi
		}
	}

	// 处理尾部不确定线段
	if len(segments) > 0 {
		lastSeg := &segments[len(segments)-1]
		unsureSeg := buildDyhUnsureSegment(bis, lastSeg.endBiIndex(bis)+1)
		if unsureSeg != nil {
			segments = append(segments, *unsureSeg)
		}
	}

	return segments
}

// endBiIndex 返回线段最后一笔在 bis 中的索引。
func (s *Segment) endBiIndex(bis []MergedBi) int {
	if len(s.BiList) == 0 {
		return 0
	}
	lastBi := s.BiList[len(s.BiList)-1]
	for i, b := range bis {
		if b.StartIndex == lastBi.StartIndex && b.EndIndex == lastBi.EndIndex {
			return i
		}
	}
	return 0
}

// dyhSituation1 判断 situation1（当前笔不极端，下一笔反向确认）。
func dyhSituation1(curBi, nextBi, preBi Bi) bool {
	if curBi.Direction == DirDown && curBi.Low > preBi.Low {
		if nextBi.High < curBi.High && nextBi.Low < curBi.Low {
			return true
		}
	} else if curBi.Direction == DirUp && curBi.High < preBi.High {
		if nextBi.Low > curBi.Low && nextBi.High > curBi.High {
			return true
		}
	}
	return false
}

// dyhSituation2 判断 situation2（当前笔极端，下一笔深度确认）。
func dyhSituation2(curBi, nextBi, preBi Bi) bool {
	if curBi.Direction == DirDown && curBi.Low < preBi.Low {
		if nextBi.High < curBi.High && nextBi.Low < preBi.Low {
			return true
		}
	} else if curBi.Direction == DirUp && curBi.High > preBi.High {
		if nextBi.Low > curBi.Low && nextBi.High > preBi.High {
			return true
		}
	}
	return false
}

// buildDyhSegment 从 startBi 到 endBi 构建一个确定线段。
func buildDyhSegment(bis []MergedBi, startBi, endBi MergedBi) *Segment {
	biList := make([]MergedBi, 0)
	inRange := false
	for _, b := range bis {
		if b.StartIndex == startBi.StartIndex {
			inRange = true
		}
		if inRange {
			biList = append(biList, b)
		}
		if b.StartIndex == endBi.StartIndex {
			break
		}
	}

	if len(biList) < 3 {
		return nil
	}

	top := biList[0].High
	bottom := biList[0].Low
	for _, b := range biList {
		if b.High > top {
			top = b.High
		}
		if b.Low < bottom {
			bottom = b.Low
		}
	}

	return &Segment{
		StartIndex: startBi.StartIndex,
		EndIndex:   endBi.EndIndex,
		Direction:  startBi.Direction,
		BiList:     biList,
		Top:        top,
		Bottom:     bottom,
		IsBroken:   true,
		BreakType:  BreakStd,
	}
}

// buildDyhUnsureSegment 构建尾部不确定线段（取最极端反向笔）。
func buildDyhUnsureSegment(bis []MergedBi, startIdx int) *Segment {
	if startIdx >= len(bis) || startIdx < 1 {
		return nil
	}

	startBi := bis[startIdx-1]
	if startIdx >= len(bis) {
		return nil
	}

	lastSegDir := startBi.Direction
	endBiIdx := -1
	peakValue := math.Inf(1)
	if lastSegDir == DirDown {
		peakValue = math.Inf(-1)
	}

	for i := startIdx + 1; i < len(bis); i++ {
		bi := bis[i]
		if bi.Direction == lastSegDir {
			continue
		}
		if lastSegDir == DirUp && bi.Low < peakValue {
			endBiIdx = i
			peakValue = bi.Low
		} else if lastSegDir == DirDown && bi.High > peakValue {
			endBiIdx = i
			peakValue = bi.High
		}
	}

	if endBiIdx < 0 {
		return nil
	}

	biList := make([]MergedBi, 0)
	top := startBi.High
	bottom := startBi.Low
	for i := startIdx - 1; i <= endBiIdx; i++ {
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
		StartIndex: startBi.StartIndex,
		EndIndex:   bis[endBiIdx].EndIndex,
		Direction:  startBi.Direction,
		BiList:     biList,
		Top:        top,
		Bottom:     bottom,
	}
}
