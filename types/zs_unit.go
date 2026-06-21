package types

// ──────────────────────────────────────────────
// ZSUnit — 中枢单元泛型接口
// ──────────────────────────────────────────────
//
// 对齐 chan.py CZS[LINE_TYPE] 泛型设计，封装笔和线段共有的接口方法。

// ZSUnit 表示可参与中枢构成的单元（笔或线段）。
type ZSUnit interface {
	Idx() int
	IsUp() bool
	IsDown() bool
	ZSHigh() float64
	ZSLow() float64
	BeginVal() float64
	EndVal() float64
	Amp() float64
}

// ── Bi 实现 ZSUnit ──

// Idx 返回笔的起始索引。
func (b Bi) Idx() int { return b.StartIndex }

// IsUp 笔是否向上。
func (b Bi) IsUp() bool { return b.Direction == DirUp }

// IsDown 笔是否向下。
func (b Bi) IsDown() bool { return b.Direction == DirDown }

// ZSHigh 返回笔的端点高点（对齐 chan.py _high）。
// 向上笔：终点（顶分型）的 high = EndPrice
// 向下笔：起点（顶分型）的 high = StartPrice
func (b Bi) ZSHigh() float64 {
	if b.IsUp() {
		return b.EndPrice
	}
	return b.StartPrice
}

// ZSLow 返回笔的端点低点（对齐 chan.py _low）。
// 向上笔：起点（底分型）的 low = StartPrice
// 向下笔：终点（底分型）的 low = EndPrice
func (b Bi) ZSLow() float64 {
	if b.IsUp() {
		return b.StartPrice
	}
	return b.EndPrice
}

// BeginVal 返回笔的起始价格。
func (b Bi) BeginVal() float64 { return b.StartPrice }

// EndVal 返回笔的结束价格。
func (b Bi) EndVal() float64 { return b.EndPrice }

// Amp 返回笔的振幅。
func (b Bi) Amp() float64 {
	if b.EndPrice > b.StartPrice {
		return b.EndPrice - b.StartPrice
	}
	return b.StartPrice - b.EndPrice
}

// ── MergedBi 通过内嵌 Bi 自动实现 ZSUnit ──

// ── Segment 实现 ZSUnit ──

// Idx 返回线段的起始索引。
func (s Segment) Idx() int { return s.StartIndex }

// IsUp 线段是否向上。
func (s Segment) IsUp() bool { return s.Direction == DirUp }

// IsDown 线段是否向下。
func (s Segment) IsDown() bool { return s.Direction == DirDown }

// ZSHigh 返回线段端点高点。
func (s Segment) ZSHigh() float64 { return s.Top }

// ZSLow 返回线段端点低点。
func (s Segment) ZSLow() float64 { return s.Bottom }

// BeginVal 线段起始价格。
func (s Segment) BeginVal() float64 {
	if s.IsUp() {
		return s.Bottom
	}
	return s.Top
}

// EndVal 线段结束价格。
func (s Segment) EndVal() float64 {
	if s.IsUp() {
		return s.Top
	}
	return s.Bottom
}

// Amp 线段振幅。
func (s Segment) Amp() float64 {
	if s.Top > s.Bottom {
		return s.Top - s.Bottom
	}
	return s.Bottom - s.Top
}

// ──────────────────────────────────────────────
// 通用中枢计算函数（基于 ZSUnit）
// ──────────────────────────────────────────────

// zsHasOverlap 判断两个区间 [l1,h1] 和 [l2,h2] 是否重叠。
// equal=true: h2 >= l1 and h1 >= l2（端点相切也算重叠）
// equal=false: h2 > l1 and h1 > l2（严格大于）
func zsHasOverlap(l1, h1, l2, h2 float64, equal bool) bool {
	if equal {
		return h2 >= l1 && h1 >= l2
	}
	return h2 > l1 && h1 > l2
}

// HasOverlap 公开版 zsHasOverlap，供外部包使用。
func HasOverlap(l1, h1, l2, h2 float64, equal bool) bool {
	return zsHasOverlap(l1, h1, l2, h2, equal)
}

// CalcZSRangeFromUnits 从构成中枢的单元计算中枢区间（求交集）。
// ZG = min(all ZSHigh), ZD = max(all ZSLow)
func CalcZSRangeFromUnits(units []ZSUnit) (zg, zd float64) {
	if len(units) == 0 {
		return 0, 0
	}
	zg = units[0].ZSHigh()
	zd = units[0].ZSLow()
	for _, u := range units[1:] {
		if u.ZSHigh() < zg {
			zg = u.ZSHigh()
		}
		if u.ZSLow() > zd {
			zd = u.ZSLow()
		}
	}
	return
}

// CalcPeakFromUnits 计算单元列表的波动极值（求并集）。
func CalcPeakFromUnits(units []ZSUnit) (peakHigh, peakLow float64) {
	if len(units) == 0 {
		return 0, 0
	}
	peakHigh = units[0].ZSHigh()
	peakLow = units[0].ZSLow()
	for _, u := range units[1:] {
		if u.ZSHigh() > peakHigh {
			peakHigh = u.ZSHigh()
		}
		if u.ZSLow() < peakLow {
			peakLow = u.ZSLow()
		}
	}
	return
}

// EndFractalType 返回笔的终点分型类型。
func (bi Bi) EndFractalType() FractalType {
	if bi.IsUp() {
		return TopFractal
	}
	return BottomFractal
}

// ToEndFractal 从笔构造其终点分型。
func (bi Bi) ToEndFractal() Fractal {
	return Fractal{Index: bi.EndIndex, Type: bi.EndFractalType(), High: bi.EndPrice, Low: bi.EndPrice}
}

// IsOneBiZs 判断是否单笔中枢。
func (p Pivot) IsOneBiZs() bool {
	return p.BeginBiIdx == p.EndBiIdx
}
