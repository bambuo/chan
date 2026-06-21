package segment

import "github.com/bambuo/chan/types"

// buildDef 使用简单定义法（连续 3 笔重叠）构建线段。
//
// 与 buildDyh 共享 simpleSegment 实现：以连续三笔的重叠区间为线段起点，
// 同向延伸至反向笔出现为止。这是缠论"线段=至少连续三笔"的基础定义实现。
// def 与 dyh 在当前版本行为一致，保留两个入口是为了与 Config.SegAlgo
// 取值（"def"/"dyh"）兼容，未来若需差异化可在此分支扩展。
func buildDef(bis []types.MergedBi) []types.Segment {
	return simpleSegment(bis)
}
