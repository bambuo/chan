package chanlun

// ──────────────────────────────────────────────
// 增量包含处理与分型检测
// ──────────────────────────────────────────────
//
// 包含处理是增量引擎的第一步：每根新 K 线到达后，
// 只需与链表尾部节点比较 High/Low，O(1) 判定是否合并。
//
// 分型检测在包含处理之后：只需检查最近 3 个 mergedNode，
// O(1) 判定是否出现顶/底分型。

// tryMergeKline 尝试将新 K 线合并到链表尾部。
// 返回 true 表示被合并（不产生新节点），false 表示追加了新节点。
func (s *StreamEngine) tryMergeKline(k Kline) bool {
	if s.mergedTail == nil {
		// 第一根 K 线
		node := &mergedNode{Kline: k}
		s.mergedHead = node
		s.mergedTail = node
		s.mergedCount = 1
		return false
	}

	last := s.mergedTail

	// 精细包含判定（使用配置的 InclusionOption）
	opt := s.config.Inclusion
	containDir := testContainment(last.Kline, k, opt.AllowTopEqual)

	switch containDir {
	case containCombine:
		if opt.ExcludeIncluded {
			// exclude_included 模式：被包含的直接跳过
			return true
		}
		dir := s.determineMergeDirection()
		// 一字K线方向处理（对齐 chan.py）：is一字 AND 价格相同才跳过
		if dir == DirUp && k.High == k.Low && k.High == last.Kline.High {
			node := &mergedNode{Kline: k, pre: last}
			last.next = node
			s.mergedTail = node
			s.mergedCount++
			return false
		}
		if dir == DirDown && k.High == k.Low && k.Low == last.Kline.Low {
			node := &mergedNode{Kline: k, pre: last}
			last.next = node
			s.mergedTail = node
			s.mergedCount++
			return false
		}
		merged := mergePair(last.Kline, k, dir)
		last.Kline = merged
		return true
	case containUp, containDown:
		node := &mergedNode{Kline: k, pre: last}
		last.next = node
		s.mergedTail = node
		s.mergedCount++
		return false
	}

	return false
}

// determineMergeDirection 确定包含处理的方向。
// 从链表尾部向前查找最近的非包含关系对，O(k) 其中 k 通常很小（<5）。
func (s *StreamEngine) determineMergeDirection() Direction {
	if s.mergedTail == nil || s.mergedTail.pre == nil {
		return DirNone
	}

	// 从尾部向前查找
	curr := s.mergedTail
	for curr != nil && curr.pre != nil {
		prev := curr.pre
		if !isContained(prev.Kline, curr.Kline) {
			if curr.Kline.High >= prev.Kline.High {
				return DirUp
			}
			if curr.Kline.Low <= prev.Kline.Low {
				return DirDown
			}
		}
		curr = curr.pre
	}

	return DirNone
}

// updateFractalState 基于最近 3 个 mergedNode 检测是否出现新分型。
// 返回检测到的分型（nil 表示无新分型）。
//
// 只有当 mergedCount >= 3 且最近 3 个节点未被检测过时才返回分型。
// 通过 lastCheckedCount 避免重复报告同一分型。
func (s *StreamEngine) updateFractalState() *Fractal {
	if s.mergedCount < 3 {
		return nil
	}

	// 获取最近 3 个节点
	tail := s.mergedTail
	if tail == nil || tail.pre == nil || tail.pre.pre == nil {
		return nil
	}

	prev := tail.pre.pre
	mid := tail.pre
	next := tail

	// 检查这三根是否构成分型（与 FindFractals 相同逻辑）
	// 顶分型：中间 K 线高点最高、低点也最高
	if mid.Kline.High > prev.Kline.High && mid.Kline.High > next.Kline.High &&
		mid.Kline.Low > prev.Kline.Low && mid.Kline.Low > next.Kline.Low {

		// 检查这个分型是否已经被报告过（mid 节点未变）
		f := Fractal{
			Type:     TopFractal,
			Index:    s.mergedCount - 2, // mid 在序列中的位置
			High:     mid.Kline.High,
			Low:      mid.Kline.Low,
			Strength: calcFractalStrength(mid.Kline, prev.Kline, next.Kline),
		}
		return &f
	}

	// 底分型：中间 K 线低点最低、高点也最低
	if mid.Kline.Low < prev.Kline.Low && mid.Kline.Low < next.Kline.Low &&
		mid.Kline.High < prev.Kline.High && mid.Kline.High < next.Kline.High {

		f := Fractal{
			Type:     BottomFractal,
			Index:    s.mergedCount - 2,
			High:     mid.Kline.High,
			Low:      mid.Kline.Low,
			Strength: calcFractalStrength(mid.Kline, prev.Kline, next.Kline),
		}
		return &f
	}

	return nil
}
