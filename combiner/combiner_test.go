package combiner

import (
	"testing"

	"github.com/bambuo/chan/types"
)

func TestCombineSegments_SameDirection(t *testing.T) {
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 10, Direction: types.DirUp, Top: 115, Bottom: 100},
		{StartIndex: 11, EndIndex: 20, Direction: types.DirUp, Top: 125, Bottom: 110},
		{StartIndex: 21, EndIndex: 30, Direction: types.DirUp, Top: 130, Bottom: 120},
	}
	bis := CombineSegments(segs)
	if len(bis) != 1 {
		t.Fatalf("expected 1 combined bi, got %d", len(bis))
	}
	if bis[0].Direction != types.DirUp {
		t.Errorf("expected up direction")
	}
	if bis[0].StartIndex != 0 {
		t.Errorf("StartIndex: got %d, want 0", bis[0].StartIndex)
	}
	if bis[0].EndIndex != 30 {
		t.Errorf("EndIndex: got %d, want 30", bis[0].EndIndex)
	}
	if bis[0].Low != 100 {
		t.Errorf("Low: got %.0f, want 100", bis[0].Low)
	}
	if bis[0].High != 130 {
		t.Errorf("High: got %.0f, want 130", bis[0].High)
	}
}

func TestCombineSegments_Alternating(t *testing.T) {
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 10, Direction: types.DirUp, Top: 110, Bottom: 100},
		{StartIndex: 11, EndIndex: 20, Direction: types.DirDown, Top: 115, Bottom: 105},
		{StartIndex: 21, EndIndex: 30, Direction: types.DirUp, Top: 120, Bottom: 108},
	}
	bis := CombineSegments(segs)
	if len(bis) != 3 {
		t.Fatalf("expected 3 bis (each direction change), got %d", len(bis))
	}
	// 方向应交替：up → down → up
	if bis[0].Direction != types.DirUp {
		t.Errorf("bis[0] expected up")
	}
	if bis[1].Direction != types.DirDown {
		t.Errorf("bis[1] expected down")
	}
	if bis[2].Direction != types.DirUp {
		t.Errorf("bis[2] expected up")
	}
}

func TestCombineSegments_Empty(t *testing.T) {
	bis := CombineSegments(nil)
	if len(bis) != 0 {
		t.Errorf("expected 0 for nil, got %d", len(bis))
	}
}

func TestCombineSegments_StrictMode(t *testing.T) {
	segs := []types.Segment{
		{StartIndex: 0, EndIndex: 10, Direction: types.DirUp, Top: 110, Bottom: 100},
		{StartIndex: 11, EndIndex: 20, Direction: types.DirUp, Top: 120, Bottom: 105},
	}
	bis := CombineSegments(segs, Option{Strict: false})
	if len(bis) != 1 {
		t.Fatalf("expected 1 bi in non-strict, got %d", len(bis))
	}
}
