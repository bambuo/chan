package chanlun

import (
	"math"
	"time"
)

// ──────────────────────────────────────────────
// §1  Kline 数据模型
// ──────────────────────────────────────────────

// BOLLValue 表示布林带指标值。
type BOLLValue struct {
	Upper float64 `json:"upper"`
	Mid   float64 `json:"mid"`
	Lower float64 `json:"lower"`
}

// KDJValue 表示 KDJ 指标值。
type KDJValue struct {
	K float64 `json:"k"`
	D float64 `json:"d"`
	J float64 `json:"j"`
}

// Kline 表示算法层统一使用的 K 线数据结构。
type Kline struct {
	Time            time.Time `json:"time"`
	Open            float64   `json:"open"`
	High            float64   `json:"high"`
	Low             float64   `json:"low"`
	Close           float64   `json:"close"`
	BaseVolume      float64   `json:"baseVolume"`
	QuoteVolume     float64   `json:"quoteVolume,omitempty"`
	Turnover        float64   `json:"turnover,omitempty"`
	TradeCount      int64     `json:"tradeCount,omitempty"`
	RawVolumeUnit   string    `json:"rawVolumeUnit,omitempty"`
	RawTurnoverUnit string    `json:"rawTurnoverUnit,omitempty"`

	// 技术指标（可选，由 StreamEngine 按需计算）
	BOLL *BOLLValue      `json:"boll,omitempty"`
	RSI  *float64        `json:"rsi,omitempty"`
	KDJ  *KDJValue       `json:"kdj,omitempty"`
	MA   map[int]float64 `json:"ma,omitempty"`
}

// ──────────────────────────────────────────────
// §2  分型
// ──────────────────────────────────────────────

// FractalType 枚举分型的类型。
type FractalType int

const (
	FractalNone   FractalType = 0 // 非分型
	TopFractal    FractalType = 1 // 顶分型
	BottomFractal FractalType = 2 // 底分型
)

// Fractal 表示一个分型。
type Fractal struct {
	Type     FractalType `json:"type"`
	Index    int         `json:"index"`
	High     float64     `json:"high"`
	Low      float64     `json:"low"`
	Strength float64     `json:"strength"`
}

// FractalRange 返回分型区间。
func (f *Fractal) FractalRange(klines []Kline) (lower, upper float64) {
	if f.Index < 1 || f.Index+1 >= len(klines) {
		return 0, 0
	}
	prev := klines[f.Index-1]
	mid := klines[f.Index]
	next := klines[f.Index+1]
	switch f.Type {
	case TopFractal:
		lower = max(prev.Low, next.Low)
		upper = mid.High
	case BottomFractal:
		lower = mid.Low
		upper = min(prev.High, next.High)
	}
	return
}

// ──────────────────────────────────────────────
// §3  笔
// ──────────────────────────────────────────────

// Direction 表示方向。
type Direction int

const (
	DirDown Direction = -1 // 向下
	DirNone Direction = 0  // 无方向
	DirUp   Direction = 1  // 向上
)

// Bi 表示一笔。
type Bi struct {
	StartIndex int       `json:"startIndex"`
	EndIndex   int       `json:"endIndex"`
	Direction  Direction `json:"direction"`
	StartPrice float64   `json:"startPrice"`
	EndPrice   float64   `json:"endPrice"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	Length     float64   `json:"length"`
	Slope      float64   `json:"slope"`
	KLineCount int       `json:"klineCount"`
}

// MergedBi 表示经包含处理后的笔。
type MergedBi struct {
	Bi
	OriginalCount int   `json:"originalCount"`
	MergedFrom    []int `json:"mergedFrom,omitempty"`
}

// ──────────────────────────────────────────────
// §4  线段
// ──────────────────────────────────────────────

// FeatureElement 表示特征序列元素。
type FeatureElement struct {
	Bi       Bi      `json:"bi"`
	StartIdx int     `json:"startIdx"`
	EndIdx   int     `json:"endIdx"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
}

// BreakType 枚举线段的破坏方式。
type BreakType int

const (
	BreakNone   BreakType = 0 // 未破坏
	BreakStd    BreakType = 1 // 第一种破坏：特征序列无缺口标准破坏（必然伴随笔破坏）
	BreakStroke BreakType = 2 // 第二种破坏：特征序列有缺口（需二次特征序列分型确认）
)

// Segment 表示一个线段。
type Segment struct {
	StartIndex   int              `json:"startIndex"`
	EndIndex     int              `json:"endIndex"`
	Direction    Direction        `json:"direction"`
	BiList       []MergedBi       `json:"biList"`
	FeatureSeq   []FeatureElement `json:"featureSeq,omitempty"`
	Top          float64          `json:"top"`
	Bottom       float64          `json:"bottom"`
	IsBroken     bool             `json:"isBroken"`
	BreakType    BreakType        `json:"breakType"`
	ConfirmIndex int              `json:"confirmIndex"`
}

// ──────────────────────────────────────────────
// §5  中枢
// ──────────────────────────────────────────────

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

// Pivot 表示一个中枢。
//
// 字段说明（严格遵循缠论原文定义）：
//
//	ZG（中枢高点/中枢上沿）= min(前两个Z段的高点)，中枢形成后不再改变
//	ZD（中枢低点/中枢下沿）= max(前两个Z段的低点)，中枢形成后不再改变
//	GG（波动最高点）= max(中枢内所有线段的高点)
//	DD（波动最低点）= min(中枢内所有线段的低点)
//	中枢区间 = [ZD, ZG]；波动区间 = [DD, GG]
type Pivot struct {
	StartIndex   int        `json:"startIndex"`
	EndIndex     int        `json:"endIndex"`
	ZG           float64    `json:"zg"`                 // 中枢上沿（不变）
	ZD           float64    `json:"zd"`                 // 中枢下沿（不变）
	GG           float64    `json:"gg"`                 // 波动最高点（随延伸更新）
	DD           float64    `json:"dd"`                 // 波动最低点（随延伸更新）
	PeakHigh     float64    `json:"peakHigh,omitempty"` // 中枢内笔的波动最高价（来自 chan.py peak_high）
	PeakLow      float64    `json:"peakLow,omitempty"`  // 中枢内笔的波动最低价（来自 chan.py peak_low）
	Segments     []Segment  `json:"segments"`
	OverlapCount int        `json:"overlapCount"`
	Level        int        `json:"level"`
	SourceLevel  string     `json:"sourceLevel,omitempty"`
	IsProxy      bool       `json:"isEngineeringProxy"`
	State        PivotState `json:"state"`
	Direction    Direction  `json:"direction"` // 中枢方向（注意：以 Z 段方向为准）
	// 上涨中枢（第一段向下→向上→向下，Z段=第1、3段的向下段）→ DirDown
	// 下跌中枢（第一段向上→向下→向上，Z段=第1、3段的向上段）→ DirUp
	// 即 Direction 表示 Z 段的同向方向，而非价格趋势方向。
	// 上涨趋势中的回调中枢标记为 DirDown，下跌趋势中的回升中枢标记为 DirUp。
}

// ──────────────────────────────────────────────
// §6  走势类型
// ──────────────────────────────────────────────

// TrendType 枚举走势类型。
type TrendType int

const (
	TrendUp   TrendType = 1  // 上涨趋势
	TrendDown TrendType = -1 // 下跌趋势
	RangeOnly TrendType = 0  // 盘整
)

// Trend 表示一个走势类型。
type Trend struct {
	Type           TrendType `json:"type"`
	Pivots         []Pivot   `json:"pivots"`
	StartIndex     int       `json:"startIndex"`
	EndIndex       int       `json:"endIndex"`
	IsComplete     bool      `json:"isComplete"`
	CompleteReason string    `json:"completeReason,omitempty"`
}

// ──────────────────────────────────────────────
// §7  背驰
// ──────────────────────────────────────────────

// DeviationLevel 枚举背驰级别。
type DeviationLevel int

const (
	BiDeviation      DeviationLevel = 0 // 笔背驰
	SegmentDeviation DeviationLevel = 1 // 线段背驰
	TrendDeviation   DeviationLevel = 2 // 走势背驰
)

// Deviation 表示一个背驰信号。
type Deviation struct {
	Type           string         `json:"type"`
	Level          DeviationLevel `json:"level"`
	Direction      Direction      `json:"direction"`
	SegmentBefore  *Segment       `json:"-"`                   // Go API，保留
	SegmentAfter   *Segment       `json:"-"`                   // Go API，保留
	SegBeforeIdx   int            `json:"segBefore,omitempty"` // JSON 用：Segment 在 Result.Segments 中的索引
	SegAfterIdx    int            `json:"segAfter,omitempty"`  // JSON 用：Segment 在 Result.Segments 中的索引
	PriceHigh      float64        `json:"priceHigh"`
	ForceBefore    float64        `json:"forceBefore"`
	ForceAfter     float64        `json:"forceAfter"`
	MACDAreaBefore float64        `json:"macdAreaBefore"`
	MACDAreaAfter  float64        `json:"macdAreaAfter"`
	MACDDiffBefore float64        `json:"macdDiffBefore"`
	MACDDiffAfter  float64        `json:"macdDiffAfter"`
}

// IntervalNestingResult 包含区间套定位的完整结果。
type IntervalNestingResult struct {
	Levels        []string    `json:"levels"`
	Confirmations []Deviation `json:"confirmations"`
	FinalIndex    int         `json:"finalIndex"`
	FinalPrice    float64     `json:"finalPrice"`
	Accuracy      float64     `json:"accuracy"`
}

// ──────────────────────────────────────────────
// §8  买卖点
// ──────────────────────────────────────────────

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
	SubT1  SignalSubType = "1"  // 标准一买/一卖（趋势背驰）
	SubT1P SignalSubType = "1p" // 盘背一买/一卖（盘整背驰）
	SubT2  SignalSubType = "2"  // 标准二买/二卖
	SubT2S SignalSubType = "2s" // 类二买/类二卖
	SubT3A SignalSubType = "3a" // 三买a/三卖a（下一线段内）
	SubT3B SignalSubType = "3b" // 三买b/三卖b（当前线段末尾）
)

// Signal 表示一个买卖点信号。
type Signal struct {
	Type      SignalType             `json:"type"`
	SubType   SignalSubType          `json:"subType,omitempty"` // 子类型（1/1p/2/2s/3a/3b）
	Level     string                 `json:"level"`
	Index     int                    `json:"index"`
	Price     float64                `json:"price"`
	Strength  float64                `json:"strength"`
	Pivot     *Pivot                 `json:"pivot,omitempty"`
	Deviation *Deviation             `json:"deviation,omitempty"`
	Nesting   *IntervalNestingResult `json:"nesting,omitempty"`
}

// ──────────────────────────────────────────────
// §9  引擎结果
// ──────────────────────────────────────────────

// Result 包含一次完整处理的所有中间和最终输出。
type Result struct {
	MergedKlines []Kline     `json:"mergedKlines,omitempty"`
	Fractals     []Fractal   `json:"fractals,omitempty"`
	BiFractals   []Fractal   `json:"biFractals,omitempty"`
	Bis          []Bi        `json:"bis,omitempty"`
	MergedBis    []MergedBi  `json:"mergedBis,omitempty"`
	Segments     []Segment   `json:"segments,omitempty"`
	Pivots       []Pivot     `json:"pivots,omitempty"`
	Trends       []Trend     `json:"trends,omitempty"`
	Deviations   []Deviation `json:"deviations,omitempty"`
	Signals      []Signal    `json:"signals,omitempty"`
}

// ──────────────────────────────────────────────
// §12  配置
// ──────────────────────────────────────────────

// InclusionOption 控制 K 线包含处理的精细行为。
type InclusionOption struct {
	ExcludeIncluded bool // 被包含不合并，直接跳过
	AllowTopEqual   int  // 1=顶相等不合并, -1=底相等不合并, 0=标准模式
}

// Config 包含缠论算法的可配置参数。
type Config struct {
	BiMinKLineCount      int     `json:"biMinKlineCount"`
	MACDFastPeriod       int     `json:"macdFastPeriod"`
	MACDSlowPeriod       int     `json:"macdSlowPeriod"`
	MACDSignalPeriod     int     `json:"macdSignalPeriod"`
	DeviationForceMethod string  `json:"deviationForceMethod"`
	DIFReturnThreshold   float64 `json:"difReturnThreshold"`
	EnableBiInclusion    bool    `json:"enableBiInclusion"`
	EnableMultiLevel     bool    `json:"enableMultiLevel"`
	UpdateWindowSize     int     `json:"updateWindowSize"`
	NewBiMinPriceRatio   float64 `json:"newBiMinPriceRatio"`

	// ── 笔配置（来自 chan.py CBiConfig）──
	BiAlgo         string `json:"biAlgo"`         // "normal" | "fx"
	BiStrict       bool   `json:"biStrict"`       // 严格笔（span>=4）
	BiFxCheck      string `json:"biFxCheck"`      // "strict" | "loss" | "half" | "totally"
	GapAsKl        bool   `json:"gapAsKl"`        // 跳空当K线
	BiEndIsPeak    bool   `json:"biEndIsPeak"`    // 笔端点必须是极值
	BiAllowSubPeak bool   `json:"biAllowSubPeak"` // 允许次级别峰

	// ── 线段配置（来自 chan.py CSegConfig）──
	SegAlgo       string `json:"segAlgo"`       // "chan" | "dyh" | "def"
	LeftSegMethod string `json:"leftSegMethod"` // "peak" | "all"

	// ── 中枢配置（来自 chan.py CZSConfig）──
	ZsCombine     bool   `json:"zsCombine"`     // 是否合并中枢
	ZsCombineMode string `json:"zsCombineMode"` // "zs" | "peak"
	OneBiZs       bool   `json:"oneBiZs"`       // 允许单笔中枢
	ZsAlgo        string `json:"zsAlgo"`        // "normal" | "over_seg" | "auto"

	// ── 买卖点配置（来自 chan.py CBSPointConfig）──
	BspDivergenceRate float64 `json:"bspDivergenceRate"`
	BspMinZsCnt       int     `json:"bspMinZsCnt"`
	Bsp1OnlyMultiBiZs bool    `json:"bsp1OnlyMultiBiZs"`
	BspMaxBs2Rate     float64 `json:"bspMaxBs2Rate"`
	BspMacdAlgo       string  `json:"bspMacdAlgo"` // "area"|"peak"|"full_area"|"diff"|"slope"|"amp"|...
	Bsp1Peak          bool    `json:"bsp1Peak"`
	BspType           string  `json:"bspType"` // "1,1p,2,2s,3a,3b"
	Bsp2Follow1       bool    `json:"bsp2Follow1"`
	Bsp3Follow1       bool    `json:"bsp3Follow1"`
	Bsp3Peak          bool    `json:"bsp3Peak"`
	Bsp2sFollow2      bool    `json:"bsp2sFollow2"`
	StrictBsp3        bool    `json:"strictBsp3"`
	Bsp3aMaxZsCnt     int     `json:"bsp3aMaxZsCnt"`

	// ── 技术指标配置──
	CalBoll      bool  `json:"calBoll"`
	BollN        int   `json:"bollN"`
	CalRsi       bool  `json:"calRsi"`
	RsiCycle     int   `json:"rsiCycle"`
	CalKdj       bool  `json:"calKdj"`
	KdjCycle     int   `json:"kdjCycle"`
	MeanMetrics  []int `json:"meanMetrics"`
	TrendMetrics []int `json:"trendMetrics"`

	// ── 包含处理选项──
	Inclusion InclusionOption `json:"inclusion,omitempty"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{
		BiMinKLineCount:      1,          // 旧笔标准
		MACDFastPeriod:       12,         // MACD 默认快速周期
		MACDSlowPeriod:       26,         // MACD 默认慢速周期
		MACDSignalPeriod:     9,          // MACD 默认信号周期
		DeviationForceMethod: "combined", // 综合使用斜率和 MACD 面积
		DIFReturnThreshold:   0.15,       // |DIF| < ATR×0.15 视为回抽到位
		EnableBiInclusion:    true,
		EnableMultiLevel:     false,
		UpdateWindowSize:     300,   // 增量更新时重算最近 300 根
		NewBiMinPriceRatio:   0.003, // 新笔标准最小价格变动 0.3%

		// 笔配置默认值（对齐 chan.py）
		BiAlgo:         "normal",
		BiStrict:       true,
		BiFxCheck:      "half",
		GapAsKl:        false,
		BiEndIsPeak:    true,
		BiAllowSubPeak: true,

		// 线段配置默认值
		SegAlgo:       "chan",
		LeftSegMethod: "peak",

		// 中枢配置默认值
		ZsCombine:     true,
		ZsCombineMode: "zs",
		OneBiZs:       false,
		ZsAlgo:        "normal",

		// 买卖点配置默认值
		BspDivergenceRate: math.Inf(1),
		BspMinZsCnt:       1,
		Bsp1OnlyMultiBiZs: true,
		BspMaxBs2Rate:     0.9999,
		BspMacdAlgo:       "peak",
		Bsp1Peak:          true,
		BspType:           "1,1p,2,2s,3a,3b",
		Bsp2Follow1:       true,
		Bsp3Follow1:       true,
		Bsp3Peak:          false,
		Bsp2sFollow2:      false,
		StrictBsp3:        false,
		Bsp3aMaxZsCnt:     1,

		// 技术指标默认关闭
		CalBoll:  false,
		BollN:    20,
		CalRsi:   false,
		RsiCycle: 14,
		CalKdj:   false,
		KdjCycle: 9,
	}
}

// NewBiConfig 返回新笔标准（严笔）配置。
func NewBiConfig() Config {
	cfg := DefaultConfig()
	cfg.BiMinKLineCount = 5
	return cfg
}

// ──────────────────────────────────────────────
//  辅助函数
// ──────────────────────────────────────────────

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
