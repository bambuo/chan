// Package combiner 提供线段合并功能：将同方向的连续线段合并为高级别笔。
//
// 对齐 Python Combiner 模块，是多级别递归的替代/补充路径。
// 合并后的高级别笔可直接用于 segment→pivot→trend→signal 的递归检测。
package combiner

import (
	"math"

	"github.com/bambuo/chan/types"
)

// Option 控制合并行为。
type Option struct {
	// Strict 为 true 时只合并严格同向的连续线段；
	// 为 false 时将异向线段也保留为独立笔（默认 false，保留全部）。
	Strict bool
}

// DefaultOption 返回默认合并选项。
func DefaultOption() Option {
	return Option{Strict: false}
}

// CombineSegments 将线段序列合并为高级别笔列表。
//
// 合并规则：
//   - 连续同向的线段合并为一笔（取首段起点到尾端终点、极值）
//   - 异向线段：Strict=false 时保留为独立笔，Strict=true 时跳过
//
// 返回的笔列表可直接用于 segment.BuildWithConfig → pivot.FindBiPivots 的递归检测。
func CombineSegments(segs []types.Segment, opt ...Option) []types.Bi {
	if len(segs) == 0 {
		return nil
	}
	o := DefaultOption()
	if len(opt) > 0 {
		o = opt[0]
	}

	var result []types.Bi
	i := 0
	for i < len(segs) {
		start := i
		dir := segs[i].Direction
		// 若 Strict 为 true，跳过方向不明确的线段
		if dir != types.DirUp && dir != types.DirDown {
			i++
			continue
		}
		for i+1 < len(segs) && segs[i+1].Direction == dir {
			i++
		}
		end := i
		// Strict 模式：跳过单线段（至少 2 条同向才合并）
		if o.Strict && start == end {
			i++
			continue
		}

		// 合并 [start, end] 中的同向线段
		first := segs[start]
		last := segs[end]

		var startPrice, endPrice float64
		switch dir {
		case types.DirUp:
			startPrice = first.Bottom
			endPrice = last.Top
		case types.DirDown:
			startPrice = first.Top
			endPrice = last.Bottom
		default:
			i++
			continue
		}

		// 取范围内极值
		hi := math.Inf(-1)
		lo := math.Inf(1)
		for j := start; j <= end; j++ {
			if segs[j].Top > hi {
				hi = segs[j].Top
			}
			if segs[j].Bottom < lo {
				lo = segs[j].Bottom
			}
		}
		if math.IsInf(hi, -1) {
			hi = first.Top
		}
		if math.IsInf(lo, 1) {
			lo = first.Bottom
		}

		ln := endPrice - startPrice
		if ln < 0 {
			ln = -ln
		}
		span := last.EndIndex - first.StartIndex
		if span <= 0 {
			span = 1
		}
		sl := 0.0
		if span > 0 {
			sl = ln / float64(span)
		}

		result = append(result, types.Bi{
			StartIndex: first.StartIndex,
			EndIndex:   last.EndIndex,
			Direction:  dir,
			StartPrice: startPrice,
			EndPrice:   endPrice,
			High:       hi,
			Low:        lo,
			Length:     ln,
			Slope:      sl,
			KLineCount: span,
		})

		i++
	}

	return result
}
