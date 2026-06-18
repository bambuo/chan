package chanlun

// ──────────────────────────────────────────────
// §3  笔（Bi）
// ──────────────────────────────────────────────
//
// 笔是顶分型与底分型之间的连线，代表一个最小级别的趋势方向。
//
// 笔的成立条件：
//   1. 相邻顶底分型之间至少包含 1 根独立 K 线（不含分型本身的 K 线）
//   2. 分型之间不能共用 K 线
//   3. 笔的方向交替出现
//
// 笔的包含处理（§3.5）：
//   在构筑线段前，需对笔序列进行包含处理。
//   - 向上的笔序列中：取高高（max(H)）、高低（max(L)）
//   - 向下的笔序列中：取低高（min(H)）、低低（min(L)）

// BuildBis 从分型列表构建笔序列。
// 分型列表必须已经过同向取极值和间隔过滤处理。
// 返回的笔序列方向严格交替，且满足最少独立 K 线数要求。
func BuildBis(klines []Kline, fractals []Fractal, minKLineCount int, minPriceRatio float64) []Bi {
	if len(fractals) < 2 {
		return nil
	}

	bis := make([]Bi, 0, len(fractals)/2)

	for i := 0; i < len(fractals)-1; i++ {
		start := fractals[i]
		end := fractals[i+1]

		// 分型必须交替出现
		if start.Type == end.Type {
			continue
		}

		// 检查方向：底分型在前→向上笔，顶分型在前→向下笔
		var dir Direction
		if start.Type == BottomFractal && end.Type == TopFractal {
			dir = DirUp
		} else if start.Type == TopFractal && end.Type == BottomFractal {
			dir = DirDown
		} else {
			continue
		}

		// 检查独立 K 线数
		// 文档 §3.2：分型之间至少间隔 1 根独立 K 线
		independentKCount := independentKCountBetweenFractals(start, end)
		if independentKCount < minKLineCount {
			continue
		}

		// 计算笔的统计信息
		bi := buildBiFromRange(klines, start, end, dir)

		// 新笔标准（严笔）：检查价格变动幅度
		if minKLineCount >= 5 && minPriceRatio > 0 {
			priceDiff := bi.Length
			avgPrice := (bi.StartPrice + bi.EndPrice) / 2
			if avgPrice > 0 && priceDiff/avgPrice < minPriceRatio {
				continue
			}
		}

		if bi.KLineCount > 0 {
			bis = append(bis, bi)
		}
	}

	return bis
}

// buildBiFromRange 在分型范围内构建一笔。
func buildBiFromRange(klines []Kline, start, end Fractal, dir Direction) Bi {
	// 确定笔的价格范围
	startPrice := start.Low
	if start.Type == TopFractal {
		startPrice = start.High
	}
	endPrice := end.Low
	if end.Type == TopFractal {
		endPrice = end.High
	}

	// 扫描笔范围内的最高/最低价
	high := startPrice
	low := startPrice
	for i := start.Index; i <= end.Index; i++ {
		if i >= 0 && i < len(klines) {
			if klines[i].High > high {
				high = klines[i].High
			}
			if klines[i].Low < low {
				low = klines[i].Low
			}
		}
	}

	length := endPrice - startPrice
	if length < 0 {
		length = -length
	}

	kCount := end.Index - start.Index + 1
	slope := 0.0
	if kCount > 0 {
		slope = length / float64(kCount)
	}

	return Bi{
		StartIndex: start.Index,
		EndIndex:   end.Index,
		Direction:  dir,
		StartPrice: startPrice,
		EndPrice:   endPrice,
		High:       high,
		Low:        low,
		Length:     length,
		Slope:      slope,
		KLineCount: kCount,
	}
}

// ──────────────────────────────────────────────
// §3.5  特征序列元素的包含处理（笔的包含处理）
// ──────────────────────────────────────────────
//
// 注意：此处的"笔的包含处理"作用对象是线段特征序列元素，而非常规笔序列。
//
// 缠论原文规定标准笔序列由顶底分型交替构成、方向严格交替出现，
// 同向笔不应在常规笔序列中连续存在。
//
// 特征序列包含处理的正确位置在线段构建内部：
//   - 向上线段取其向下笔序列作为特征序列
//   - 向下线段取其向上笔序列作为特征序列
//
// 本函数是对特征序列元素在进入 BuildSegments 前的预处理（可选），
// 与 segment.go 中的 mergeFeatureToCache 协作完成特征序列的包含处理。
// 开启 EnableBiInclusion 时先在此合并特征序列元素中的包含关系，
// 再由 mergeFeatureToCache 在延伸过程中进行增量包含处理。
//
// 笔包含的定义：
//
//	笔 A 包含笔 B：笔 A 的高点 > 笔 B 的高点，且笔 A 的低点 < 笔 B 的低点
//
// 处理规则（与 K 线包含处理逻辑一致）：
//
//	方向由最近非包含笔对的关系确定
//	- 向上序列中：取高高（max(H)）、高低（max(L)）
//	- 向下序列中：取低高（min(H)）、低低（min(L)）
func MergeBis(bis []Bi) []MergedBi {
	if len(bis) < 2 {
		result := make([]MergedBi, len(bis))
		for i, b := range bis {
			result[i] = MergedBi{Bi: b, OriginalCount: 1}
		}
		return result
	}

	result := make([]MergedBi, 0, len(bis))
	result = append(result, MergedBi{Bi: bis[0], OriginalCount: 1, MergedFrom: []int{0}})

	biIdx := 1
	for biIdx < len(bis) {
		last := &result[len(result)-1]
		curr := bis[biIdx]

		// 不同方向：不能合并，直接追加
		if last.Direction != curr.Direction {
			result = append(result, MergedBi{
				Bi:            curr,
				OriginalCount: 1,
				MergedFrom:    []int{biIdx},
			})
			biIdx++
			continue
		}

		// 同方向：检查包含关系
		if isBiContained(curr, last.Bi) {
			// 当前笔包含（或等于）最后一笔，需要合并
			// 方向由最近非包含笔对关系确定（而非笔自身方向）
			dir := determineBiDirection(result)
			merged := mergeBiPair(last.Bi, curr, dir)
			last.Bi = merged
			last.OriginalCount++
			last.MergedFrom = append(last.MergedFrom, biIdx)
			biIdx++
			continue
		}

		// 检查反向包含：最后一笔是否被当前笔包含
		if isBiContained(last.Bi, curr) {
			// 最后一笔包含（或等于）当前笔，替换它
			dir := determineBiDirection(result[:len(result)-1])
			merged := mergeBiPair(curr, last.Bi, dir)
			result[len(result)-1] = MergedBi{
				Bi:            merged,
				OriginalCount: last.OriginalCount + 1,
				MergedFrom:    append(last.MergedFrom, biIdx),
			}
			biIdx++
			continue
		}

		// 无包含关系
		result = append(result, MergedBi{
			Bi:            curr,
			OriginalCount: 1,
			MergedFrom:    []int{biIdx},
		})
		biIdx++
	}

	return result
}

// isBiContained 判断笔 a 是否包含笔 b。
//
//	文档 §3.5: 特征元素 A 包含特征元素 B :=
//	  high(A) >= high(B) && low(A) <= low(B)
//
// 参数 a 为外层笔，b 为被包含的笔。
func isBiContained(a, b Bi) bool {
	return a.High >= b.High && a.Low <= b.Low
}

// mergeBiPair 合并两支同向笔。
// dir 由非包含笔对的方向确定（而非笔自身方向）。
//   - DirUp:   向上处理 → 取高高（max(H)）、高低（max(L)）
//   - DirDown: 向下处理 → 取低高（min(H)）、低低（min(L)）
func mergeBiPair(a, b Bi, dir Direction) Bi {
	merged := a
	merged.EndIndex = maxInt(a.EndIndex, b.EndIndex)
	merged.EndPrice = b.EndPrice

	// 重新计算 K 线数量为覆盖范围
	merged.KLineCount = merged.EndIndex - merged.StartIndex + 1

	switch dir {
	case DirUp:
		merged.High = max(a.High, b.High)
		merged.Low = max(a.Low, b.Low)
	case DirDown:
		merged.High = min(a.High, b.High)
		merged.Low = min(a.Low, b.Low)
	default:
		// 无法确定方向时，以笔自身方向为准（保守回退）
		if a.Direction == DirUp {
			merged.High = max(a.High, b.High)
			merged.Low = max(a.Low, b.Low)
		} else {
			merged.High = min(a.High, b.High)
			merged.Low = min(a.Low, b.Low)
		}
	}

	// 重新计算长度和斜率
	merged.Length = merged.EndPrice - merged.StartPrice
	if merged.Length < 0 {
		merged.Length = -merged.Length
	}
	if merged.KLineCount > 0 {
		merged.Slope = merged.Length / float64(merged.KLineCount)
	}

	return merged
}

// determineBiDirection 确定笔序列的当前处理方向。
// 从结果序列末尾向前查找最近的非包含笔对。
// 返回 1=向上, -1=向下, 0=无法确定。
//
// 规则（与 K 线包含处理的 determineDirection 一致）：
//
//	向上：当前笔 High > 前一笔 High 且 Low > 前一笔 Low
//	向下：当前笔 High < 前一笔 High 且 Low < 前一笔 Low
func determineBiDirection(bis []MergedBi) Direction {
	if len(bis) < 2 {
		return DirNone
	}

	// 从末尾向前查找最近的非包含关系对
	for i := len(bis) - 1; i >= 1; i-- {
		prev := bis[i-1].Bi
		curr := bis[i].Bi

		if !isBiContained(prev, curr) && !isBiContained(curr, prev) {
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
