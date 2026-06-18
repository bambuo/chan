package chanlun

// ──────────────────────────────────────────────
// 增量笔/线段管理
// ──────────────────────────────────────────────
//
// 包含虚拟笔端机制（借鉴 chan.py 的 update_virtual_end）：
//   - 当最新分型可能形成新笔但尚未确认时，临时延伸最后一笔的端点
//   - 当确认失败时恢复到原始端点
//
// 线段增量更新：
//   - 每新增一笔，检查是否可以延伸当前线段
//   - 特征序列使用增量包含处理（mergeFeatureToCache）
//   - 检测特征序列分型以判断线段是否被破坏

// updateVirtualBiEnd 临时延伸最后一笔的端点到新分型位置。
// 借鉴 chan.py 的 update_virtual_end：在分型尚未确认成笔时，
// 临时将最后一笔的端点移到更极端的位置。
func (s *StreamEngine) updateVirtualBiEnd(f *Fractal, inc *IncrementalResult) {
	if s.biTail == nil {
		return
	}

	// 保存原始端点（仅首次虚拟时保存）
	if !s.hasVirtualBi {
		biCopy := s.biTail.Bi
		s.virtualBiSavedEnd = &biCopy
		s.hasVirtualBi = true
	}

	// 延伸笔的端点到新分型位置
	bi := &s.biTail.Bi
	if f.Type == TopFractal && bi.Direction == DirUp {
		bi.EndIndex = f.Index
		bi.EndPrice = f.High
		bi.High = f.High
	} else if f.Type == BottomFractal && bi.Direction == DirDown {
		bi.EndIndex = f.Index
		bi.EndPrice = f.Low
		bi.Low = f.Low
	}
	bi.Length = abs(bi.EndPrice - bi.StartPrice)
	bi.KLineCount = bi.EndIndex - bi.StartIndex + 1
	if bi.KLineCount > 0 {
		bi.Slope = bi.Length / float64(bi.KLineCount)
	}
	s.biTail.virtual = true

	updatedBi := s.biTail.Bi
	inc.UpdatedBiEnd = &updatedBi
}

// restoreVirtualBiEnd 恢复虚拟笔端到保存的原始端点。
func (s *StreamEngine) restoreVirtualBiEnd() {
	if s.biTail == nil || !s.hasVirtualBi || s.virtualBiSavedEnd == nil {
		return
	}

	// 恢复原始端点
	s.biTail.Bi = *s.virtualBiSavedEnd
	s.biTail.virtual = false
	s.hasVirtualBi = false
	s.virtualBiSavedEnd = nil
}

// processNewBi 处理新增笔，尝试延伸或结束当前线段。
func (s *StreamEngine) processNewBi(newBi *biNode, inc *IncrementalResult) {
	if s.segTail == nil {
		// 尝试初始化第一个线段（需要至少 3 笔有重叠）
		s.tryInitFirstSegment(newBi, inc)
		return
	}

	// 尝试延伸当前线段
	s.tryExtendSegment(newBi, inc)
}

// tryInitFirstSegment 尝试用最近的笔初始化第一个线段。
func (s *StreamEngine) tryInitFirstSegment(latestBi *biNode, inc *IncrementalResult) {
	// 收集最近的笔，至少需要 3 笔
	bis := s.collectLastBis(3)
	if len(bis) < 3 {
		return
	}

	b0, b1, b2 := bis[0], bis[1], bis[2]

	// 三笔必须有重叠
	overlapHigh := min(min(b0.High, b1.High), b2.High)
	overlapLow := max(max(b0.Low, b1.Low), b2.Low)
	if overlapHigh < overlapLow {
		return
	}

	seg := Segment{
		StartIndex: b0.StartIndex,
		EndIndex:   b2.EndIndex,
		Direction:  b0.Direction,
		BiList:     []MergedBi{biToMerged(b0), biToMerged(b1), biToMerged(b2)},
		Top:        max(max(b0.High, b1.High), b2.High),
		Bottom:     min(min(b0.Low, b1.Low), b2.Low),
	}

	// 提取初始特征序列
	initFeatures := extractInitialFeatures(seg.BiList, seg.Direction)
	seg.FeatureSeq = initFeatures

	node := &segNode{
		Segment:      seg,
		featureCache: initFeatures,
	}
	s.segHead = node
	s.segTail = node
	s.segCount = 1

	segCopy := seg
	inc.NewSegment = &segCopy
}

// tryExtendSegment 尝试用新笔延伸当前线段，或结束当前线段。
func (s *StreamEngine) tryExtendSegment(newBi *biNode, inc *IncrementalResult) {
	seg := &s.segTail.Segment
	mb := toMergedBi(newBi)

	// 添加到线段
	seg.BiList = append(seg.BiList, mb)
	seg.EndIndex = newBi.Bi.EndIndex
	if newBi.Bi.High > seg.Top {
		seg.Top = newBi.Bi.High
	}
	if newBi.Bi.Low < seg.Bottom {
		seg.Bottom = newBi.Bi.Low
	}

	if !s.segTail.gapPending {
		// ── 正常模式：收集反向笔构成特征序列 ──
		if newBi.Bi.Direction != seg.Direction {
			newFeat := FeatureElement{
				Bi:       newBi.Bi,
				StartIdx: newBi.Bi.StartIndex,
				EndIdx:   newBi.Bi.EndIndex,
				High:     newBi.Bi.High,
				Low:      newBi.Bi.Low,
			}

			seg.FeatureSeq = append(seg.FeatureSeq, newFeat)
			s.segTail.featureCache = mergeFeatureToCache(s.segTail.featureCache, newFeat)

			// 检测特征序列分型
			if len(s.segTail.featureCache) >= 3 {
				last3 := s.segTail.featureCache[len(s.segTail.featureCache)-3:]
				if checkFeatureFractal(last3, seg.Direction) {
					hasGap := hasFeatureGap(last3[0], last3[1])

					if !hasGap {
						// 情况一：无缺口 → 标准破坏
						seg.IsBroken = true
						seg.BreakType = BreakStd
						seg.ConfirmIndex = newBi.Bi.EndIndex
						segCopy := *seg
						inc.NewSegment = &segCopy

						// 开始新线段准备
						s.prepareNewSegment(newBi)
						return
					}

					// 情况二：有缺口 → 进入二次确认
					s.segTail.gapPending = true
					s.segTail.gapFractalIdx = last3[1].EndIdx

					// 重置特征序列缓存
					newFeatures := make([]FeatureElement, 0)
					for _, b := range seg.BiList {
						if b.Direction == seg.Direction && b.StartIndex > last3[1].StartIdx {
							newFeatures = append(newFeatures, FeatureElement{
								Bi:       b.Bi,
								StartIdx: b.StartIndex,
								EndIdx:   b.EndIndex,
								High:     b.High,
								Low:      b.Low,
							})
						}
					}
					s.segTail.featureCache = make([]FeatureElement, 0, len(newFeatures)+8)
					for _, f := range newFeatures {
						s.segTail.featureCache = mergeFeatureToCache(s.segTail.featureCache, f)
					}
				}
			}
		}
	} else {
		// ── 二次确认模式：收集同向笔 ──
		if newBi.Bi.Direction == seg.Direction {
			newFeat := FeatureElement{
				Bi:       newBi.Bi,
				StartIdx: newBi.Bi.StartIndex,
				EndIdx:   newBi.Bi.EndIndex,
				High:     newBi.Bi.High,
				Low:      newBi.Bi.Low,
			}

			s.segTail.featureCache = mergeFeatureToCache(s.segTail.featureCache, newFeat)

			var oppositeDir Direction
			if seg.Direction == DirUp {
				oppositeDir = DirDown
			} else {
				oppositeDir = DirUp
			}

			if len(s.segTail.featureCache) >= 3 {
				last3 := s.segTail.featureCache[len(s.segTail.featureCache)-3:]
				if checkFeatureFractal(last3, oppositeDir) {
					seg.IsBroken = true
					seg.BreakType = BreakStroke
					seg.ConfirmIndex = s.segTail.gapFractalIdx
					segCopy := *seg
					inc.NewSegment = &segCopy

					s.prepareNewSegment(newBi)
					return
				}
			}
		}
	}
}

// prepareNewSegment 在线段结束后准备新线段的起始笔。
func (s *StreamEngine) prepareNewSegment(lastBi *biNode) {
	// 新线段从最后几笔开始尝试
	node := &segNode{
		Segment: Segment{},
		pre:     s.segTail,
	}
	s.segTail.next = node
	s.segTail = node
	s.segCount++
}

// ──────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────

// collectLastBis 从链表尾部收集最近 N 个笔节点。
func (s *StreamEngine) collectLastBis(n int) []Bi {
	if s.biTail == nil || n <= 0 {
		return nil
	}

	// 先收集到临时 slice（从尾到头）
	tmp := make([]*biNode, 0, n)
	curr := s.biTail
	for len(tmp) < n && curr != nil {
		tmp = append(tmp, curr)
		curr = curr.pre
	}

	// 反转为从头到尾
	result := make([]Bi, len(tmp))
	for i, node := range tmp {
		result[len(tmp)-1-i] = node.Bi
	}
	return result
}

// biToMerged 将 Bi 值类型转换为 MergedBi。
func biToMerged(b Bi) MergedBi {
	return MergedBi{Bi: b, OriginalCount: 1}
}

// toMergedBi 将笔节点转换为 MergedBi。
func toMergedBi(node *biNode) MergedBi {
	return MergedBi{
		Bi:            node.Bi,
		OriginalCount: 1,
	}
}
