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

	// 判断走势是否可能已完成
	// 上涨趋势：最后一个中枢后的离开段出现顶背驰 → IsComplete=true
	// 下跌趋势：最后一个中枢后的离开段出现底背驰
	// 此处仅做标记，实际背驰检测在 deviation.go 中完成
	if len(trend.Pivots) >= 1 {
		lastPivot := trend.Pivots[len(trend.Pivots)-1]
		if lastPivot.State == PivotDestroyed {
			trend.IsComplete = true
			trend.CompleteReason = "中枢被第三类买卖点破坏"
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
