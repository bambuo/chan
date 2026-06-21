package validate

import (
	"math"
	"testing"

	"github.com/bambuo/chan/types"
)

func TestBi_Valid(t *testing.T) {
	upBi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 100, EndPrice: 110, KLineCount: 6}
	if err := Bi(upBi); err != nil {
		t.Errorf("valid up-bi: unexpected error: %v", err)
	}
	downBi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 110, EndPrice: 100, KLineCount: 6}
	if err := Bi(downBi); err != nil {
		t.Errorf("valid down-bi: unexpected error: %v", err)
	}
}

func TestBi_InvalidUp(t *testing.T) {
	// 向上笔终点 < 起点
	bi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 110, EndPrice: 100, KLineCount: 6}
	if err := Bi(bi); err == nil {
		t.Error("expected error for up-bi with end < start")
	}
}

func TestBi_InvalidDown(t *testing.T) {
	// 向下笔终点 > 起点
	bi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirDown, StartPrice: 100, EndPrice: 110, KLineCount: 6}
	if err := Bi(bi); err == nil {
		t.Error("expected error for down-bi with end > start")
	}
}

func TestBi_TooFewKlines(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 0, Direction: types.DirUp, StartPrice: 100, EndPrice: 110, KLineCount: 1}
	if err := Bi(bi); err == nil {
		t.Error("expected error for KLineCount < 2")
	}
}

func TestBi_NanPrice(t *testing.T) {
	bi := types.Bi{StartIndex: 0, EndIndex: 5, Direction: types.DirUp, StartPrice: 100, EndPrice: nan(), KLineCount: 6}
	if err := Bi(bi); err == nil {
		t.Error("expected error for NaN price")
	}
}

func TestBi_NegativeIndex(t *testing.T) {
	bi := types.Bi{StartIndex: -1, EndIndex: 5, Direction: types.DirUp, StartPrice: 100, EndPrice: 110, KLineCount: 6}
	if err := Bi(bi); err == nil {
		t.Error("expected error for negative index")
	}
}

func nan() float64 { return math.NaN() }
