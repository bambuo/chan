package chanlun

import "time"

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

// MergeKlines 对 Kline 序列进行包含处理，返回合并后的新序列。
// 输入必须是按时间升序排列的原始 K 线。
// 返回的 K 线序列中不存在包含关系。
// 可选参数 opts 控制精细包含行为（一字K线、顶底相等、跳过模式）。
func MergeKlines(klines []Kline, opts ...InclusionOption) []Kline {
	if klines == nil {
		return nil
	}
	if len(klines) < 2 {
		result := make([]Kline, len(klines))
		copy(result, klines)
		return result
	}

	// 解析选项
	var opt InclusionOption
	if len(opts) > 0 {
		opt = opts[0]
	}

	// 结果序列
	result := make([]Kline, 0, len(klines))
	result = append(result, klines[0])

	for i := 1; i < len(klines); i++ {
		current := klines[i]
		last := &result[len(result)-1]

		// 精细包含判定
		containDir := testContainment(*last, current, opt.AllowTopEqual)

		switch containDir {
		case containCombine:
			if opt.ExcludeIncluded {
				// exclude_included 模式：被包含的直接跳过
				continue
			}
			dir := determineDirection(result)
			// 一字K线方向处理（对齐 chan.py KLine_Combiner.try_add）：
			// Python 条件：if combine_item.high != combine_item.low or combine_item.high != self.high
			// 取反（执行合并的条件）：is一字 AND current.High == last.High
			if dir == DirUp && current.High == current.Low && current.High == last.High {
				// 一字K且价格相同，跳过合并
				result = append(result, current)
				continue
			}
			if dir == DirDown && current.High == current.Low && current.Low == last.Low {
				result = append(result, current)
				continue
			}
			merged := mergePair(*last, current, dir)
			result[len(result)-1] = merged
		case containUp:
			result = append(result, current)
		case containDown:
			result = append(result, current)
		}
	}

	return result
}

// containmentResult 枚举包含判定结果。
type containmentResult int

const (
	containCombine containmentResult = iota // 需要合并
	containUp                               // 向上关系
	containDown                             // 向下关系
)

// testContainment 精细包含判定（移植自 chan.py CKLine_Combiner.test_combine）。
// allowTopEqual: 1=被包含时顶相等不合并, -1=被包含时底相等不合并, 0=标准模式
func testContainment(last, curr Kline, allowTopEqual int) containmentResult {
	// last 包含 curr（curr 在 last 范围内）
	if last.High >= curr.High && last.Low <= curr.Low {
		return containCombine
	}
	// curr 包含 last（last 在 curr 范围内）
	if last.High <= curr.High && last.Low >= curr.Low {
		// allowTopEqual 精细控制
		if allowTopEqual == 1 && last.High == curr.High && last.Low > curr.Low {
			return containDown
		}
		if allowTopEqual == -1 && last.Low == curr.Low && last.High < curr.High {
			return containUp
		}
		return containCombine
	}
	// 向上关系
	if last.High < curr.High && last.Low < curr.Low {
		return containUp
	}
	// 向下关系
	if last.High > curr.High && last.Low > curr.Low {
		return containDown
	}
	// 其他情况（不应出现），默认合并
	return containCombine
}

// isContained 判断两根 K 线是否存在包含关系。
// 一根的高低点完全在另一根的高低点范围之内。
func isContained(a, b Kline) bool {
	return a.High <= b.High && a.Low >= b.Low ||
		b.High <= a.High && b.Low >= a.Low
}

// determineDirection 判断最近非包含关系的方向。
// 从结果序列的末尾向前查找最近的非包含关系 K 线对。
// 返回 1=向上, -1=向下, 0=无法确定。
func determineDirection(klines []Kline) Direction {
	if len(klines) < 2 {
		return DirNone
	}

	// 从末尾向前查找最近的非包含关系对
	for i := len(klines) - 1; i >= 1; i-- {
		prev := klines[i-1]
		curr := klines[i]

		if !isContained(prev, curr) {
			if curr.High >= prev.High {
				return DirUp
			}
			if curr.Low <= prev.Low {
				return DirDown
			}
		}
	}

	return DirNone
}

// mergePair 根据方向合并两根有包含关系的 K 线。
// 向上处理：取高高（max(H)）、高低（max(L)）
// 向下处理：取低高（min(H)）、低低（min(L)）
func mergePair(a, b Kline, dir Direction) Kline {
	// 取一根时间戳较新的
	t := a.Time
	if b.Time.After(a.Time) {
		t = b.Time
	}

	merged := mergeKlineMeta(a, b, t)
	switch dir {
	case DirUp:
		merged.High = max(a.High, b.High)
		merged.Low = max(a.Low, b.Low)
	case DirDown:
		merged.High = min(a.High, b.High)
		merged.Low = min(a.Low, b.Low)
	default:
		// 方向无法确定时（如序列最开头两两包含），保留第一根价格区间。
		merged.High = a.High
		merged.Low = a.Low
		merged.Close = a.Close
	}
	return merged
}

func mergeKlineMeta(a, b Kline, t time.Time) Kline {
	return Kline{
		Time:            t,
		Open:            a.Open,
		High:            a.High,
		Low:             a.Low,
		Close:           b.Close,
		BaseVolume:      a.BaseVolume + b.BaseVolume,
		QuoteVolume:     a.QuoteVolume + b.QuoteVolume,
		Turnover:        a.Turnover + b.Turnover,
		TradeCount:      a.TradeCount + b.TradeCount,
		RawVolumeUnit:   preferUnit(a.RawVolumeUnit, b.RawVolumeUnit),
		RawTurnoverUnit: preferUnit(a.RawTurnoverUnit, b.RawTurnoverUnit),
	}
}

func preferUnit(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
