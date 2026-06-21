package segment

import "github.com/bambuo/chan/types"

// BuildSegments 从笔序列构建线段（默认 chan 算法）。
func BuildSegments(bis []types.MergedBi) []types.Segment {
	return buildWithAlgo(bis, "chan")
}

// BuildWithAlgo 使用指定算法从笔序列构建线段。
func BuildWithAlgo(bis []types.MergedBi, algo string) []types.Segment {
	return buildWithAlgo(bis, algo)
}

// BuildWithConfig 使用指定算法和配置构建线段，含左侧延伸。
func BuildWithConfig(bis []types.MergedBi, algo, leftMethod string) []types.Segment {
	segs := buildWithAlgo(bis, algo)
	if leftMethod == "peak" || leftMethod == "all" {
		segs = CollectLeftSeg(segs, bis, leftMethod)
	}
	return segs
}

func buildWithAlgo(bis []types.MergedBi, algo string) []types.Segment {
	switch algo {
	case "dyh":
		return buildDyh(bis)
	case "def":
		return buildDef(bis)
	default:
		return buildChan(bis)
	}
}
