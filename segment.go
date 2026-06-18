package chanlun

// ──────────────────────────────────────────────
// §4  线段
// ──────────────────────────────────────────────
//
// 线段由连续三笔重叠构成，代表一个次级别走势类型。
//
// 特征序列是线段划分的核心工具：
//   - 向上线段的特征序列 = 线段内的向下笔序列（第2、4、6...笔）
//   - 向下线段的特征序列 = 线段内的向上笔序列
//
// 线段破坏（文档 §4.6）：
//   - 情况一（无缺口）：特征序列经包含处理后出现反向分型，第一二元素无缺口
//     → 向上线段特征序列出现顶分型；向下线段特征序列出现底分型
//   - 情况二（有缺口）：特征序列分型第一二元素间有缺口，需二次确认

// BuildSegments 从经包含处理后的笔序列构建线段（默认使用 chan 算法）。
// 每个线段至少由 3 笔构成，且内部笔相互重叠。
func BuildSegments(bis []MergedBi) []Segment {
	return BuildSegmentsWithAlgo(bis, "chan")
}

// BuildSegmentsWithAlgo 使用指定算法从笔序列构建线段。
// algo: "chan"（标准缠论）| "dyh"（DYH）| "def"（简化）
func BuildSegmentsWithAlgo(bis []MergedBi, algo string) []Segment {
	builder := NewSegmentBuilder(algo)
	return builder.BuildSegments(bis)
}

// tryBuildSegment 尝试从 start 位置开始构建一个线段。
// 返回线段和下一个待处理笔的索引（线段最后一笔的位置+1）。
func tryBuildSegment(bis []MergedBi, start int) (*Segment, int) {
	if start+2 >= len(bis) {
		return nil, len(bis) // 无法形成线段，跳过所有剩余笔
	}

	// 前三笔确定线段的初始方向和范围
	b0 := bis[start]
	b1 := bis[start+1]
	b2 := bis[start+2]

	// 三笔必须有重叠区域
	overlapHigh := min(min(b0.High, b1.High), b2.High)
	overlapLow := max(max(b0.Low, b1.Low), b2.Low)
	if overlapHigh < overlapLow {
		// 没有重叠，不是线段
		return nil, start + 1
	}

	// 线段的初始方向由第一笔的方向决定
	seg := Segment{
		StartIndex: b0.StartIndex,
		Direction:  b0.Direction,
		BiList:     []MergedBi{b0, b1, b2},
		Top:        max(max(b0.High, b1.High), b2.High),
		Bottom:     min(min(b0.Low, b1.Low), b2.Low),
	}

	// 提取前三笔中的特征序列元素（与线段方向相反的笔）
	// 必须在 extendSegment 之前预填充，否则初始特征元素不会被纳入分型检测
	initFeatures := extractInitialFeatures([]MergedBi{b0, b1, b2}, seg.Direction)
	seg.FeatureSeq = initFeatures

	// 尝试延伸线段，获取延伸后的结果和下一位置
	seg, nextPos := extendSegment(seg, bis, start+3, start)
	return &seg, nextPos
}

// extractInitialFeatures 从初始笔列表中提取特征序列元素。
func extractInitialFeatures(bis []MergedBi, segDir Direction) []FeatureElement {
	features := make([]FeatureElement, 0, 2)
	for _, b := range bis {
		if b.Direction != segDir {
			features = append(features, FeatureElement{
				Bi:       b.Bi,
				StartIdx: b.StartIndex,
				EndIdx:   b.EndIndex,
				High:     b.High,
				Low:      b.Low,
			})
		}
	}
	return features
}

// extendSegment 尝试延伸线段，直到被破坏。
// 返回延伸后的线段和下一个待处理的笔索引。
// startPos: 从 bis 中的哪个位置开始延伸
// segStart: 线段的起始 bi 在 bis 中的索引
//
// 线段破坏分两种情况（文档 §4.5，严格遵循缠论原文）：
//
//	情况一（特征序列无缺口标准破坏）：
//	  特征序列经包含处理后出现反向分型，且该分型的
//	  第一元素与第二元素之间没有价格缺口（有重叠）。
//	  → 线段立即结束，结束点为该分型的极点位置。
//
//	情况二（特征序列有缺口的破坏，需二次确认）：
//	  特征序列出现反向分型，但该分型的第一元素与第二元素
//	  之间存在价格缺口（无重叠）。需等待后续走势形成
//	  新的反向特征序列分型来确认。
//
// 情况二实现说明：
//
//	理论要求（第67课）：从缺口分型的极点开始，形成一个新（试探性）线段，
//	该新线段的特征序列出现分型（任一情况）后，原线段在第一次分型极点处结束。
//
//	当前实现步骤：
//	  1. 特征序列出现分型且有缺口 → 记下 gapFractalIdx（分型中间元素索引）
//	     重置 processedFeatures，进入二次确认模式
//	  2. 二次确认模式中，收集与原线段同向的笔（构成新线段的特征序列）
//	     新线段方向与原线段相反，故其特征序列 = 与原线段同向的笔序列
//	  3. 对新特征序列执行包含处理，检测反向分型：
//	     原线段向上 → 新线段向下 → 新特征序列(向上笔)出现底分型 → 确认
//	     原线段向下 → 新线段向上 → 新特征序列(向下笔)出现顶分型 → 确认
//	  4. 确认后，ConfirmIndex = gapFractalIdx（理论正确的结束位置）
func extendSegment(seg Segment, bis []MergedBi, startPos, segStart int) (Segment, int) {
	pos := startPos
	gapPending := false // Case 2: 特征序列第一、二元素有缺口，等待二次确认
	gapFractalIdx := 0  // 缺口分型中间元素在 K 线空间中的索引（用于 ConfirmIndex）

	// 缓存经包含处理后的特征序列（预填充初始特征元素，避免每次全量重算）
	processedFeatures := make([]FeatureElement, 0, len(seg.FeatureSeq)+16)
	// 对初始特征序列执行包含处理，确保分型检测的输入一致
	for _, f := range seg.FeatureSeq {
		processedFeatures = mergeFeatureToCache(processedFeatures, f)
	}

	for pos < len(bis) {
		curr := bis[pos]

		// 添加到线段
		seg.BiList = append(seg.BiList, curr)
		seg.EndIndex = curr.EndIndex
		if curr.High > seg.Top {
			seg.Top = curr.High
		}
		if curr.Low < seg.Bottom {
			seg.Bottom = curr.Low
		}

		if !gapPending {
			// ── 正常模式：收集反向笔构成原线段的特征序列 ──
			if curr.Direction != seg.Direction {
				newFeat := FeatureElement{
					Bi:       curr.Bi,
					StartIdx: curr.StartIndex,
					EndIdx:   curr.EndIndex,
					High:     curr.High,
					Low:      curr.Low,
				}

				seg.FeatureSeq = append(seg.FeatureSeq, newFeat)
				processedFeatures = mergeFeatureToCache(processedFeatures, newFeat)

				// 检测特征序列分型（在已包含处理的序列上）
				if len(processedFeatures) >= 3 {
					last3 := processedFeatures[len(processedFeatures)-3:]
					if checkFeatureFractal(last3, seg.Direction) {
						// actual_break 检查（对齐 chan.py EigenFX.actual_break）：
						// 验证第三元素是否真正突破了第二元素的极值
						if !checkActualBreak(last3, seg.Direction) {
							// 未实际突破，不确认线段破坏
							continue
						}

						hasGap := hasFeatureGap(last3[0], last3[1])

						if !hasGap {
							// 情况一：无缺口 → 标准破坏，线段立即结束
							seg.IsBroken = true
							seg.BreakType = BreakStd
							seg.ConfirmIndex = curr.EndIndex
							return seg, pos + 1
						}

						// 情况二：有缺口 → 进入二次确认模式
						gapPending = true
						// 记录分型中间元素的结束索引作为缺口极点位置
						gapFractalIdx = last3[1].EndIdx
						// 重置特征序列缓存，开始收集新（试探性）线段的特征序列
						// 新线段方向与原线段相反，其特征序列 = 与原线段同向的笔
						//
						// 重要：需回溯收集 gap 分型中间元素之后已经处理过的同向笔，
						// 例如 gap 在 pos=5 处触发，但 pos=4 的笔是同向的，应纳入新特征序列。
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
						processedFeatures = make([]FeatureElement, 0, len(newFeatures)+8)
						for _, f := range newFeatures {
							processedFeatures = mergeFeatureToCache(processedFeatures, f)
						}
					}
				}
			}
		} else {
			// ── 二次确认模式：收集同向笔构成新线段的特征序列 ──
			// 原线段向上 → 新(试探性)线段向下 → 新特征序列 = 向上笔(同向)
			// 原线段向下 → 新(试探性)线段向上 → 新特征序列 = 向下笔(同向)
			if curr.Direction == seg.Direction {
				newFeat := FeatureElement{
					Bi:       curr.Bi,
					StartIdx: curr.StartIndex,
					EndIdx:   curr.EndIndex,
					High:     curr.High,
					Low:      curr.Low,
				}

				processedFeatures = mergeFeatureToCache(processedFeatures, newFeat)

				// 确定新线段方向对应的特征序列分型类型
				// 新线段与原线段方向相反，故检查相反方向的分型
				var oppositeDir Direction
				if seg.Direction == DirUp {
					oppositeDir = DirDown // 新线段向下 → 新特征序列(向上笔)出现底分型
				} else {
					oppositeDir = DirUp // 新线段向上 → 新特征序列(向下笔)出现顶分型
				}

				if len(processedFeatures) >= 3 {
					last3 := processedFeatures[len(processedFeatures)-3:]
					if checkFeatureFractal(last3, oppositeDir) {
						// 新特征序列分型确认：原线段在第一次分型的极点处结束
						seg.IsBroken = true
						seg.BreakType = BreakStroke
						seg.ConfirmIndex = gapFractalIdx
						return seg, pos + 1
					}
				}
			}
		}

		pos++
	}

	// 直到结束未被破坏
	seg.EndIndex = bis[len(bis)-1].EndIndex
	return seg, len(bis)
}

// buildFeatureSeq 从线段的笔列表中提取特征序列。
// 向上线段：特征序列 = 向下笔（方向相反的笔）
// 向下线段：特征序列 = 向上笔
func buildFeatureSeq(bis []MergedBi, segDir Direction) []FeatureElement {
	features := make([]FeatureElement, 0)

	for _, b := range bis {
		// 特征序列元素是与线段方向相反的笔
		if b.Direction != segDir {
			features = append(features, FeatureElement{
				Bi:       b.Bi,
				StartIdx: b.StartIndex,
				EndIdx:   b.EndIndex,
				High:     b.High,
				Low:      b.Low,
			})
		}
	}

	return features
}

// processFeatureInclusion 对特征序列进行包含处理。
// 处理规则与 K 线包含处理类似。
func processFeatureInclusion(features []FeatureElement) []FeatureElement {
	if len(features) < 2 {
		result := make([]FeatureElement, len(features))
		copy(result, features)
		return result
	}

	result := make([]FeatureElement, 0, len(features))
	result = append(result, features[0])

	for i := 1; i < len(features); i++ {
		curr := features[i]
		last := &result[len(result)-1]

		// 判断包含关系
		contained := false
		if curr.High <= last.High && curr.Low >= last.Low {
			// curr 被 last 包含
			contained = true
			// 提取方向：由特征序列元素间的非包含关系确定
			dir := determineFeatureDirection(result)
			switch dir {
			case DirUp:
				last.High = max(last.High, curr.High)
				last.Low = max(last.Low, curr.Low)
			case DirDown:
				last.High = min(last.High, curr.High)
				last.Low = min(last.Low, curr.Low)
			default:
				// 默认向上处理
				last.High = max(last.High, curr.High)
				last.Low = max(last.Low, curr.Low)
			}
			last.EndIdx = curr.EndIdx
		} else if last.High <= curr.High && last.Low >= curr.Low {
			// last 被 curr 包含
			contained = true
			dir := determineFeatureDirection(result[:len(result)-1])
			switch dir {
			case DirUp:
				last.High = max(curr.High, last.High)
				last.Low = max(curr.Low, last.Low)
			case DirDown:
				last.High = min(curr.High, last.High)
				last.Low = min(curr.Low, last.Low)
			default:
				last.High = max(curr.High, last.High)
				last.Low = max(curr.Low, last.Low)
			}
			last.EndIdx = curr.EndIdx
		}

		if !contained {
			result = append(result, curr)
		}
	}

	return result
}

// determineFeatureDirection 确定特征序列元素的当前方向。
// 从结果序列末尾向前查找最近的非包含关系对。
func determineFeatureDirection(features []FeatureElement) Direction {
	if len(features) < 2 {
		return DirNone
	}

	for i := len(features) - 1; i >= 1; i-- {
		prev := features[i-1]
		curr := features[i]

		// 非包含关系
		if !(curr.High <= prev.High && curr.Low >= prev.Low) &&
			!(prev.High <= curr.High && prev.Low >= curr.Low) {
			if curr.High > prev.High && curr.Low > prev.Low {
				return DirUp
			}
			if curr.High < prev.High && curr.Low < prev.Low {
				return DirDown
			}
		}
	}

	return DirNone
}

// checkFeatureFractal 检查特征序列是否出现反向分型。
//
// 文档 §4.4 精确定义：
//   - 向上线段的特征序列（向下笔序列）出现标准顶分型 → 线段被破坏
//   - 向下线段的特征序列（向上笔序列）出现标准底分型 → 线段被破坏
//
// 特征序列顶分型公式：
//
//	high(E_i) > high(E_{i-1}) && high(E_i) > high(E_{i+1})
//	&& low(E_i) > low(E_{i-1}) && low(E_i) > low(E_{i+1})
//
// 特征序列底分型公式：
//
//	low(E_i) < low(E_{i-1}) && low(E_i) < low(E_{i+1})
//	&& high(E_i) < high(E_{i-1}) && high(E_i) < high(E_{i+1})
func checkFeatureFractal(features []FeatureElement, segDir Direction) bool {
	if len(features) < 3 {
		return false
	}

	// 检查最后三个特征元素是否构成分型
	last3 := features[len(features)-3:]

	if segDir == DirUp {
		// 向上线段：特征序列（向下笔序列）出现顶分型 → 破坏
		// 顶分型：中间元素 High 最高，Low 也最高
		if last3[1].High > last3[0].High && last3[1].High > last3[2].High &&
			last3[1].Low > last3[0].Low && last3[1].Low > last3[2].Low {
			return true
		}
	} else {
		// 向下线段：特征序列（向上笔序列）出现底分型 → 破坏
		// 底分型：中间元素 Low 最低，High 也最低
		if last3[1].Low < last3[0].Low && last3[1].Low < last3[2].Low &&
			last3[1].High < last3[0].High && last3[1].High < last3[2].High {
			return true
		}
	}

	return false
}

// hasFeatureGap 检查两个特征序列元素之间是否存在缺口。
// 缺口定义：两个相邻特征序列元素的价格范围没有重叠。
//   - 无重叠（有缺口）：first.High < second.Low 或 second.High < first.Low
//   - 有重叠（无缺口）：价格范围有交集
func hasFeatureGap(first, second FeatureElement) bool {
	// 存在缺口 = 无价格重叠
	return first.High < second.Low || second.High < first.Low
}

// checkActualBreak 检查特征序列分型是否实际突破（对齐 chan.py EigenFX.actual_break）。
//
// 向上线段的特征序列顶分型：第三元素的 low 应 < 第二元素的 low（实际向下突破）
// 向下线段的特征序列底分型：第三元素的 high 应 > 第二元素的 high（实际向上突破）
//
// 如果第三元素未实际突破第二元素的极值，则可能是假分型，不应确认线段破坏。
func checkActualBreak(features []FeatureElement, segDir Direction) bool {
	if len(features) < 3 {
		return false
	}
	second := features[1]
	third := features[2]

	if segDir == DirUp {
		// 向上线段：特征序列出现顶分型，第三元素应向下突破第二元素的 low
		return third.Low < second.Low
	}
	// 向下线段：特征序列出现底分型，第三元素应向上突破第二元素的 high
	return third.High > second.High
}

// isStrokeBreakOnFeatures 检查是否发生笔破坏（文档 §4.5 情况二的先兆）。
// 使用缓存的已处理特征序列（不包含当前新笔），确保比较的是「最后一个端点」。
// 笔破坏：反向一笔直接穿透当前线段的最后一个端点。
//
//	向上线段：向下笔跌破最后一个底 = 最近一个已处理特征元素的低点
//	向下线段：向上笔升破最后一个顶 = 最近一个已处理特征元素的高点
//
// 若尚未形成任何特征元素，则回退到线段整体的 Bottom/Top。
func isStrokeBreakOnFeatures(bi MergedBi, segDir Direction, processedFeatures []FeatureElement) bool {
	if len(processedFeatures) == 0 {
		// 无特征元素，无法判断笔破坏
		return false
	}

	// 取最后一个已处理的特征元素作为"最后一个端点"
	lastFeature := processedFeatures[len(processedFeatures)-1]

	if segDir == DirUp {
		// 向上线段：新的向下笔跌破最后一个特征元素的低点
		return bi.Direction == DirDown && bi.Low < lastFeature.Low
	}
	// 向下线段：新的向上笔升破最后一个特征元素的高点
	return bi.Direction == DirUp && bi.High > lastFeature.High
}

// mergeFeatureToCache 将新特征元素增量合并到已处理的缓存中。
// 规则同 K 线包含处理：
//   - 若与最后一个元素有包含关系，按方向合并
//   - 若无包含关系，直接追加
//
// 避免每次全量重算整个特征序列。
func mergeFeatureToCache(cache []FeatureElement, newFeat FeatureElement) []FeatureElement {
	if len(cache) == 0 {
		return append(cache, newFeat)
	}

	last := &cache[len(cache)-1]

	// 检查包含关系
	if newFeat.High <= last.High && newFeat.Low >= last.Low {
		// newFeat 被 last 包含，合并入 last
		dir := determineFeatureDirection(cache)
		switch dir {
		case DirUp:
			last.High = max(last.High, newFeat.High)
			last.Low = max(last.Low, newFeat.Low)
		case DirDown:
			last.High = min(last.High, newFeat.High)
			last.Low = min(last.Low, newFeat.Low)
		default:
			last.High = max(last.High, newFeat.High)
			last.Low = max(last.Low, newFeat.Low)
		}
		last.EndIdx = newFeat.EndIdx
		return cache
	}

	if last.High <= newFeat.High && last.Low >= newFeat.Low {
		// last 被 newFeat 包含，替换 last
		dir := determineFeatureDirection(cache[:len(cache)-1])
		switch dir {
		case DirUp:
			last.High = max(newFeat.High, last.High)
			last.Low = max(newFeat.Low, last.Low)
		case DirDown:
			last.High = min(newFeat.High, last.High)
			last.Low = min(newFeat.Low, last.Low)
		default:
			last.High = max(newFeat.High, last.High)
			last.Low = max(newFeat.Low, last.Low)
		}
		last.EndIdx = newFeat.EndIdx
		return cache
	}

	// 无包含关系，直接追加
	return append(cache, newFeat)
}
