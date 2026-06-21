package segment

import (
	"math"
	"testing"
	"time"

	"github.com/bambuo/chan/types"
)

// TestSegmentLeftCollection_SingleBi 测试剩余单笔收集（对齐 Python collect_left_as_seg）。
func TestSegmentLeftCollection_SingleBi(t *testing.T) {
	// 构建 4 笔，第一段结束后剩 1 笔
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 4, Direction: types.DirUp, StartPrice: 100, EndPrice: 110, High: 112, Low: 98}},
		{Bi: types.Bi{StartIndex: 5, EndIndex: 9, Direction: types.DirDown, StartPrice: 110, EndPrice: 102, High: 111, Low: 100}},
		{Bi: types.Bi{StartIndex: 10, EndIndex: 14, Direction: types.DirUp, StartPrice: 102, EndPrice: 115, High: 116, Low: 101}},
		{Bi: types.Bi{StartIndex: 15, EndIndex: 19, Direction: types.DirDown, StartPrice: 115, EndPrice: 108, High: 117, Low: 107}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}

	segs := BuildWithConfig(bis, "chan", "peak")
	if len(segs) < 1 {
		t.Fatal("expected at least 1 segment")
	}

	// 剩余笔应被收集为尾部线段
	t.Logf("segments: %d", len(segs))
	for i, s := range segs {
		t.Logf("  seg[%d]: dir=%v sure=%v start=%d end=%d bis=%d",
			i, s.Direction, s.IsSure, s.StartIndex, s.EndIndex, len(s.BiList))
	}
}

// TestSegmentLeftCollection_NoSegs 测试无确定线段时的首段收集。
func TestSegmentLeftCollection_NoSegs(t *testing.T) {
	bis := makeBisFromSine(60)
	segs := buildWithAlgo(bis, "chan")
	if len(segs) == 0 {
		t.Log("buildChan returned 0 segments (expected for short data)")
		// CollectLeftSeg 应能从所有笔推断首段
		segs = CollectLeftSeg(segs, bis, "peak")
		t.Logf("after CollectLeftSeg: %d segments", len(segs))
		if len(segs) < 1 {
			t.Error("expected CollectLeftSeg to produce at least 1 segment")
		}
	}
}

func makeBisFromSine(n int) []types.MergedBi {
	k := make([]types.Kline, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range k {
		p := float64(i) * 0.4
		tr := float64(i) * 0.15
		mid := 100.0 + tr + 8.0*math.Sin(p)
		k[i] = types.Kline{
			Time: types.DateTime{Time: t.Add(time.Duration(i) * time.Hour)},
			Open: mid - 0.5, High: mid + 2.0, Low: mid - 1.5, Close: mid + 0.3,
		}
	}

	// Simple bi building for test
	var bis []types.MergedBi
	dir := types.DirUp
	start := 0
	for i := 3; i < n-2; i++ {
		prev, mid, next := k[i-1], k[i], k[i+1]
		isTop := mid.High > prev.High && mid.High > next.High
		isBottom := mid.Low < prev.Low && mid.Low < next.Low
		if (dir == types.DirUp && isTop) || (dir == types.DirDown && isBottom) {
			bi := types.Bi{
				StartIndex: start, EndIndex: i, Direction: dir,
				StartPrice: k[start].Close, EndPrice: k[i].Close,
				High: mid.High, Low: mid.Low,
			}
			bis = append(bis, types.MergedBi{Bi: bi, OriginalCount: 1})
			start = i
			if dir == types.DirUp {
				dir = types.DirDown
			} else {
				dir = types.DirUp
			}
		}
	}
	return bis
}
