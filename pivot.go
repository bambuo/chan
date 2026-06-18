package chanlun

import "math"

// ──────────────────────────────────────────────
// §5  中枢
// ──────────────────────────────────────────────
//
// 中枢是至少三个连续次级别走势类型的重叠区域，代表多空力量均衡区。
// 当前实现以已确认线段作为次级别走势类型的工程承载结构，并在 Pivot 中标记代理口径。
//
// 严格遵循缠论原文定义：
//   ZG（中枢上沿）= min(前两个Z段的高点) —— 中枢形成后锁定不变
//   ZD（中枢下沿）= max(前两个Z段的低点) —— 中枢形成后锁定不变
//   GG（波动最高点）= max(所有线段高点) —— 随延伸/扩展更新
//   DD（波动最低点）= min(所有线段低点) —— 随延伸/扩展更新
//   中枢区间 = [ZD, ZG]，波动区间 = [DD, GG]
//
// Z段判定：
//   上涨中枢（第一段向下→向上→向下）：Z段 = 向下段（第1、3段）
//   下跌中枢（第一段向上→向下→向上）：Z段 = 向上段（第1、3段）
//
// 中枢演变：
//   延伸：价格在中枢区间内震荡，段数<9，级别不变
//   扩展：震荡延伸段数≥9，级别+1
//   扩张：两中枢波动区间[DD,GG]重叠，级别+1
//   破坏：第三类买卖点确认中枢完成
//
// 第三类买卖点（两段结构）：
//   三买：第一段向上离开中枢(ZG)，第二段回抽不触及 ZG
//   三卖：第一段向下离开中枢(ZD)，第二段回抽不触及 ZD

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
		overlapHigh := min(min(s0.Top, s1.Top), s2.Top)
		overlapLow := max(max(s0.Bottom, s1.Bottom), s2.Bottom)

		if overlapHigh >= overlapLow {
			// 确定中枢方向
			// 上涨中枢：第一段向下（形成回调中枢，Z段=向下段）
			// 下跌中枢：第一段向上（形成回升中枢，Z段=向上段）
			pivotDir := determinePivotDirection(s0, s1, s2)

			// ZG/ZD 由前两个 Z 段确定，形成后锁定不变
			zg, zd := calcZGZD(s0, s1, s2, pivotDir)

			// GG/DD 初始为三段全范围
			gg := max(max(s0.Top, s1.Top), s2.Top)
			dd := min(min(s0.Bottom, s1.Bottom), s2.Bottom)

			// PeakHigh/PeakLow：中枢内所有笔的波动极值
			peakHigh, peakLow := calcPivotPeaks([]Segment{s0, s1, s2})

			pivot := Pivot{
				StartIndex:   s0.StartIndex,
				EndIndex:     s2.EndIndex,
				ZG:           zg,
				ZD:           zd,
				GG:           gg,
				DD:           dd,
				PeakHigh:     peakHigh,
				PeakLow:      peakLow,
				Segments:     []Segment{s0, s1, s2},
				OverlapCount: 3,
				Level:        1,
				SourceLevel:  "segment",
				IsProxy:      true,
				State:        PivotFormed,
				Direction:    pivotDir,
			}

			// 尝试延伸中枢
			pivot, nextOffset := extendPivot(pivot, segments, i+3)
			pivots = append(pivots, pivot)
			i += nextOffset
		} else {
			i++
		}
	}

	return pivots
}

// determinePivotDirection 根据前三段判定中枢方向。
//
// 注意：返回值以 Z 段方向为准（而非价格趋势方向），具体对应关系：
//
//	DirDown = 上涨中枢（第一段向下，Z段=向下段，常见于上涨趋势中的回调中枢）
//	DirUp   = 下跌中枢（第一段向上，Z段=向上段，常见于下跌趋势中的回升中枢）
//
// 使用示例：
//
//	上涨趋势中出现回调中枢 → pivot.Direction = DirDown（Z段方向=向下）
//	下跌趋势中出现回升中枢 → pivot.Direction = DirUp（Z段方向=向上）
//
// 无法判定时，默认 DirUp（对应下跌趋势中的回升中枢，为保守默认值）。
func determinePivotDirection(s0, s1, s2 Segment) Direction {
	// 以第一段方向判定：s0向下→上涨中枢，s0向上→下跌中枢
	if s0.Direction == DirDown {
		return DirDown // 上涨中枢（Z段=向下段）
	}
	if s0.Direction == DirUp {
		return DirUp // 下跌中枢（Z段=向上段）
	}
	// 退而求其次：用第三段反向推断
	if s2.Direction == DirDown {
		return DirDown
	}
	return DirUp
}

// calcZGZD 根据中枢形成段计算中枢区间边界。
//
// 文档 §5.1 精确定义：
//
//	ZG（中枢上沿）= min(g1, g2) —— 仅前两个 Z 走势段的高点
//	ZD（中枢下沿）= max(d1, d2) —— 仅前两个 Z 走势段的低点
//
// Z 段判定（中线为与中枢方向同向的段）：
//   - 上涨中枢（s0↓, s1↑, s2↓）：Z段 = s0, s2（第1、3段）
//   - 下跌中枢（s0↑, s1↓, s2↑）：Z段 = s0, s2（第1、3段）
//
// 两种情况下 Z 段均为 s0 和 s2，s1 为反向段，不参与 ZG/ZD 计算。
// ZG/ZD 在中枢形成后永不改变。
func calcZGZD(s0, s1, s2 Segment, pivotDir Direction) (zg, zd float64) {
	// ZG = min(第一个Z段高点, 第二个Z段高点)
	zg = min(s0.Top, s2.Top)
	// ZD = max(第一个Z段低点, 第二个Z段低点)
	zd = max(s0.Bottom, s2.Bottom)
	return
}

// extendPivot 尝试延伸中枢。
// ZG/ZD 锁定不变。GG/DD 随新线段更新。
// 返回延伸后的中枢和下一个待处理线段的偏移量（相对于原 segments 起始）。
func extendPivot(pivot Pivot, segments []Segment, pos int) (Pivot, int) {
	for pos < len(segments) {
		s := segments[pos]

		// 检查新线段是否与中枢区间 [ZD, ZG] 有重叠
		if s.Top >= pivot.ZD && s.Bottom <= pivot.ZG {
			// 在中枢区间内震荡：延伸
			pivot.Segments = append(pivot.Segments, s)
			pivot.OverlapCount++
			pivot.EndIndex = s.EndIndex

			// 更新波动区间 GG/DD（ZG/ZD 保持不变）
			if s.Top > pivot.GG {
				pivot.GG = s.Top
			}
			if s.Bottom < pivot.DD {
				pivot.DD = s.Bottom
			}

			// 检查是否扩展（段数 ≥ 9）
			if pivot.OverlapCount >= 9 {
				pivot.State = PivotExpanded
				pivot.Level++
			} else if pivot.State == PivotFormed {
				pivot.State = PivotExtending
			}

			pos++
		} else if s.Top < pivot.ZD || s.Bottom > pivot.ZG {
			// 线段完全脱离中枢区间 [ZD, ZG]
			// 检查是否为第三类买卖点（需要两段确认：离开段 + 回抽段）
			thirdBP := checkThirdBuySell(segments, pos, pivot)
			if thirdBP != nil {
				// 确认第三类买卖点：中枢被破坏
				pivot.Segments = append(pivot.Segments, segments[pos]) // 离开段
				if pos+1 < len(segments) {
					pivot.Segments = append(pivot.Segments, segments[pos+1]) // 回抽段
				}
				pivot.OverlapCount += 2
				pivot.EndIndex = thirdBP.confirmIndex
				pivot.State = PivotDestroyed
				if thirdBP.sigType == SellPoint3 && pivot.Direction == DirDown {
					// 下跌中枢被三卖破坏
				}
				return pivot, pos + 2 // 跳过离开段和回抽段
			}
			// 未形成三买卖点：线段脱离但未被确认，仍结束延伸
			break
		} else {
			break
		}
	}

	return pivot, pos
}

// thirdBuySellInfo 第三类买卖点确认信息。
type thirdBuySellInfo struct {
	sigType      SignalType
	confirmIndex int
	confirmPrice float64
}

// checkThirdBuySell 检查是否形成第三类买卖点（两段结构）。
//
// 理论定义：
//
//	三买 = 线段向上离开中枢 + 次级别回抽不触及 ZG
//	三卖 = 线段向下离开中枢 + 次级别回抽不触及 ZD
//
// 检测流程：
//  1. 当前段 s[pos] 是离开段（完全在 ZG 之上或 ZD 之下）
//  2. 下一段 s[pos+1] 是回抽段（反向段，不触及中枢区间边界）
func checkThirdBuySell(segments []Segment, pos int, pivot Pivot) *thirdBuySellInfo {
	if pos >= len(segments) || pos+1 >= len(segments) {
		return nil
	}

	leaveSeg := segments[pos]      // 离开段
	pullbackSeg := segments[pos+1] // 回抽段

	// 三买：离开段向上突破 ZG，回抽段向下但不触及 ZG
	if leaveSeg.Direction == DirUp && leaveSeg.Bottom > pivot.ZG {
		// 回抽段必须是反向（向下），且其低点不触及 ZG
		if pullbackSeg.Direction == DirDown && pullbackSeg.Bottom >= pivot.ZG {
			return &thirdBuySellInfo{
				sigType:      BuyPoint3,
				confirmIndex: pullbackSeg.EndIndex,
				confirmPrice: pullbackSeg.Bottom,
			}
		}
	}

	// 三卖：离开段向下突破 ZD，回抽段向上但不触及 ZD
	if leaveSeg.Direction == DirDown && leaveSeg.Top < pivot.ZD {
		// 回抽段必须是反向（向上），且其高点不触及 ZD
		if pullbackSeg.Direction == DirUp && pullbackSeg.Top <= pivot.ZD {
			return &thirdBuySellInfo{
				sigType:      SellPoint3,
				confirmIndex: pullbackSeg.EndIndex,
				confirmPrice: pullbackSeg.Top,
			}
		}
	}

	return nil
}

// CheckPivotEnlargement 检查两个中枢是否发生扩张。
// 中枢扩张：第二个中枢的波动区间与第一个中枢的波动区间发生重叠。
// 两个本级别中枢叠加产生更高级别中枢。
//
// 使用波动区间 [DD, GG] 而非中枢区间 [ZD, ZG]。
func CheckPivotEnlargement(p1, p2 Pivot) bool {
	// 两个中枢各自的波动全范围 [DD, GG]
	return p1.GG >= p2.DD && p2.GG >= p1.DD
}

// CombinePivots 合并重叠的中枢（移植自 chan.py CZSList.try_combine）。
// mode: "zs" 模式使用中枢区间 [ZD,ZG] 重叠判断；
//
//	"peak" 模式使用波动区间 [DD,GG] 重叠判断。
//
// 合并后，前一个中枢的区间扩大，后一个被删除。
func CombinePivots(pivots []Pivot, mode string) []Pivot {
	if len(pivots) < 2 {
		return pivots
	}

	result := make([]Pivot, 0, len(pivots))
	result = append(result, pivots[0])

	for i := 1; i < len(pivots); i++ {
		last := &result[len(result)-1]
		curr := &pivots[i]

		if canCombinePivots(*last, *curr, mode) {
			// 合并：扩大前一个中枢的区间
			last.ZD = min(last.ZD, curr.ZD)
			last.ZG = max(last.ZG, curr.ZG)
			last.PeakLow = min(last.PeakLow, curr.PeakLow)
			last.PeakHigh = max(last.PeakHigh, curr.PeakHigh)
			last.GG = max(last.GG, curr.GG)
			last.DD = min(last.DD, curr.DD)
			last.EndIndex = curr.EndIndex
			last.Segments = append(last.Segments, curr.Segments...)
			last.OverlapCount += curr.OverlapCount
		} else {
			result = append(result, *curr)
		}
	}

	return result
}

// canCombinePivots 判断两个中枢是否可以合并。
func canCombinePivots(p1, p2 Pivot, mode string) bool {
	switch mode {
	case "peak":
		// 波动区间 [DD,GG] 重叠
		return hasOverlap(p1.PeakLow, p1.PeakHigh, p2.PeakLow, p2.PeakHigh)
	default:
		// "zs" 模式：中枢区间 [ZD,ZG] 重叠
		return hasOverlap(p1.ZD, p1.ZG, p2.ZD, p2.ZG)
	}
}

// hasOverlap 判断两个区间是否有重叠（含相等边界）。
func hasOverlap(low1, high1, low2, high2 float64) bool {
	return low1 <= high2 && low2 <= high1
}

// calcPivotPeaks 从中枢的线段笔列表中计算波动极值（peak_high, peak_low）。
func calcPivotPeaks(segments []Segment) (peakHigh, peakLow float64) {
	peakHigh = math.Inf(-1)
	peakLow = math.Inf(1)
	for _, seg := range segments {
		for _, mb := range seg.BiList {
			if mb.High > peakHigh {
				peakHigh = mb.High
			}
			if mb.Low < peakLow {
				peakLow = mb.Low
			}
		}
	}
	if math.IsInf(peakHigh, -1) {
		peakHigh = 0
	}
	if math.IsInf(peakLow, 1) {
		peakLow = 0
	}
	return
}
