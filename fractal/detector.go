package fractal

import (
	"fmt"

	"github.com/bambuo/chan/types"
)

// FindFractals 在经包含处理后的 Kline 序列上识别所有客观分型。
func FindFractals(klines []types.Kline) []types.Fractal {
	if len(klines) < 3 {
		return nil
	}
	var result []types.Fractal
	for i := 1; i < len(klines)-1; i++ {
		prev, mid, next := klines[i-1], klines[i], klines[i+1]
		if mid.High > prev.High && mid.High > next.High &&
			mid.Low > prev.Low && mid.Low > next.Low {
			result = append(result, types.Fractal{
				Type: types.TopFractal, Index: i,
				High: mid.High, Low: mid.Low,
				Strength: strength(mid, prev, next),
			})
		} else if mid.Low < prev.Low && mid.Low < next.Low &&
			mid.High < prev.High && mid.High < next.High {
			result = append(result, types.Fractal{
				Type: types.BottomFractal, Index: i,
				High: mid.High, Low: mid.Low,
				Strength: strength(mid, prev, next),
			})
		}
	}
	return result
}

// FilterForBi 筛选可用于成笔的有效顶底分型。
func FilterForBi(fractals []types.Fractal, minGap int) []types.Fractal {
	if len(fractals) == 0 {
		return nil
	}
	r := make([]types.Fractal, 0, len(fractals))
	r = append(r, fractals[0])
	for i := 1; i < len(fractals); i++ {
		last, curr := &r[len(r)-1], fractals[i]
		if curr.Type == last.Type {
			if curr.Type == types.TopFractal && curr.High > last.High {
				r[len(r)-1] = curr
			} else if curr.Type == types.BottomFractal && curr.Low < last.Low {
				r[len(r)-1] = curr
			}
		} else if idxGap(*last, curr) >= minGap {
			r = append(r, curr)
		}
	}
	return r
}

func idxGap(a, b types.Fractal) int { return (b.Index - 1) - (a.Index + 1) - 1 }

func strength(mid, prev, next types.Kline) float64 {
	body := abs(mid.Close-mid.Open) / (mid.High - mid.Low + 1e-10)
	gap := 0.0
	if (mid.Low > prev.High) || (mid.High < prev.Low) {
		gap = 0.2
	}
	if (next.Low > mid.High) || (next.High < mid.Low) {
		gap = 0.2
	}
	s := body + gap
	if s > 1 {
		s = 1
	}
	return s
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ValidateFractals 对分型序列做调试检测，返回警告信息列表。
// 检测内容：
//  1. 顶底分型是否共用同一 K 线索引（理论不应发生，但可用于发现边界 bug）
//  2. FilterForBi 输出是否严格交替（顶-底-顶-底）
func ValidateFractals(raw, filtered []types.Fractal) []string {
	var warnings []string

	// 检测 1：原始分型中的索引冲突
	seen := make(map[int]types.FractalType)
	for _, f := range raw {
		if prev, ok := seen[f.Index]; ok && prev != f.Type {
			warnings = append(warnings,
				fmt.Sprintf("分型冲突: K线[%d] 同时被识别为顶分型和底分型", f.Index))
		}
		seen[f.Index] = f.Type
	}

	// 检测 2：筛选后分型是否交替
	if len(filtered) >= 2 {
		for i := 1; i < len(filtered); i++ {
			if filtered[i].Type == filtered[i-1].Type {
				warnings = append(warnings,
					fmt.Sprintf("分型交替异常: filtered[%d] type=%v 与 filtered[%d] type=%v 同向",
						i-1, filtered[i-1].Type, i, filtered[i].Type))
			}
		}
	}

	// 检测 3：筛选后分型数量合理性（至少需要 2 个不同方向的分型才能成笔）
	if len(filtered) > 0 && len(filtered) < 2 {
		warnings = append(warnings, "分型数量不足: 筛选后少于 2 个分型，无法构建任何笔")
	}

	return warnings
}
