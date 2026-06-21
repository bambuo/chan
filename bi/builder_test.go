package bi

import (
	"math"
	"testing"
	"time"

	"github.com/bambuo/chan/types"
)

// TestBiBuilder_NoZeroPriceAfterDelVirt 回归测试 S7：
// delVirt 恢复 lastEnd 时曾遗漏 High/Low 字段，导致后续 add 生成 StartPrice=0 的非法笔。
// 此测试构造会触发 delVirt 恢复路径的序列，验证所有笔价格非零。
func TestBiBuilder_NoZeroPriceAfterDelVirt(t *testing.T) {
	klines, _ := sineWaveData(300)
	cfg := alignedTestConfig()
	fractals := detectFractalsForTest(klines, cfg)
	bis := BuildBis(klines, fractals, cfg)

	if len(bis) == 0 {
		t.Skip("no bis generated")
	}
	for i, b := range bis {
		if b.StartPrice == 0 {
			t.Errorf("bi[%d] StartPrice=0 (S7 regression: delVirt lost High/Low)", i)
		}
		if b.EndPrice == 0 {
			t.Errorf("bi[%d] EndPrice=0", i)
		}
		// 向上笔终点必须高于起点，向下笔终点必须低于起点
		if b.IsUp() && b.EndPrice <= b.StartPrice {
			t.Errorf("bi[%d] 向上笔终点 %.4f 不高于起点 %.4f", i, b.EndPrice, b.StartPrice)
		}
		if b.IsDown() && b.EndPrice >= b.StartPrice {
			t.Errorf("bi[%d] 向下笔终点 %.4f 不低于起点 %.4f", i, b.EndPrice, b.StartPrice)
		}
	}
}

// TestBiBuilder_VirtualBiExtensionPreservesDirection 回归测试 S7：
// tryAddVirtualBi 同向延伸曾用 Close 作为 EndPrice，导致向下笔终点高于起点（方向反转）。
// 此测试构造尾部强势反弹的序列，验证虚笔延伸不破坏方向合法性。
func TestBiBuilder_VirtualBiExtensionPreservesDirection(t *testing.T) {
	// 构造：先形成一段明确的向下笔，然后尾部强势反弹（Close 远高于向下笔终点）。
	// 修复前：虚笔同向延伸会用反弹的 Close 覆盖向下笔终点，造成 ep > sp。
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := []types.Kline{}
	// 下跌段：从 100 逐步跌到 80
	prices := []float64{100, 98, 95, 92, 90, 88, 85, 82, 80}
	for i, p := range prices {
		klines = append(klines, types.Kline{
			Time: types.DateTime{Time: base.Add(time.Duration(i) * time.Hour)},
			Open: p + 1, High: p + 2, Low: p - 1, Close: p, BaseVolume: 1000,
		})
	}
	// 尾部强势反弹：Close 从 80 飙到 120，但 Low 仍维持低位
	// 这样向下笔的"同向延伸"条件 (Low 刷新) 不应触发；
	// 即便触发，EndPrice 也应取 Low 而非 Close=120。
	for i := 0; i < 10; i++ {
		p := 80 + float64(i)*4 // Close 从 80 升到 116
		klines = append(klines, types.Kline{
			Time: types.DateTime{Time: base.Add(time.Duration(9+i) * time.Hour)},
			Open: p - 2, High: p + 5, Low: 79, Close: p, BaseVolume: 1000,
		})
	}

	cfg := alignedTestConfig()
	// 放宽过滤以确保能成笔
	cfg.BiStrict = false
	fractals := detectFractalsForTest(klines, cfg)
	bis := BuildBis(klines, fractals, cfg)

	for i, b := range bis {
		if b.IsDown() && b.EndPrice > b.StartPrice {
			t.Errorf("bi[%d] 向下笔终点 %.2f 高于起点 %.2f (S7: 虚笔延伸用 Close 覆盖了方向极值)",
				i, b.EndPrice, b.StartPrice)
		}
		if b.IsUp() && b.EndPrice < b.StartPrice {
			t.Errorf("bi[%d] 向上笔终点 %.2f 低于起点 %.2f", i, b.EndPrice, b.StartPrice)
		}
	}
}

func TestMachineTruncateStateKeepsSideTablesAligned(t *testing.T) {
	m := &machine{
		bis:  []types.Bi{{}, {}, {}},
		sure: []bool{true, false, true},
		used: []bool{true, true, false},
		ends: [][]types.Fractal{{}, {}, {}},
	}

	m.truncateState(2)

	if len(m.bis) != 2 || len(m.sure) != 2 || len(m.used) != 2 || len(m.ends) != 2 {
		t.Fatalf("state lengths not aligned after truncate: bis=%d sure=%d used=%d ends=%d",
			len(m.bis), len(m.sure), len(m.used), len(m.ends))
	}
}

// TestBiBuilder_TrailingVirtualBi 测试 tryAddVirtualBi：末尾K线能构成虚拟笔。
func TestBiBuilder_TrailingVirtualBi(t *testing.T) {
	klines, _ := sineWaveData(200)
	cfg := alignedTestConfig()

	fractals := detectFractalsForTest(klines, cfg)
	bis := BuildBis(klines, fractals, cfg)

	// 应产生 > 20 笔
	if len(bis) < 20 {
		t.Fatalf("expected at least 20 bis, got %d", len(bis))
	}

	// 最后一笔应覆盖到数据末尾附近
	lastBi := bis[len(bis)-1]
	lastKlIdx := len(klines) - 1
	if lastBi.EndIndex < lastKlIdx-5 {
		t.Errorf("last bi ends at %d, but data has %d K-lines — trailing bi not created?",
			lastBi.EndIndex, lastKlIdx)
	}

	// 检查方向交替
	for i := 1; i < len(bis); i++ {
		if bis[i].Direction == bis[i-1].Direction {
			t.Errorf("bi[%d] and bi[%d] have same direction %v", i-1, i, bis[i].Direction)
		}
	}
}

// TestBiBuilder_EndToEnd 端到端对比测试（与 Python 参考输出对齐）。
func TestBiBuilder_EndToEnd(t *testing.T) {
	klines, _ := sineWaveData(200)
	cfg := alignedTestConfig()

	fractals := detectFractalsForTest(klines, cfg)
	bis := BuildBis(klines, fractals, cfg)

	// 确认笔数量（对齐 Python chan.py 输出）
	if len(bis) != 25 {
		t.Errorf("bi count: got %d, want 25 (matching Python chan.py)", len(bis))
	}

	// 确认每笔的起止索引（对齐 Python）
	expected := []struct{ start, end int }{
		{4, 12}, {12, 20}, {20, 27}, {27, 35}, {35, 43},
		{43, 51}, {51, 59}, {59, 67}, {67, 74}, {74, 83},
		{83, 90}, {90, 98}, {98, 106}, {106, 114}, {114, 122},
		{122, 130}, {130, 137}, {137, 145}, {145, 153}, {153, 161},
		{161, 169}, {169, 177}, {177, 184}, {184, 193}, {193, 199},
	}
	if len(bis) != len(expected) {
		t.Fatalf("expected %d bis, got %d", len(expected), len(bis))
	}
	for i, e := range expected {
		if bis[i].StartIndex != e.start || bis[i].EndIndex != e.end {
			t.Errorf("bi[%d]: got [%d,%d], want [%d,%d]",
				i, bis[i].StartIndex, bis[i].EndIndex, e.start, e.end)
		}
	}
}

// TestBiBuilder_ShortData 短数据边界测试。
func TestBiBuilder_ShortData(t *testing.T) {
	klines := []types.Kline{
		{Time: types.DateTime{Time: time.Now()}, Open: 100, High: 102, Low: 99, Close: 101},
		{Time: types.DateTime{Time: time.Now()}, Open: 101, High: 105, Low: 100, Close: 104},
		{Time: types.DateTime{Time: time.Now()}, Open: 104, High: 106, Low: 102, Close: 103},
		{Time: types.DateTime{Time: time.Now()}, Open: 103, High: 104, Low: 98, Close: 99},
		{Time: types.DateTime{Time: time.Now()}, Open: 99, High: 100, Low: 96, Close: 97},
	}
	cfg := alignedTestConfig()
	fractals := detectFractalsForTest(klines, cfg)
	bis := BuildBis(klines, fractals, cfg)

	// 短数据不应 panic，笔数应合理
	if len(bis) > len(klines) {
		t.Errorf("too many bis: %d for %d klines", len(bis), len(klines))
	}
}

// ── 辅助 ──

func alignedTestConfig() types.Config {
	cfg := types.DefaultConfig()
	cfg.BiStrict = true
	cfg.BiAlgo = "normal"
	cfg.BiFxCheck = "strict"
	cfg.GapAsKl = true
	cfg.BiEndIsPeak = true
	cfg.BiAllowSubPeak = true
	cfg.SegAlgo = "chan"
	cfg.LeftSegMethod = "peak"
	cfg.ZsAlgo = "normal"
	cfg.ZsCombine = true
	cfg.ZsCombineMode = "zs"
	cfg.OneBiZs = false
	cfg.BspDivergenceRate = math.Inf(1)
	cfg.BspMinZsCnt = 1
	cfg.Bsp1OnlyMultiBiZs = true
	cfg.BspMaxBs2Rate = 0.9999
	cfg.BspMacdAlgo = "peak"
	cfg.Bsp1Peak = true
	cfg.BspType = "1,1p,2,2s,3a,3b"
	cfg.Bsp2Follow1 = true
	cfg.Bsp3Follow1 = true
	cfg.Bsp3Peak = false
	cfg.StrictBsp3 = false
	return cfg
}

func detectFractalsForTest(klines []types.Kline, cfg types.Config) []types.Fractal {
	// 先做包含处理
	merged := make([]types.Kline, len(klines))
	copy(merged, klines)
	// 简单包含处理（测试用，不依赖 kline 包）
	return simpleFindFractals(merged)
}

func simpleFindFractals(klines []types.Kline) []types.Fractal {
	if len(klines) < 3 {
		return nil
	}
	var fractals []types.Fractal
	for i := 1; i < len(klines)-1; i++ {
		prev, mid, next := klines[i-1], klines[i], klines[i+1]
		if mid.High > prev.High && mid.High > next.High &&
			mid.Low > prev.Low && mid.Low > next.Low {
			fractals = append(fractals, types.Fractal{
				Type:  types.TopFractal,
				Index: i,
				High:  mid.High,
				Low:   mid.Low,
			})
		} else if mid.Low < prev.Low && mid.Low < next.Low &&
			mid.High < prev.High && mid.High < next.High {
			fractals = append(fractals, types.Fractal{
				Type:  types.BottomFractal,
				Index: i,
				High:  mid.High,
				Low:   mid.Low,
			})
		}
	}
	// 简单过滤（保留更极端的同类型分型）
	if len(fractals) <= 1 {
		return fractals
	}
	filtered := []types.Fractal{fractals[0]}
	for i := 1; i < len(fractals); i++ {
		last := &filtered[len(filtered)-1]
		cur := fractals[i]
		if last.Type == cur.Type {
			if (cur.Type == types.TopFractal && cur.High > last.High) ||
				(cur.Type == types.BottomFractal && cur.Low < last.Low) {
				filtered[len(filtered)-1] = cur
			}
		} else {
			if cur.Index-last.Index >= 4 {
				filtered = append(filtered, cur)
			}
		}
	}
	return filtered
}

func sineWaveData(n int) ([]types.Kline, []types.Fractal) {
	k := make([]types.Kline, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range k {
		p := float64(i) * 0.4
		tr := float64(i) * 0.15
		mid := 100.0 + tr + 8.0*math.Sin(p)
		k[i] = types.Kline{
			Time:       types.DateTime{Time: t.Add(time.Duration(i) * time.Hour)},
			Open:       mid - 0.5,
			High:       mid + 2.0,
			Low:        mid - 1.5,
			Close:      mid + 0.3,
			BaseVolume: 1000,
		}
	}
	// 先做包含处理再检测分型
	merged := simpleMergeKlines(k)
	fractals := simpleFindFractals(merged)
	return k, fractals
}

func simpleMergeKlines(klines []types.Kline) []types.Kline {
	if len(klines) < 2 {
		return klines
	}
	result := []types.Kline{klines[0]}
	for i := 1; i < len(klines); i++ {
		last := &result[len(result)-1]
		cur := klines[i]
		// 包含关系检测
		if last.High >= cur.High && last.Low <= cur.Low {
			continue // 前包含后，跳过
		}
		if last.High <= cur.High && last.Low >= cur.Low {
			// 后包含前，替换
			result[len(result)-1] = cur
			continue
		}
		result = append(result, cur)
	}
	return result
}

func TestBuildVirtualBi_UpThenDown(t *testing.T) {
	klines := []types.Kline{
		{Time: types.DateTime{Time: time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC)}, Open: 100, High: 105, Low: 95, Close: 102},
		{Time: types.DateTime{Time: time.Date(2026, 1, 1, 9, 31, 0, 0, time.UTC)}, Open: 102, High: 110, Low: 100, Close: 108},
		{Time: types.DateTime{Time: time.Date(2026, 1, 1, 9, 32, 0, 0, time.UTC)}, Open: 108, High: 109, Low: 101, Close: 102},
		{Time: types.DateTime{Time: time.Date(2026, 1, 1, 9, 33, 0, 0, time.UTC)}, Open: 102, High: 106, Low: 99, Close: 100},
	}
	bis := []types.Bi{
		{StartIndex: 0, EndIndex: 1, Direction: types.DirUp, StartPrice: 95, EndPrice: 110, High: 110, Low: 95},
	}
	v := BuildVirtualBi(klines, bis)
	if v == nil {
		t.Fatal("expected virtual bi, got nil")
	}
	if v.Direction != types.DirDown {
		t.Errorf("expected down virtual bi, got %d", v.Direction)
	}
	if v.EndPrice >= v.StartPrice {
		t.Errorf("virtual bi down must have end < start: %.2f >= %.2f", v.EndPrice, v.StartPrice)
	}
}

func TestBuildVirtualBi_DownThenUp(t *testing.T) {
	klines := []types.Kline{
		{Time: types.DateTime{Time: time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC)}, Open: 105, High: 110, Low: 100, Close: 108},
		{Time: types.DateTime{Time: time.Date(2026, 1, 1, 9, 31, 0, 0, time.UTC)}, Open: 108, High: 102, Low: 90, Close: 95},
		{Time: types.DateTime{Time: time.Date(2026, 1, 1, 9, 32, 0, 0, time.UTC)}, Open: 95, High: 103, Low: 92, Close: 100},
		{Time: types.DateTime{Time: time.Date(2026, 1, 1, 9, 33, 0, 0, time.UTC)}, Open: 100, High: 115, Low: 98, Close: 112},
	}
	bis := []types.Bi{
		{StartIndex: 0, EndIndex: 1, Direction: types.DirDown, StartPrice: 110, EndPrice: 90, High: 110, Low: 90},
	}
	v := BuildVirtualBi(klines, bis)
	if v == nil {
		t.Fatal("expected virtual bi, got nil")
	}
	if v.Direction != types.DirUp {
		t.Errorf("expected up virtual bi, got %d", v.Direction)
	}
	if v.EndPrice <= v.StartPrice {
		t.Errorf("virtual bi up must have end > start: %.2f <= %.2f", v.EndPrice, v.StartPrice)
	}
}

func TestBuildVirtualBi_NoTail(t *testing.T) {
	klines := []types.Kline{
		{Time: types.DateTime{Time: time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC)}, Open: 100, High: 105, Low: 95, Close: 102},
	}
	bis := []types.Bi{
		{StartIndex: 0, EndIndex: 0, Direction: types.DirUp, StartPrice: 95, EndPrice: 105, High: 105, Low: 95},
	}
	v := BuildVirtualBi(klines, bis)
	if v != nil {
		t.Errorf("expected nil for no tail, got virtual bi: %+v", v)
	}
}

func TestBuildVirtualBi_EmptyBis(t *testing.T) {
	v := BuildVirtualBi(nil, nil)
	if v != nil {
		t.Error("expected nil for empty input")
	}
}
