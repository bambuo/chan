package pivot

import (
	"testing"

	"github.com/bambuo/chan/types"
)

func TestFindBiPivots_NormalMode(t *testing.T) {
	// normal 模式只在每个线段内部搜索反向笔来构建中枢。
	// 对一个 DirUp 的线段，需要至少 2 个向下笔来形成重叠区间。
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 90, EndPrice: 110, High: 112, Low: 88}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirDown, StartPrice: 110, EndPrice: 95, High: 112, Low: 93}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirUp, StartPrice: 95, EndPrice: 115, High: 117, Low: 93}},
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirDown, StartPrice: 115, EndPrice: 98, High: 117, Low: 96}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}

	// normal 模式下：向上线段 → 跳过向上笔 → 用向下笔（idx 1 和 3）构成中枢
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 20, Direction: types.DirUp, IsSure: true, BiList: bis},
	}

	cfg := types.DefaultConfig()
	cfg.ZsAlgo = "normal"
	cfg.ZsCombine = false
	cfg.OneBiZs = false

	pivots := FindBiPivots(bis, segs, cfg)
	if len(pivots) == 0 {
		t.Fatal("expected at least 1 pivot in normal mode")
	}
	p := pivots[0]
	if p.ZG <= p.ZD {
		t.Errorf("ZG (%.2f) must be > ZD (%.2f)", p.ZG, p.ZD)
	}
	if p.SourceLevel != "bi" {
		t.Errorf("expected sourceLevel=bi, got %q", p.SourceLevel)
	}
	if p.State != types.PivotFormed {
		t.Errorf("expected state=Formed, got %v", p.State)
	}
	t.Logf("normal mode pivot: ZG=%.2f ZD=%.2f state=%v bi[%d:%d]",
		p.ZG, p.ZD, p.State, p.BeginBiIdx, p.EndBiIdx)
}

func TestFindBiPivots_OverSegMode(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 90, EndPrice: 110, High: 112, Low: 88}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirDown, StartPrice: 110, EndPrice: 95, High: 112, Low: 93}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirUp, StartPrice: 95, EndPrice: 115, High: 117, Low: 93}},
		{Bi: types.Bi{StartIndex: 16, EndIndex: 20, Direction: types.DirDown, StartPrice: 115, EndPrice: 98, High: 117, Low: 96}},
		{Bi: types.Bi{StartIndex: 21, EndIndex: 25, Direction: types.DirUp, StartPrice: 98, EndPrice: 118, High: 120, Low: 96}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}

	cfg := types.DefaultConfig()
	cfg.ZsAlgo = "over_seg"
	cfg.ZsCombine = false

	pivots := FindBiPivots(bis, nil, cfg)
	if len(pivots) == 0 {
		t.Fatal("expected at least 1 pivot in over_seg mode")
	}
	for i, p := range pivots {
		if p.ZG <= p.ZD {
			t.Errorf("pivot[%d]: ZG (%.2f) must be > ZD (%.2f)", i, p.ZG, p.ZD)
		}
	}
}

func TestFindBiPivots_ShortData(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 90, EndPrice: 110}},
	}
	bis[0].OriginalCount = 1

	cfg := types.DefaultConfig()
	pivots := FindBiPivots(bis, nil, cfg)
	if len(pivots) != 0 {
		t.Errorf("expected 0 pivots for <3 bis, got %d", len(pivots))
	}
}

func TestUpdateZSInSeg(t *testing.T) {
	bis := []types.MergedBi{
		{Bi: types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 90, EndPrice: 110}},
		{Bi: types.Bi{StartIndex: 6, EndIndex: 10, Direction: types.DirDown, StartPrice: 110, EndPrice: 95}},
		{Bi: types.Bi{StartIndex: 11, EndIndex: 15, Direction: types.DirUp, StartPrice: 95, EndPrice: 115}},
	}
	for i := range bis {
		bis[i].OriginalCount = 1
	}

	pivots := []types.Pivot{
		{BeginBiIdx: 0, EndBiIdx: 2, ZG: 112, ZD: 95, SourceLevel: "bi", State: types.PivotFormed},
	}
	UpdateZSInSeg(bis, pivots)

	if pivots[0].BiIn != nil {
		t.Logf("BiIn: start=%d end=%d dir=%v", pivots[0].BiIn.StartIndex, pivots[0].BiIn.EndIndex, pivots[0].BiIn.Direction)
	}
	if len(pivots[0].BiList) != 3 {
		t.Errorf("expected 3 bi in pivot.BiList, got %d", len(pivots[0].BiList))
	}
}
