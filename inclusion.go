package chanlun

// ──────────────────────────────────────────────
// §1  K 线包含处理
// ──────────────────────────────────────────────
//
// 包含关系定义：相邻两根 K 线，一根的高低点完全在另一根的高低点范围之内。
//
// 处理规则：
//   - 向上处理（前一非包含 K 线对构成向上关系）：取高高（max(H1,H2)）、高低（max(L1,L2)）
//   - 向下处理（前一非包含 K 线对构成向下关系）：取低高（min(H1,H2)）、低低（min(L1,L2)）
//   - 方向由最近一个非包含关系的 K 线对确定
//   - 从左到右逐根处理，合并后的新 K 线继续与后续 K 线比较

// MergeCandles 对 K 线序列进行包含处理，返回合并后的新序列。
// 输入必须是按时间升序排列的原始 K 线。
// 返回的 K 线序列中不存在包含关系。
func MergeCandles(candles []Candle) []Candle {
	if candles == nil {
		return nil
	}
	if len(candles) < 2 {
		result := make([]Candle, len(candles))
		copy(result, candles)
		return result
	}

	// 结果序列
	result := make([]Candle, 0, len(candles))
	result = append(result, candles[0])

	for i := 1; i < len(candles); i++ {
		current := candles[i]
		last := &result[len(result)-1]

		if isContained(current, *last) {
			// 存在包含关系，需要合并
			dir := determineDirection(result)
			merged := mergePair(*last, current, dir)
			result[len(result)-1] = merged
		} else {
			result = append(result, current)
		}
	}

	return result
}

// isContained 判断两根 K 线是否存在包含关系。
// 一根的高低点完全在另一根的高低点范围之内。
func isContained(a, b Candle) bool {
	return a.High <= b.High && a.Low >= b.Low ||
		b.High <= a.High && b.Low >= a.Low
}

// determineDirection 判断最近非包含关系的方向。
// 从结果序列的末尾向前查找最近的非包含关系 K 线对。
// 返回 1=向上, -1=向下, 0=无法确定。
func determineDirection(candles []Candle) Direction {
	if len(candles) < 2 {
		return DirNone
	}

	// 从末尾向前查找最近的非包含关系对
	for i := len(candles) - 1; i >= 1; i-- {
		prev := candles[i-1]
		curr := candles[i]

		if !isContained(prev, curr) {
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

// mergePair 根据方向合并两根有包含关系的 K 线。
// 向上处理：取高高（max(H)）、高低（max(L)）
// 向下处理：取低高（min(H)）、低低（min(L)）
func mergePair(a, b Candle, dir Direction) Candle {
	// 取一根时间戳较新的
	t := a.Time
	if b.Time.After(a.Time) {
		t = b.Time
	}

		switch dir {
		case DirUp:
			return Candle{
				Time:   t,
				Open:   a.Open, // 保留第一根的开盘价
				High:   max(a.High, b.High),
				Low:    max(a.Low, b.Low),
				Close:  b.Close, // 取最后一根的收盘价
				Volume: a.Volume + b.Volume,
			}
		case DirDown:
			return Candle{
				Time:   t,
				Open:   a.Open,
				High:   min(a.High, b.High),
				Low:    min(a.Low, b.Low),
				Close:  b.Close,
				Volume: a.Volume + b.Volume,
			}
		default:
			// 方向无法确定时（如序列最开头两两包含），保留第一根，丢弃当前根
			// 避免在没有方向信息的情况下做出假设
			return Candle{
				Time:   t,
				Open:   a.Open,
				High:   a.High,
				Low:    a.Low,
				Close:  a.Close,
				Volume: a.Volume + b.Volume,
			}
		}
}
