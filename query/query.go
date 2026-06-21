package query

import "github.com/bambuo/chan/types"

// ResultQuery 提供对 Result 的便捷查询方法。
type ResultQuery struct {
	Result *types.Result
}

// NewResultQuery 为 Result 创建查询包装。
func NewResultQuery(r *types.Result) *ResultQuery {
	return &ResultQuery{Result: r}
}

// PivotOfSegment 查找包含指定线段的第一个中枢。
func (q *ResultQuery) PivotOfSegment(segIdx int) *types.Pivot {
	if q.Result == nil || segIdx < 0 || segIdx >= len(q.Result.Segments) {
		return nil
	}
	seg := &q.Result.Segments[segIdx]
	for i := range q.Result.Pivots {
		p := &q.Result.Pivots[i]
		if p.StartIndex <= seg.StartIndex && p.EndIndex >= seg.EndIndex {
			return p
		}
	}
	return nil
}

// SegmentsOfPivot 返回中枢包含的线段列表。
func (q *ResultQuery) SegmentsOfPivot(pivotIdx int) []types.Segment {
	if q.Result == nil || pivotIdx < 0 || pivotIdx >= len(q.Result.Pivots) {
		return nil
	}
	return q.Result.Pivots[pivotIdx].Segments
}

// BiOfSegment 返回线段包含的笔列表。
func (q *ResultQuery) BiOfSegment(segIdx int) []types.Bi {
	if q.Result == nil || segIdx < 0 || segIdx >= len(q.Result.Segments) {
		return nil
	}
	bis := make([]types.Bi, 0)
	for _, mb := range q.Result.Segments[segIdx].BiList {
		bis = append(bis, mb.Bi)
	}
	return bis
}

// SignalAt 返回指定索引位置的买卖点信号。
func (q *ResultQuery) SignalAt(index int) *types.Signal {
	if q.Result == nil {
		return nil
	}
	for i := range q.Result.Signals {
		if q.Result.Signals[i].Index == index {
			return &q.Result.Signals[i]
		}
	}
	return nil
}

// LatestPivot 返回最近（最后一个）中枢。
func (q *ResultQuery) LatestPivot() *types.Pivot {
	if q.Result == nil || len(q.Result.Pivots) == 0 {
		return nil
	}
	return &q.Result.Pivots[len(q.Result.Pivots)-1]
}

// LatestBi 返回最近（最后一）笔。
func (q *ResultQuery) LatestBi() *types.Bi {
	if q.Result == nil || len(q.Result.Bis) == 0 {
		return nil
	}
	return &q.Result.Bis[len(q.Result.Bis)-1]
}

// LatestSegment 返回最近（最后一个）线段。
func (q *ResultQuery) LatestSegment() *types.Segment {
	if q.Result == nil || len(q.Result.Segments) == 0 {
		return nil
	}
	return &q.Result.Segments[len(q.Result.Segments)-1]
}

// LatestTrend 返回最近（最后一个）走势。
func (q *ResultQuery) LatestTrend() *types.Trend {
	if q.Result == nil || len(q.Result.Trends) == 0 {
		return nil
	}
	return &q.Result.Trends[len(q.Result.Trends)-1]
}

// BiAtIndex 返回指定索引位置的笔。
func (q *ResultQuery) BiAtIndex(index int) *types.Bi {
	if q.Result == nil {
		return nil
	}
	for i := range q.Result.Bis {
		if q.Result.Bis[i].StartIndex == index || q.Result.Bis[i].EndIndex == index {
			return &q.Result.Bis[i]
		}
	}
	return nil
}

// SegmentAtIndex 返回指定索引位置的线段。
func (q *ResultQuery) SegmentAtIndex(index int) *types.Segment {
	if q.Result == nil {
		return nil
	}
	for i := range q.Result.Segments {
		if q.Result.Segments[i].StartIndex <= index && q.Result.Segments[i].EndIndex >= index {
			return &q.Result.Segments[i]
		}
	}
	return nil
}

// PivotCount 返回中枢总数。
func (q *ResultQuery) PivotCount() int {
	if q.Result == nil {
		return 0
	}
	return len(q.Result.Pivots)
}

// SignalsByType 返回指定类型的买卖点信号列表。
func (q *ResultQuery) SignalsByType(sigType types.SignalType) []types.Signal {
	if q.Result == nil {
		return nil
	}
	result := make([]types.Signal, 0)
	for _, s := range q.Result.Signals {
		if s.Type == sigType {
			result = append(result, s)
		}
	}
	return result
}

// BiPivotCount 返回笔级中枢数量。
func (q *ResultQuery) BiPivotCount() int {
	if q.Result == nil {
		return 0
	}
	cnt := 0
	for _, p := range q.Result.Pivots {
		if p.SourceLevel == "bi" {
			cnt++
		}
	}
	return cnt
}

// LatestBiPivot 返回最新的笔级中枢。
func (q *ResultQuery) LatestBiPivot() *types.Pivot {
	if q.Result == nil {
		return nil
	}
	for i := len(q.Result.Pivots) - 1; i >= 0; i-- {
		if q.Result.Pivots[i].SourceLevel == "bi" {
			return &q.Result.Pivots[i]
		}
	}
	return nil
}

// MultiBiZsCount 返回多笔中枢数量（非单笔中枢）。
func (q *ResultQuery) MultiBiZsCount() int {
	if q.Result == nil {
		return 0
	}
	cnt := 0
	for _, p := range q.Result.Pivots {
		if !p.IsOneBiZs() {
			cnt++
		}
	}
	return cnt
}

// PivotOfBi 查找包含指定笔的中枢。
func (q *ResultQuery) PivotOfBi(biIdx int) *types.Pivot {
	if q.Result == nil || biIdx < 0 || biIdx >= len(q.Result.Bis) {
		return nil
	}
	for i := range q.Result.Pivots {
		p := &q.Result.Pivots[i]
		if p.SourceLevel == "bi" && p.BeginBiIdx <= biIdx && p.EndBiIdx >= biIdx {
			return p
		}
	}
	return nil
}
