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

// BuildBis 从分型列表构建笔序列（向后兼容包装）。
// 使用 Config 中的笔配置参数进行成笔验证。
func BuildBis(klines []Kline, fractals []Fractal, minKLineCount int, minPriceRatio float64) []Bi {
	cfg := DefaultConfig()
	cfg.BiMinKLineCount = minKLineCount
	cfg.NewBiMinPriceRatio = minPriceRatio
	return BuildBisWithConfig(klines, fractals, cfg)
}

// BuildBisWithConfig 从分型列表构建笔序列（对齐 chan.py BiList 行为）。
//
// 成笔三重验证（移植自 chan.py BiList.can_make_bi）：
//  1. satisfyBiSpan: 分型间距 >= 4（严格）或 >= 3（非严格），支持 gap_as_kl
//  2. checkFxValid: 分型间价格关系合理（bi_fx_check: half/loss/strict/totally）
//  3. endIsPeak: 笔端点必须是两分型之间的极值点（bi_end_is_peak）
func BuildBisWithConfig(klines []Kline, fractals []Fractal, config Config) []Bi {
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

		// 检查方向
		var dir Direction
		if start.Type == BottomFractal && end.Type == TopFractal {
			dir = DirUp
		} else if start.Type == TopFractal && end.Type == BottomFractal {
			dir = DirDown
		} else {
			continue
		}

		// ── 检查 1: satisfyBiSpan（移植自 chan.py satisfy_bi_span）──
		if config.BiAlgo != "fx" {
			if !satisfyBiSpan(klines, start, end, config.BiStrict, config.GapAsKl) {
				continue
			}
		}

		// ── 检查 2: checkFxValid（移植自 chan.py check_fx_valid）──
		if !checkFxValid(klines, start, end, config.BiFxCheck) {
			continue
		}

		// ── 检查 3: endIsPeak（移植自 chan.py end_is_peak）──
		if config.BiEndIsPeak && !endIsPeak(klines, start, end) {
			continue
		}

		// 计算笔的统计信息
		bi := buildBiFromRange(klines, start, end, dir)

		// 新笔标准（严笔）：检查价格变动幅度
		if config.BiMinKLineCount >= 5 && config.NewBiMinPriceRatio > 0 {
			priceDiff := bi.Length
			avgPrice := (bi.StartPrice + bi.EndPrice) / 2
			if avgPrice > 0 && priceDiff/avgPrice < config.NewBiMinPriceRatio {
				continue
			}
		}

		if bi.KLineCount > 0 {
			bis = append(bis, bi)
		}
	}

	return bis
}

// ──────────────────────────────────────────────
// 成笔验证函数（移植自 chan.py BiList）
// ──────────────────────────────────────────────

// endIsPeak 检查两个分型之间的 K 线是否不超过终点分型的极值。
// 移植自 chan.py BiList.py#L216-L235。
//
// 底→顶笔（向上笔）：中间 K 线的 high 不能 > 终点顶分型的 high
// 顶→底笔（向下笔）：中间 K 线的 low 不能 < 终点底分型的 low
func endIsPeak(klines []Kline, start, end Fractal) bool {
	if start.Type == BottomFractal {
		// 底→顶：检查中间 K 线的 high 不超过终点的 high
		cmpThresh := end.High
		for i := start.Index + 1; i < end.Index && i < len(klines); i++ {
			if i < 0 {
				continue
			}
			if klines[i].High > cmpThresh {
				return false
			}
		}
	} else if start.Type == TopFractal {
		// 顶→底：检查中间 K 线的 low 不低于终点的 low
		cmpThresh := end.Low
		for i := start.Index + 1; i < end.Index && i < len(klines); i++ {
			if i < 0 {
				continue
			}
			if klines[i].Low < cmpThresh {
				return false
			}
		}
	}
	return true
}

// checkFxValid 验证顶底分型间的价格关系是否合理。
// 移植自 chan.py KLine.py#L45-L97 check_fx_valid。
//
// method: "half"(默认) | "loss" | "strict" | "totally"
//
// half: 检查分型与前/后一根 K 线的组合高低关系
// loss: 仅检查分型本身的高低关系
// strict: 使用三根 K 线范围（含前/后各一根）
// totally: 要求顶分型的 low > 底分型区域所有 high（完全不重叠）
func checkFxValid(klines []Kline, start, end Fractal, method string) bool {
	if start.Index < 0 || start.Index >= len(klines) || end.Index < 0 || end.Index >= len(klines) {
		return true // 索引越界时默认通过
	}

	if start.Type == TopFractal {
		// 顶→底笔
		var endHigh, startLow float64

		switch method {
		case "half":
			// 检查前两 KLC: end 用 pre+自身, start 用自身+next
			endPreHigh := klineHighAt(klines, end.Index-1)
			endHigh = max(endPreHigh, end.High)
			startNextLow := klineLowAt(klines, start.Index+1)
			startLow = min(start.Low, startNextLow)
		case "loss":
			// 仅检查分型本身
			endHigh = end.High
			startLow = start.Low
		case "strict", "totally":
			// 使用三根 K 线范围
			endPreHigh := klineHighAt(klines, end.Index-1)
			endNextHigh := klineHighAt(klines, end.Index+1)
			endHigh = max(endPreHigh, max(end.High, endNextHigh))
			startPreLow := klineLowAt(klines, start.Index-1)
			startNextLow := klineLowAt(klines, start.Index+1)
			startLow = min(startPreLow, min(start.Low, startNextLow))
		default:
			// 默认 half
			endPreHigh := klineHighAt(klines, end.Index-1)
			endHigh = max(endPreHigh, end.High)
			startNextLow := klineLowAt(klines, start.Index+1)
			startLow = min(start.Low, startNextLow)
		}

		if method == "totally" {
			return start.Low > endHigh
		}
		return start.High > endHigh && end.Low < startLow

	} else if start.Type == BottomFractal {
		// 底→顶笔
		var endLow, startHigh float64

		switch method {
		case "half":
			endPreLow := klineLowAt(klines, end.Index-1)
			endLow = min(endPreLow, end.Low)
			startNextHigh := klineHighAt(klines, start.Index+1)
			startHigh = max(start.High, startNextHigh)
		case "loss":
			endLow = end.Low
			startHigh = start.High
		case "strict", "totally":
			endPreLow := klineLowAt(klines, end.Index-1)
			endNextLow := klineLowAt(klines, end.Index+1)
			endLow = min(endPreLow, min(end.Low, endNextLow))
			startPreHigh := klineHighAt(klines, start.Index-1)
			startNextHigh := klineHighAt(klines, start.Index+1)
			startHigh = max(startPreHigh, max(start.High, startNextHigh))
		default:
			endPreLow := klineLowAt(klines, end.Index-1)
			endLow = min(endPreLow, end.Low)
			startNextHigh := klineHighAt(klines, start.Index+1)
			startHigh = max(start.High, startNextHigh)
		}

		if method == "totally" {
			return start.High < endLow
		}
		return start.Low < endLow && end.High > startHigh
	}

	return true
}

// satisfyBiSpan 检查两个分型间距是否满足成笔条件。
// 移植自 chan.py BiList.py#L149-L176 satisfy_bi_span + get_klc_span。
//
// strict 模式: span >= 4（分型中心之间的 K 线索引差）
// 非严格模式: span >= 3
// gapAsKl: 每个跳空额外 +1
func satisfyBiSpan(klines []Kline, start, end Fractal, strict, gapAsKl bool) bool {
	span := end.Index - start.Index

	if gapAsKl && span < 4 {
		// 计算跳空数量，每个跳空 +1
		for i := start.Index; i < end.Index && i < len(klines)-1; i++ {
			if i < 0 {
				continue
			}
			if hasGapBetween(klines[i], klines[i+1]) {
				span++
			}
		}
	}

	if strict {
		return span >= 4
	}
	// 非严格模式：span >= 3
	return span >= 3
}

// hasGapBetween 判断两根相邻 K 线之间是否有跳空。
func hasGapBetween(a, b Kline) bool {
	return a.High < b.Low || a.Low > b.High
}

// klineHighAt 安全获取指定索引 K 线的 High 值。
func klineHighAt(klines []Kline, idx int) float64 {
	if idx >= 0 && idx < len(klines) {
		return klines[idx].High
	}
	// 越界时返回极值，使 max/min 不受影响
	if idx < 0 {
		return 0 // 使 max 不受影响
	}
	return 0
}

// klineLowAt 安全获取指定索引 K 线的 Low 值。
func klineLowAt(klines []Kline, idx int) float64 {
	if idx >= 0 && idx < len(klines) {
		return klines[idx].Low
	}
	// 越界时返回极值，使 min 不受影响
	if idx < 0 {
		return 1e18
	}
	return 1e18
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
