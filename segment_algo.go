package chanlun

// ──────────────────────────────────────────────
// 线段划分算法策略接口
// ──────────────────────────────────────────────
//
// 支持三种线段划分算法（移植自 chan.py）：
//   - "chan": 标准缠论特征序列算法（默认）
//   - "dyh": DYH 算法（基于 situation1/situation2 判定）
//   - "def": 简化算法（基于相邻笔方向变化）

// SegmentBuilder 是线段划分算法的策略接口。
type SegmentBuilder interface {
	BuildSegments(bis []MergedBi) []Segment
}

// NewSegmentBuilder 根据算法名称创建对应的线段构建器。
func NewSegmentBuilder(algo string) SegmentBuilder {
	switch algo {
	case "dyh":
		return &dyhSegmentBuilder{}
	case "def":
		return &defSegmentBuilder{}
	default:
		return &chanSegmentBuilder{}
	}
}

// chanSegmentBuilder 使用标准缠论特征序列算法（已有实现）。
type chanSegmentBuilder struct{}

func (b *chanSegmentBuilder) BuildSegments(bis []MergedBi) []Segment {
	return buildSegmentsChan(bis)
}

// buildSegmentsChan 是原有的 BuildSegments 实现，重命名以支持策略分发。
func buildSegmentsChan(bis []MergedBi) []Segment {
	if len(bis) < 3 {
		return nil
	}

	segments := make([]Segment, 0)
	i := 0

	for i < len(bis) {
		seg, nextIdx := tryBuildSegment(bis, i)
		if seg == nil {
			i = nextIdx
			continue
		}
		segments = append(segments, *seg)
		i = nextIdx
	}

	return segments
}
