package types

// ──────────────────────────────────────────────
// 枚举类型定义
// ──────────────────────────────────────────────

// Direction 表示方向。
type Direction int

const (
	DirDown Direction = -1 // 向下
	DirNone Direction = 0  // 无方向
	DirUp   Direction = 1  // 向上
)

// FractalType 枚举分型的类型。
type FractalType int

const (
	FractalNone   FractalType = 0 // 非分型
	TopFractal    FractalType = 1 // 顶分型
	BottomFractal FractalType = 2 // 底分型
)

// BreakType 枚举线段的破坏方式。
type BreakType int

const (
	BreakNone   BreakType = 0 // 未破坏
	BreakStd    BreakType = 1 // 第一种破坏：特征序列无缺口标准破坏（必然伴随笔破坏）
	BreakStroke BreakType = 2 // 第二种破坏：特征序列有缺口（需二次特征序列分型确认）
)

// PivotState 枚举中枢的生命周期状态。
type PivotState int

const (
	PivotForming   PivotState = 0 // 形成中
	PivotFormed    PivotState = 1 // 已形成
	PivotExtending PivotState = 2 // 延伸中
	PivotExpanded  PivotState = 3 // 扩展升级
	PivotEnlarged  PivotState = 4 // 扩张升级
	PivotDestroyed PivotState = 5 // 被破坏
)

// TrendType 枚举走势类型。
type TrendType int

const (
	TrendUp   TrendType = 1  // 上涨趋势
	TrendDown TrendType = -1 // 下跌趋势
	RangeOnly TrendType = 0  // 盘整
)

// DeviationLevel 枚举背驰级别。
type DeviationLevel int

const (
	BiDeviation      DeviationLevel = 0 // 笔背驰
	SegmentDeviation DeviationLevel = 1 // 线段背驰
	TrendDeviation   DeviationLevel = 2 // 走势背驰
)

// SignalType 枚举买卖点类型。
type SignalType int

const (
	BuyPoint1  SignalType = 1  // 第一类买点
	BuyPoint2  SignalType = 2  // 第二类买点
	BuyPoint3  SignalType = 3  // 第三类买点
	SellPoint1 SignalType = -1 // 第一类卖点
	SellPoint2 SignalType = -2 // 第二类卖点
	SellPoint3 SignalType = -3 // 第三类卖点
)

// SignalSubType 买卖点子类型（对齐 chan.py BSP_TYPE）。
type SignalSubType string

const (
	SubT1       SignalSubType = "1"       // 标准一买/一卖（趋势背驰）
	SubT1P      SignalSubType = "1p"      // 盘背一买/一卖（盘整背驰）
	SubT2       SignalSubType = "2"       // 标准二买/二卖
	SubT2S      SignalSubType = "2s"      // 类二买/类二卖
	SubT3A      SignalSubType = "3a"      // 三买a/三卖a（下一线段内）
	SubT3B      SignalSubType = "3b"      // 三买b/三卖b（当前线段末尾）
	SubTSupport SignalSubType = "support" // 支撑（回踩中枢下沿ZD附近企稳）
	SubTResist  SignalSubType = "resist"  // 压力（反弹中枢上沿ZG附近受阻）
	SubTBreakUp SignalSubType = "breakUp" // 突破（上穿中枢上沿ZG）
	SubTBreakDn SignalSubType = "breakDn" // 跌破（下穿中枢下沿ZD）
)
