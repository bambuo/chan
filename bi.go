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
// 分型列表必须已经过同向取极值处理（由 FindFractals 保证）。
// 返回的笔序列方向严格交替，且满足最少独立 K 线数要求。
func BuildBis(candles []Candle, fractals []Fractal, minKLineCount int, minPriceRatio float64) []Bi {
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
		independentKCount := end.Index - start.Index - 3
		if independentKCount < minKLineCount {
			continue
		}

		// 计算笔的统计信息
		bi := buildBiFromRange(candles, start, end, dir)

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
func buildBiFromRange(candles []Candle, start, end Fractal, dir Direction) Bi {
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
		if i >= 0 && i < len(candles) {
			if candles[i].High > high {
				high = candles[i].High
			}
			if candles[i].Low < low {
				low = candles[i].Low
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
// §3.5  笔的包含处理
// ──────────────────────────────────────────────

// MergeBis 对笔序列进行包含处理。
// 连续同方向的笔之间可能存在包含关系，需要合并。
//
// 笔包含的定义：
//   笔 A 包含笔 B：笔 A 的高点 > 笔 B 的高点，且笔 A 的低点 < 笔 B 的低点
//
// 处理规则：
//   - 向上笔序列中：取高高（max(H)）、高低（max(L)）
//   - 向下笔序列中：取低高（min(H)）、低低（min(L)）
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
			// 当前笔被最后一笔包含，需要合并
			merged := mergeBiPair(last.Bi, curr)
			last.Bi = merged
			last.OriginalCount++
			last.MergedFrom = append(last.MergedFrom, biIdx)
			biIdx++
			continue
		}

		// 检查反向包含：最后一笔是否被当前笔包含
		if isBiContained(last.Bi, curr) {
			// 最后一笔被当前笔包含，替换它
			merged := mergeBiPair(curr, last.Bi)
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
// 笔 A 包含笔 B: A.High > B.High 且 A.Low < B.Low
func isBiContained(a, b Bi) bool {
	return a.High > b.High && a.Low < b.Low
}

// mergeBiPair 合并两支同向笔。
// 向上笔：取高高（max(H)）、高低（max(L)）
// 向下笔：取低高（min(H)）、低低（min(L)）
func mergeBiPair(a, b Bi) Bi {
	merged := a
	merged.EndIndex = maxInt(a.EndIndex, b.EndIndex)
	merged.EndPrice = b.EndPrice

	if merged.KLineCount < b.KLineCount {
		merged.KLineCount = b.KLineCount
	}
	// 重新计算 K 线数量为覆盖范围
	merged.KLineCount = merged.EndIndex - merged.StartIndex + 1

	switch a.Direction {
	case DirUp:
		merged.High = max(a.High, b.High)
		merged.Low = max(a.Low, b.Low)
	case DirDown:
		merged.High = min(a.High, b.High)
		merged.Low = min(a.Low, b.Low)
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
