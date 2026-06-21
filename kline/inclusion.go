package kline

import "github.com/bambuo/chan/types"

// MergeKlines 对 Kline 序列进行包含处理，返回合并后的新序列。
// 方向判定基于合并结果序列（对齐 chan.py KLine_Combiner.test_combine 使用合并后 KLC 的 high/low）。
func MergeKlines(klines []types.Kline, opts ...types.InclusionOption) []types.Kline {
	if klines == nil {
		return nil
	}
	if len(klines) < 2 {
		r := make([]types.Kline, len(klines))
		copy(r, klines)
		return r
	}
	var opt types.InclusionOption
	if len(opts) > 0 {
		opt = opts[0]
	}
	result := make([]types.Kline, 0, len(klines))
	result = append(result, klines[0])
	dir := types.DirUp
	for i := 1; i < len(klines); i++ {
		curr := klines[i]
		last := &result[len(result)-1]
		switch testContainment(*last, curr, opt.AllowTopEqual) {
		case containCombine:
			if opt.ExcludeIncluded {
				continue
			}
			if dir == types.DirUp && curr.High == curr.Low && curr.High == last.High {
				result = append(result, curr)
				dir = directionFromResult(result, dir)
				continue
			}
			if dir == types.DirDown && curr.High == curr.Low && curr.Low == last.Low {
				result = append(result, curr)
				dir = directionFromResult(result, dir)
				continue
			}
			result[len(result)-1] = mergePair(*last, curr, dir)
		case containUp:
			result = append(result, curr)
		case containDown:
			result = append(result, curr)
		}
		dir = directionFromResult(result, dir)
	}
	return result
}

// directionFromResult 从合并结果序列的最后两根 K 线判断方向（对齐 chan.py 合并后 KLC 比较）。
func directionFromResult(result []types.Kline, fallback types.Direction) types.Direction {
	if len(result) < 2 {
		return fallback
	}
	prev := result[len(result)-2]
	curr := result[len(result)-1]
	return updateDirectionFromRaw(prev, curr, fallback)
}
