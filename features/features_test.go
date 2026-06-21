package features

import (
	"os"
	"testing"

	"github.com/bambuo/chan/types"
)

func TestCommon(t *testing.T) {
	bi := types.Bi{StartPrice: 100, EndPrice: 120}
	m := Common(bi)
	if m[KeyBiAmp] != 20 {
		t.Errorf("bsp_bi_amp = %.0f, want 20", m[KeyBiAmp])
	}
}

func TestT1(t *testing.T) {
	m := T1(0.5, 3)
	if m[KeyDivergenceRate] != 0.5 {
		t.Errorf("divergence_rate = %.1f, want 0.5", m[KeyDivergenceRate])
	}
	if m[KeyZsCnt] != 3 {
		t.Errorf("zs_cnt = %.0f, want 3", m[KeyZsCnt])
	}
}

func TestT1P(t *testing.T) {
	m := T1P(0.8, 15)
	if m[KeyDivergenceRate] != 0.8 {
		t.Errorf("divergence_rate = %.1f, want 0.8", m[KeyDivergenceRate])
	}
	if m[KeyBsp1BiAmp] != 15 {
		t.Errorf("bsp1_bi_amp = %.0f, want 15", m[KeyBsp1BiAmp])
	}
}

func TestT2(t *testing.T) {
	m := T2(0.3, 10, 5)
	if m[KeyBsp2Retrace] != 0.3 || m[KeyBsp2BreakAmp] != 10 || m[KeyBsp2BiAmp] != 5 {
		t.Errorf("T2 features mismatch: %v", m)
	}
}

func TestT2S(t *testing.T) {
	m := T2S(0.4, 12, 6, 2)
	if m[KeyBsp2sRetrace] != 0.4 || m[KeyBsp2sBreakAmp] != 12 || m[KeyBsp2sBiAmp] != 6 || m[KeyBsp2sLv] != 2 {
		t.Errorf("T2S features mismatch: %v", m)
	}
}

func TestT3(t *testing.T) {
	m := T3(110, 90, 25)
	if m[KeyBsp3BiAmp] != 25 {
		t.Errorf("bsp3_bi_amp = %.0f, want 25", m[KeyBsp3BiAmp])
	}
	expectedHeight := (110.0 - 90.0) / 90.0
	if m[KeyBsp3ZsHeight] != expectedHeight {
		t.Errorf("bsp3_zs_height = %.4f, want %.4f", m[KeyBsp3ZsHeight], expectedHeight)
	}
}

func TestT3_ZeroZD(t *testing.T) {
	m := T3(10, 0, 5)
	if m[KeyBsp3ZsHeight] != 0 {
		t.Errorf("bsp3_zs_height should be 0 when ZD=0, got %.4f", m[KeyBsp3ZsHeight])
	}
}

func TestMerge(t *testing.T) {
	a := map[string]float64{"x": 1, "y": 2}
	b := map[string]float64{"y": 3, "z": 4}
	m := Merge(a, b)
	if m["x"] != 1 || m["y"] != 3 || m["z"] != 4 {
		t.Errorf("Merge result: %v", m)
	}
}

func TestExportToLibSVM(t *testing.T) {
	signals := []types.Signal{
		{
			Type: types.BuyPoint1, Index: 10, Price: 100,
			Features: map[string]float64{"feature_a": 1.0, "feature_b": 2.0},
		},
		{
			Type: types.BuyPoint2, Index: 20, Price: 110,
			Features: map[string]float64{"feature_a": 3.0},
		},
	}
	labels := []int{1, 0}

	tmpDir := t.TempDir()
	prefix := tmpDir + "/test_feature"
	if err := ExportToLibSVM(signals, labels, prefix); err != nil {
		t.Fatalf("ExportToLibSVM failed: %v", err)
	}

	// 验证 meta 文件
	metaData, err := os.ReadFile(prefix + ".meta")
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	if len(metaData) == 0 {
		t.Error("meta file is empty")
	}

	// 验证 libsvm 文件
	libsvmData, err := os.ReadFile(prefix + ".libsvm")
	if err != nil {
		t.Fatalf("read libsvm: %v", err)
	}
	content := string(libsvmData)
	if len(content) == 0 {
		t.Fatal("libsvm file is empty")
	}
	// 第一行应为 "1 ..."
	if content[0] != '1' {
		t.Errorf("first line should start with '1', got %c", content[0])
	}
}

func TestExportToLibSVM_Errors(t *testing.T) {
	if err := ExportToLibSVM(nil, nil, ""); err == nil {
		t.Error("expected error for empty signals")
	}
	if err := ExportToLibSVM(
		[]types.Signal{{}},
		[]int{1, 2},
		"",
	); err == nil {
		t.Error("expected error for length mismatch")
	}
}
