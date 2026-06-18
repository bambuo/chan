package chanlun

// ──────────────────────────────────────────────
// Result 跨层查询 API
// ──────────────────────────────────────────────
//
// 提供从 Result 中进行跨层查询的方法，替代 chan.py 的双向指针引用。
// 所有查询基于索引匹配，时间复杂度为 O(n)（n 为对应切片长度）。
//
// 使用示例：
//
//	result := engine.Snapshot()
//	pivot := result.PivotOfSegment(5)       // 第 5 段属于哪个中枢？
//	bis := result.BiOfSegment(3)            // 第 3 段包含哪些笔？
//	signals := result.SignalAt(42)          // K 线 42 处有买卖点吗？

// PivotOfSegment 查找指定线段（按 StartIndex 匹配）所属的中枢。
// 返回包含该线段的中枢，若不存在则返回 nil。
func (r *Result) PivotOfSegment(segStartIndex int) *Pivot {
	if r == nil {
		return nil
	}
	for i := range r.Pivots {
		p := &r.Pivots[i]
		for _, seg := range p.Segments {
			if seg.StartIndex == segStartIndex {
				return p
			}
		}
	}
	return nil
}

// SegmentsOfPivot 获取指定中枢（按索引）包含的所有线段。
// pivotIdx 是 Pivot 在 Result.Pivots 切片中的索引。
func (r *Result) SegmentsOfPivot(pivotIdx int) []Segment {
	if r == nil || pivotIdx < 0 || pivotIdx >= len(r.Pivots) {
		return nil
	}
	return r.Pivots[pivotIdx].Segments
}

// BiOfSegment 获取指定线段（按 StartIndex 匹配）包含的所有笔。
func (r *Result) BiOfSegment(segStartIndex int) []Bi {
	if r == nil {
		return nil
	}
	// 找到目标线段
	var targetSeg *Segment
	for i := range r.Segments {
		if r.Segments[i].StartIndex == segStartIndex {
			targetSeg = &r.Segments[i]
			break
		}
	}
	if targetSeg == nil {
		return nil
	}

	// 从线段的 BiList 提取
	result := make([]Bi, 0, len(targetSeg.BiList))
	for _, mb := range targetSeg.BiList {
		result = append(result, mb.Bi)
	}
	return result
}

// SignalAt 查找指定 K 线索引位置的所有买卖点信号。
func (r *Result) SignalAt(index int) []Signal {
	if r == nil {
		return nil
	}
	result := make([]Signal, 0)
	for _, s := range r.Signals {
		if s.Index == index {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// TrendAt 查找指定 K 线索引位置所属的走势类型。
// 返回包含该索引的走势，若不存在则返回 nil。
func (r *Result) TrendAt(index int) *Trend {
	if r == nil {
		return nil
	}
	for i := range r.Trends {
		t := &r.Trends[i]
		if index >= t.StartIndex && index <= t.EndIndex {
			return t
		}
	}
	return nil
}

// LatestPivot 获取最新（最后一个）中枢。
func (r *Result) LatestPivot() *Pivot {
	if r == nil || len(r.Pivots) == 0 {
		return nil
	}
	return &r.Pivots[len(r.Pivots)-1]
}

// LatestBi 获取最新（最后一个）笔。
func (r *Result) LatestBi() *Bi {
	if r == nil || len(r.Bis) == 0 {
		return nil
	}
	return &r.Bis[len(r.Bis)-1]
}

// LatestSegment 获取最新（最后一个）线段。
func (r *Result) LatestSegment() *Segment {
	if r == nil || len(r.Segments) == 0 {
		return nil
	}
	return &r.Segments[len(r.Segments)-1]
}

// LatestTrend 获取最新（最后一个）走势类型。
func (r *Result) LatestTrend() *Trend {
	if r == nil || len(r.Trends) == 0 {
		return nil
	}
	return &r.Trends[len(r.Trends)-1]
}

// BiAtIndex 查找指定 K 线索引位置所属的笔。
// 返回包含该索引的笔，若不存在则返回 nil。
func (r *Result) BiAtIndex(index int) *Bi {
	if r == nil {
		return nil
	}
	for i := range r.Bis {
		b := &r.Bis[i]
		if index >= b.StartIndex && index <= b.EndIndex {
			return b
		}
	}
	return nil
}

// SegmentAtIndex 查找指定 K 线索引位置所属的线段。
// 返回包含该索引的线段，若不存在则返回 nil。
func (r *Result) SegmentAtIndex(index int) *Segment {
	if r == nil {
		return nil
	}
	for i := range r.Segments {
		s := &r.Segments[i]
		if index >= s.StartIndex && index <= s.EndIndex {
			return s
		}
	}
	return nil
}

// PivotCount 返回中枢总数。
func (r *Result) PivotCount() int {
	if r == nil {
		return 0
	}
	return len(r.Pivots)
}

// SignalsByType 按类型筛选买卖点信号。
func (r *Result) SignalsByType(sigType SignalType) []Signal {
	if r == nil {
		return nil
	}
	result := make([]Signal, 0)
	for _, s := range r.Signals {
		if s.Type == sigType {
			result = append(result, s)
		}
	}
	return result
}
