package chanlun

// ──────────────────────────────────────────────
// §6  走势类型与走势必完美
// ──────────────────────────────────────────────
//
// 走势必完美（第一原理）：
//   任何级别的任何走势类型终将完成。
//   - 上涨趋势必以顶背驰结束
//   - 下跌趋势必以底背驰结束
//   - 盘整必以第三类买卖点结束
//
// 走势类型分类：
//   - 上涨趋势：包含两个或以上依次向上的中枢，中枢之间无重叠
//   - 下跌趋势：包含两个或以上依次向下的中枢，中枢之间无重叠
//   - 盘整：仅包含一个中枢

// ClassifyTrends 从中枢列表分析走势类型。
// 返回连续的走势类型列表。
func ClassifyTrends(pivots []Pivot) []Trend {
	if len(pivots) == 0 {
		return nil
	}

	trends := make([]Trend, 0)
	i := 0

	for i < len(pivots) {
		trend, nextIdx := classifyFrom(pivots, i)
		if trend == nil {
			i++
			continue
		}
		trends = append(trends, *trend)
		i = nextIdx
	}

	return trends
}

// classifyFrom 从位置 i 开始分析走势类型。
// 支持盘整 + 上涨/下跌趋势
func classifyFrom(pivots []Pivot, i int) (*Trend, int) {
	if i >= len(pivots) {
		return nil, i
	}

	// 单个中枢 → 盘整
	if i == len(pivots)-1 {
		p := pivots[i]
		return &Trend{
			Type:       RangeOnly,
			Pivots:     []Pivot{p},
			StartIndex: p.StartIndex,
			EndIndex:   p.EndIndex,
		}, i + 1
	}

	p0 := pivots[i]
	p1 := pivots[i+1]

		// 检查两个中枢的方向
		if isUpTrend(p0, p1) {
			// 上涨趋势：依次向上的中枢，ZD(N) ≥ ZG(N-1)
			t := buildTrend(TrendUp, pivots, i)
			return t, i + len(t.Pivots)
		}
		if isDownTrend(p0, p1) {
			// 下跌趋势：依次向下的中枢，ZG(N) ≤ ZD(N-1)
			t := buildTrend(TrendDown, pivots, i)
			return t, i + len(t.Pivots)
		}

	// 中枢之间无明确方向关系 → 独立盘整
	return &Trend{
		Type:       RangeOnly,
		Pivots:     []Pivot{p0},
		StartIndex: p0.StartIndex,
		EndIndex:   p0.EndIndex,
	}, i + 1
}

// isUpTrend 检查两个中枢是否构成上涨趋势。
// 条件：p1 的 ZD ≥ p0 的 ZG（中枢之间不重叠，且 p1 在 p0 上方）
func isUpTrend(p0, p1 Pivot) bool {
	return p1.ZD >= p0.ZG
}

// isDownTrend 检查两个中枢是否构成下跌趋势。
// 条件：p1 的 ZG ≤ p0 的 ZD（中枢之间不重叠，且 p1 在 p0 下方）
func isDownTrend(p0, p1 Pivot) bool {
	return p1.ZG <= p0.ZD
}

// buildTrend 从起始位置构建趋势（上涨或下跌）。
// 连续收集方向一致的中枢。
func buildTrend(trendType TrendType, pivots []Pivot, start int) *Trend {
	trend := &Trend{
		Type:       trendType,
		Pivots:     []Pivot{pivots[start]},
		StartIndex: pivots[start].StartIndex,
		EndIndex:   pivots[start].EndIndex,
	}

	for i := start + 1; i < len(pivots); i++ {
		prev := trend.Pivots[len(trend.Pivots)-1]
		curr := pivots[i]

		var match bool
		if trendType == TrendUp {
			match = isUpTrend(prev, curr)
		} else {
			match = isDownTrend(prev, curr)
		}

		if !match {
			break
		}

		trend.Pivots = append(trend.Pivots, curr)
		trend.EndIndex = curr.EndIndex
	}

	// 走势是否完成需通过背驰确认（走势必完美定理）
	// 上涨必以顶背驰结束，下跌必以底背驰结束，盘整必以第三类买卖点结束
	// 此处仅做初步标记，最终由 deviation.go 中的 DetectTrendDeviations 确认
	// 并在 engine.go 中调用 MarkTrendComplete 更新
	if len(trend.Pivots) >= 1 {
		lastPivot := trend.Pivots[len(trend.Pivots)-1]
		if lastPivot.State == PivotDestroyed {
			// 中枢被三买卖点破坏是走势可能完成的信号，但需要背驰最终确认
			// 标记 CompleteReason 供后续背驰检测参考
			trend.CompleteReason = "中枢被第三类买卖点破坏（待背驰确认）"
		}
	}

	return trend
}

// IsTrendComplete 基于走势必完美定理判断走势是否完成。
// 该函数可在背驰检测后被调用，更新趋势的完成状态。
func IsTrendComplete(trend *Trend) bool {
	if trend == nil {
		return false
	}
	return trend.IsComplete
}

// MarkTrendComplete 基于背驰确认标记走势完成。
//
// 走势必完美定理：
//   上涨趋势 → 顶背驰确认完成
//   下跌趋势 → 底背驰确认完成
//   盘整     → 第三类买卖点确认完成
//
// 调用时机：在 DetectTrendDeviations 发现趋势背驰后调用。
func MarkTrendComplete(trend *Trend, dev *Deviation) {
	if trend == nil || dev == nil {
		return
	}

	// 验证背驰方向与趋势方向一致
	if trend.Type == TrendUp && dev.Direction == DirUp {
		// 上涨趋势 + 顶背驰 → 完成
		trend.IsComplete = true
		trend.CompleteReason = "顶背驰确认完成"
	} else if trend.Type == TrendDown && dev.Direction == DirDown {
		// 下跌趋势 + 底背驰 → 完成
		trend.IsComplete = true
		trend.CompleteReason = "底背驰确认完成"
	} else if trend.Type == RangeOnly && trend.Pivots[0].State == PivotDestroyed {
		// 盘整 + 第三类买卖点 → 完成
		trend.IsComplete = true
		trend.CompleteReason = "第三类买卖点确认盘整完成"
	}
}

// UpdateTrendsWithDeviations 用背驰检测结果更新走势完成状态。
// 在 DetectTrendDeviations 之后调用。
func UpdateTrendsWithDeviations(trends []Trend, trendDeviations []Deviation) {
	for i := range trends {
		for _, dev := range trendDeviations {
			if dev.SegmentAfter != nil {
				// 检查该背驰是否属于此走势（通过时间范围）
				if dev.SegmentAfter.EndIndex >= trends[i].StartIndex &&
					dev.SegmentAfter.EndIndex <= trends[i].EndIndex {
					MarkTrendComplete(&trends[i], &dev)
					break // 找到一个即可确认
				}
			}
		}
	}
}
