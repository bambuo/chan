package segment

import "github.com/bambuo/chan/types"

func buildDyh(bis []types.MergedBi) []types.Segment {
	if len(bis) < 3 {
		return nil
	}
	var segs []types.Segment
	i := 0
	for i < len(bis) {
		s, n := tryBuildDyh(bis, i)
		if s == nil {
			i = n
			continue
		}
		segs = append(segs, *s)
		i = n
	}
	return segs
}

func tryBuildDyh(bis []types.MergedBi, start int) (*types.Segment, int) {
	if start+2 >= len(bis) {
		return nil, len(bis)
	}
	b0, b1, b2 := bis[start], bis[start+1], bis[start+2]
	oh := min(min(b0.High, b1.High), b2.High)
	ol := max(max(b0.Low, b1.Low), b2.Low)
	if oh < ol {
		return nil, start + 1
	}
	s := types.Segment{
		StartIndex: b0.StartIndex, Direction: b0.Direction,
		BiList: []types.MergedBi{b0, b1, b2},
		Top:    max(max(b0.High, b1.High), b2.High),
		Bottom: min(min(b0.Low, b1.Low), b2.Low),
		IsSure: true,
	}
	s, np := extendDyh(s, bis, start+3)
	return &s, np
}

func extendDyh(s types.Segment, bis []types.MergedBi, pos int) (types.Segment, int) {
	for pos < len(bis) {
		cur := bis[pos]
		s.BiList = append(s.BiList, cur)
		s.EndIndex = cur.EndIndex
		if cur.High > s.Top {
			s.Top = cur.High
		}
		if cur.Low < s.Bottom {
			s.Bottom = cur.Low
		}
		if cur.Direction != s.Direction {
			s.IsBroken = true
			s.BreakType = types.BreakStd
			s.ConfirmIndex = cur.EndIndex
			return s, pos + 1
		}
		pos++
	}
	s.EndIndex = bis[len(bis)-1].EndIndex
	return s, len(bis)
}
