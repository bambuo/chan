package types

// ──────────────────────────────────────────────
// Kline 数据模型
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
	Time            DateTime `json:"time" csv:"time"`
	Open            float64  `json:"open" csv:"open"`
	High            float64  `json:"high" csv:"high"`
	Low             float64  `json:"low" csv:"low"`
	Close           float64  `json:"close" csv:"close"`
	BaseVolume      float64  `json:"baseVolume" csv:"baseVolume"`
	QuoteVolume     float64  `json:"quoteVolume,omitempty"`
	Turnover        float64  `json:"turnover,omitempty"`
	TradeCount      int64    `json:"tradeCount,omitempty"`
	RawVolumeUnit   string   `json:"rawVolumeUnit,omitempty"`
	RawTurnoverUnit string   `json:"rawTurnoverUnit,omitempty"`

	// 技术指标（可选）
	BOLL *BOLLValue      `json:"boll,omitempty"`
	RSI  *float64        `json:"rsi,omitempty"`
	KDJ  *KDJValue       `json:"kdj,omitempty"`
	MA   map[int]float64 `json:"ma,omitempty"`
}

// ──────────────────────────────────────────────
// 分型
// ──────────────────────────────────────────────

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
// 笔
// ──────────────────────────────────────────────

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
// 线段
// ──────────────────────────────────────────────

// FeatureElement 表示特征序列元素。
type FeatureElement struct {
	Bi       Bi      `json:"bi"`
	StartIdx int     `json:"startIdx"`
	EndIdx   int     `json:"endIdx"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
}

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
	IsSure       bool             `json:"isSure"`       // 对齐 Python CSeg.is_sure
	UsedToBeSure bool             `json:"usedToBeSure"` // 曾为确定线段
}

// ──────────────────────────────────────────────
// 中枢
// ──────────────────────────────────────────────

// Pivot 表示一个中枢。
type Pivot struct {
	StartIndex   int        `json:"startIndex"`
	EndIndex     int        `json:"endIndex"`
	ZG           float64    `json:"zg"`                 // 中枢上沿（不变）
	ZD           float64    `json:"zd"`                 // 中枢下沿（不变）
	GG           float64    `json:"gg"`                 // 波动最高点（随延伸更新）
	DD           float64    `json:"dd"`                 // 波动最低点（随延伸更新）
	PeakHigh     float64    `json:"peakHigh,omitempty"` // 中枢内笔的波动最高价
	PeakLow      float64    `json:"peakLow,omitempty"`  // 中枢内笔的波动最低价
	Segments     []Segment  `json:"segments"`
	OverlapCount int        `json:"overlapCount"`
	Level        int        `json:"level"`
	SourceLevel  string     `json:"sourceLevel,omitempty"` // "bi"=笔级中枢，"segment"=旧线段级中枢
	IsProxy      bool       `json:"isEngineeringProxy"`
	State        PivotState `json:"state"`
	Direction    Direction  `json:"direction"` // 中枢方向（以 Z 段方向为准）

	// ── 笔级中枢字段 ──
	BeginBiIdx int     `json:"beginBiIdx,omitempty"` // 中枢起始笔索引
	EndBiIdx   int     `json:"endBiIdx,omitempty"`   // 中枢结束笔索引
	BiIn       *Bi     `json:"biIn,omitempty"`       // 进入中枢的笔
	BiOut      *Bi     `json:"biOut,omitempty"`      // 离开中枢的笔
	BiList     []Bi    `json:"biList,omitempty"`     // 中枢内所有笔
	SubPivots  []Pivot `json:"subPivots,omitempty"`  // 合并前的原始子中枢
	IsSure     bool    `json:"isSure,omitempty"`     // 中枢是否确定
}

// ──────────────────────────────────────────────
// 走势类型
// ──────────────────────────────────────────────

// Trend 表示一个走势类型。
type Trend struct {
	Type           TrendType       `json:"type"`
	Pivots         []Pivot         `json:"pivots"`
	StartIndex     int             `json:"startIndex"`
	EndIndex       int             `json:"endIndex"`
	IsComplete     bool            `json:"isComplete"`
	CompleteReason string          `json:"completeReason,omitempty"`
	Metrics        map[int]float64 `json:"metrics,omitempty"` // TrendMetrics 计算结果 key=周期 value=值
}

// ──────────────────────────────────────────────
// 背驰
// ──────────────────────────────────────────────

// Deviation 表示一个背驰信号。
type Deviation struct {
	Type           string         `json:"type"`
	Level          DeviationLevel `json:"level"`
	Direction      Direction      `json:"direction"`
	SegmentBefore  *Segment       `json:"-"`
	SegmentAfter   *Segment       `json:"-"`
	SegBeforeIdx   int            `json:"segBefore,omitempty"`
	SegAfterIdx    int            `json:"segAfter,omitempty"`
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
// 买卖点
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Result 查询方法
// ──────────────────────────────────────────────

// PivotOfSegment 查找包含指定线段的第一个中枢。
func (r *Result) PivotOfSegment(segIdx int) *Pivot {
	if segIdx < 0 || segIdx >= len(r.Segments) {
		return nil
	}
	seg := &r.Segments[segIdx]
	for i := range r.Pivots {
		p := &r.Pivots[i]
		if p.StartIndex <= seg.StartIndex && p.EndIndex >= seg.EndIndex {
			return p
		}
	}
	return nil
}

// SegmentsOfPivot 返回中枢包含的线段列表。
func (r *Result) SegmentsOfPivot(pivotIdx int) []Segment {
	if pivotIdx < 0 || pivotIdx >= len(r.Pivots) {
		return nil
	}
	return r.Pivots[pivotIdx].Segments
}

// BiOfSegment 返回线段包含的笔列表。
func (r *Result) BiOfSegment(segIdx int) []Bi {
	if segIdx < 0 || segIdx >= len(r.Segments) {
		return nil
	}
	bis := make([]Bi, 0)
	for _, mb := range r.Segments[segIdx].BiList {
		bis = append(bis, mb.Bi)
	}
	return bis
}

// SignalAt 返回指定索引位置的买卖点信号。
func (r *Result) SignalAt(index int) *Signal {
	for i := range r.Signals {
		if r.Signals[i].Index == index {
			return &r.Signals[i]
		}
	}
	return nil
}

// LatestPivot 返回最近（最后一个）中枢。
func (r *Result) LatestPivot() *Pivot {
	if len(r.Pivots) == 0 {
		return nil
	}
	return &r.Pivots[len(r.Pivots)-1]
}

// LatestBi 返回最近（最后一）笔。
func (r *Result) LatestBi() *Bi {
	if len(r.Bis) == 0 {
		return nil
	}
	return &r.Bis[len(r.Bis)-1]
}

// LatestSegment 返回最近（最后一个）线段。
func (r *Result) LatestSegment() *Segment {
	if len(r.Segments) == 0 {
		return nil
	}
	return &r.Segments[len(r.Segments)-1]
}

// LatestTrend 返回最近（最后一个）走势。
func (r *Result) LatestTrend() *Trend {
	if len(r.Trends) == 0 {
		return nil
	}
	return &r.Trends[len(r.Trends)-1]
}

// BiPivotCount 返回笔级中枢数量。
func (r *Result) BiPivotCount() int {
	cnt := 0
	for _, p := range r.Pivots {
		if p.SourceLevel == "bi" {
			cnt++
		}
	}
	return cnt
}

// MultiBiZsCount 返回多笔中枢数量（非单笔中枢）。
func (r *Result) MultiBiZsCount() int {
	cnt := 0
	for _, p := range r.Pivots {
		if !p.IsOneBiZs() {
			cnt++
		}
	}
	return cnt
}

// ──────────────────────────────────────────────
// Signal 表示一个买卖点信号。
type Signal struct {
	Type             SignalType             `json:"type"`
	SubType          SignalSubType          `json:"subType,omitempty"`
	Level            string                 `json:"level"`
	Index            int                    `json:"index"`
	Price            float64                `json:"price"`
	Strength         float64                `json:"strength"`
	Pivot            *Pivot                 `json:"pivot,omitempty"`
	Deviation        *Deviation             `json:"deviation,omitempty"`
	Nesting          *IntervalNestingResult `json:"nesting,omitempty"`
	RelatedBSP1Index int                    `json:"relatedBsp1Index,omitempty"`
	Features         map[string]float64     `json:"features,omitempty"` // ML 特征向量
}

// ──────────────────────────────────────────────
// 引擎结果
// ──────────────────────────────────────────────

// DemarkSignal 表示一个 Demark 九转序列信号。
type DemarkSignal struct {
	Type  string  `json:"type"` // "setup" / "countdown"
	Dir   int     `json:"dir"`  // 1=up, -1=down
	Idx   int     `json:"idx"`  // 序号（1-based）
	Price float64 `json:"price"`
	KLIdx int     `json:"klIdx"` // 对应的 K 线索引
}

// Result 包含一次完整处理的所有中间和最终输出。
type Result struct {
	MergedKlines  []Kline        `json:"mergedKlines,omitempty"`
	Fractals      []Fractal      `json:"fractals,omitempty"`
	BiFractals    []Fractal      `json:"biFractals,omitempty"`
	Bis           []Bi           `json:"bis,omitempty"`
	MergedBis     []MergedBi     `json:"mergedBis,omitempty"`
	Segments      []Segment      `json:"segments,omitempty"`
	Pivots        []Pivot        `json:"pivots,omitempty"`
	Trends        []Trend        `json:"trends,omitempty"`
	Deviations    []Deviation    `json:"deviations,omitempty"`
	Signals       []Signal       `json:"signals,omitempty"`
	SegSignals    []Signal       `json:"segSignals,omitempty"`
	DemarkSignals []DemarkSignal `json:"demarkSignals,omitempty"`
	VirtualBi     *Bi            `json:"virtualBi,omitempty"`

	// 调试信息
	FractalWarnings []string `json:"fractalWarnings,omitempty"`
}
