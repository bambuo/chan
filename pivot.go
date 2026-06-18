package chanlun

// ──────────────────────────────────────────────
// §5  中枢
// ──────────────────────────────────────────────
//
// 中枢是三段连续线段的重叠区域，代表多空力量均衡区。
//
// ZG = 中枢上沿 = min(各线段高点)
// ZD = 中枢低沿 = max(各线段低点)
// 中枢区间 = [ZD, ZG]
//
// 中枢有三种演变方式：
//   延伸：价格在中枢区间内震荡，段数<9，级别不变
//   扩展：震荡延伸段数≥9，级别+1
//   扩张：两中枢波动重叠，级别+1

// FindPivots 从线段序列中识别所有中枢。
func FindPivots(segments []Segment) []Pivot {
	if len(segments) < 3 {
		return nil
	}

	pivots := make([]Pivot, 0)
	i := 0

	for i+2 < len(segments) {
		s0 := segments[i]
		s1 := segments[i+1]
		s2 := segments[i+2]

		// 检查三段线段是否有重叠区域
		zg := min(min(s0.Top, s1.Top), s2.Top) // 中枢上沿 = min(高点)
		zd := max(max(s0.Bottom, s1.Bottom), s2.Bottom) // 中枢下沿 = max(低点)

		if zg > zd {
			// 有效的重叠区域，创建中枢
			pivot := Pivot{
				StartIndex:   s0.StartIndex,
				EndIndex:     s2.EndIndex,
				ZG:           zg,
				ZD:           zd,
				Segments:     []Segment{s0, s1, s2},
				OverlapCount: 3,
				Level:        1,
				State:        PivotFormed,
			}

			// 尝试延伸中枢
			pivot = extendPivot(pivot, segments, i+3)

			pivots = append(pivots, pivot)
			i += pivot.OverlapCount - 2 // 重叠的段，向前移动时保留重叠部分
		} else {
			i++
		}
	}

	return pivots
}

// extendPivot 尝试延伸中枢（添加更多重叠线段）。
// 返回延伸后的中枢。
func extendPivot(pivot Pivot, segments []Segment, pos int) Pivot {
	for pos < len(segments) {
		s := segments[pos]

		// 检查新线段是否与当前中枢区间重叠
		if s.Top > pivot.ZD && s.Bottom < pivot.ZG {
			// 在中枢区间内震荡：延伸
			pivot.Segments = append(pivot.Segments, s)
			pivot.OverlapCount++
			pivot.EndIndex = s.EndIndex

			// 更新中枢区间（取更大范围的震荡）
			pivot.ZG = min(pivot.ZG, s.Top)
			pivot.ZD = max(pivot.ZD, s.Bottom)

			// 检查是否扩展（段数 ≥ 9）
			if pivot.OverlapCount >= 9 {
				pivot.State = PivotExpanded
				pivot.Level++
			} else if pivot.State == PivotFormed {
				pivot.State = PivotExtending
			}

			pos++
		} else if s.Top <= pivot.ZD || s.Bottom >= pivot.ZG {
			// 线段脱离中枢区间
			// 检查是否为第三类买卖点
			if isThirdBuySellPoint(s, pivot) {
				pivot.State = PivotDestroyed
				pivot.EndIndex = s.EndIndex
				pivot.Segments = append(pivot.Segments, s)
				pivot.OverlapCount++
				break
			}
			break
		} else {
			break
		}
	}

	return pivot
}

// isThirdBuySellPoint 检查线段是否是第三类买卖点。
// 第三类买点：线段离开中枢后，回抽不进入中枢区间（即不触及 ZG）
// 第三类卖点：线段离开中枢后，回抽不进入中枢区间（即不触及 ZD）
func isThirdBuySellPoint(s Segment, pivot Pivot) bool {
	if s.Direction == DirUp && s.Bottom > pivot.ZG {
		return true // 第三类买点确认
	}
	if s.Direction == DirDown && s.Top < pivot.ZD {
		return true // 第三类卖点确认
	}
	return false
}

// CheckPivotEnlargement 检查两个中枢是否发生扩张。
// 中枢扩张：第二个中枢的波动区间与第一个中枢的波动区间发生重叠。
// 两个本级别中枢叠加产生更高级别中枢。
//
// 波动区间 = 中枢内部所有线段的全范围 [DD, GG]
//   GG = max(各线段高点)
//   DD = min(各线段低点)
func CheckPivotEnlargement(p1, p2 Pivot) bool {
	// 两个中枢各自的波动全范围
	p1High := getSegmentHigh(p1.Segments)
	p1Low := getSegmentLow(p1.Segments)
	p2High := getSegmentHigh(p2.Segments)
	p2Low := getSegmentLow(p2.Segments)

	// 两个波动区间有任何重叠 → 扩张
	return p1High > p2Low && p2High > p1Low
}

func getSegmentHigh(segs []Segment) float64 {
	h := 0.0
	for i, s := range segs {
		if i == 0 || s.Top > h {
			h = s.Top
		}
	}
	return h
}

func getSegmentLow(segs []Segment) float64 {
	l := 0.0
	for i, s := range segs {
		if i == 0 || s.Bottom < l {
			l = s.Bottom
		}
	}
	return l
}
