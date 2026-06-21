package segment

import "github.com/bambuo/chan/types"

// buildDef 使用简单定义法（连续 3 笔重叠）构建线段。
//
// 当前实现与 buildDyh 共享 simpleSegment 逻辑，未来可按需差异化。
func buildDef(bis []types.MergedBi) []types.Segment {
	return simpleSegment(bis, "def")
}
