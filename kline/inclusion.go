package kline

import "github.com/bambuo/chan/types"

// MergeKlines 对 Kline 序列进行包含处理，返回合并后的新序列。
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
	for i := 1; i < len(klines); i++ {
		curr := klines[i]
		last := &result[len(result)-1]
		switch testContainment(*last, curr, opt.AllowTopEqual) {
		case containCombine:
			if opt.ExcludeIncluded {
				continue
			}
			dir := direction(result)
			if dir == types.DirUp && curr.High == curr.Low && curr.High == last.High {
				result = append(result, curr)
				continue
			}
			if dir == types.DirDown && curr.High == curr.Low && curr.Low == last.Low {
				result = append(result, curr)
				continue
			}
			result[len(result)-1] = mergePair(*last, curr, dir)
		case containUp:
			result = append(result, curr)
		case containDown:
			result = append(result, curr)
		}
	}
	return result
}
