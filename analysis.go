package chanlun

import (
	"github.com/bambuo/chan/bi"
	"github.com/bambuo/chan/deviation"
	"github.com/bambuo/chan/fractal"
	"github.com/bambuo/chan/kline"
	"github.com/bambuo/chan/multi"
	"github.com/bambuo/chan/pivot"
	"github.com/bambuo/chan/scoring"
	"github.com/bambuo/chan/segment"
	"github.com/bambuo/chan/signal"
	"github.com/bambuo/chan/trend"
	"github.com/bambuo/chan/types"
	"github.com/bambuo/talib"
)

// KlineView 封装 K 线序列，携带包含处理行为。
type KlineView struct{ Items []types.Kline }

// Merge 对 K 线进行包含处理。
func (v *KlineView) Merge(opts ...types.InclusionOption) *KlineView {
	var opt types.InclusionOption
	if len(opts) > 0 {
		opt = opts[0]
	}
	v.Items = kline.MergeKlines(v.Items, opt)
	return v
}

// FractalView 封装分型序列。
type FractalView struct {
	Items  []types.Fractal
	Klines []types.Kline
}

// Detect 识别分型。
func (v *FractalView) Detect(klines []types.Kline) *FractalView {
	v.Items = fractal.FindFractals(klines)
	v.Klines = klines
	return v
}

// FilterForBi 筛选成笔分型。
func (v *FractalView) FilterForBi(gap int) *FractalView {
	v.Items = fractal.FilterForBi(v.Items, gap)
	return v
}

// BiView 封装笔序列。
type BiView struct {
	Items     []types.Bi
	Merged    []types.MergedBi
	Klines    []types.Kline
	VirtualBi *types.Bi
}

// Build 从分型构建笔。
func (v *BiView) Build(klines []types.Kline, fractals []types.Fractal, cfg types.Config) *BiView {
	v.Items = bi.BuildBis(klines, fractals, cfg)
	v.Klines = klines
	if cfg.EnableBiInclusion && len(v.Items) >= 2 {
		v.Merged = bi.MergeBis(v.Items)
	} else {
		v.Merged = make([]types.MergedBi, len(v.Items))
		for i, b := range v.Items {
			v.Merged[i] = types.MergedBi{Bi: b, OriginalCount: 1}
		}
	}
	// 构建虚笔（不修改现有笔列表）
	v.VirtualBi = bi.BuildVirtualBi(klines, v.Items)
	return v
}

// SegmentView 封装线段序列。
type SegmentView struct{ Items []types.Segment }

// Build 构建线段。
func (v *SegmentView) Build(bis []types.MergedBi, algo, left string) *SegmentView {
	v.Items = segment.BuildWithConfig(bis, algo, left)
	return v
}

// PivotView 封装中枢序列。
type PivotView struct{ Items []types.Pivot }

// Detect 检测中枢。
func (v *PivotView) Detect(bis []types.MergedBi, segs []types.Segment, cfg types.Config) *PivotView {
	v.Items = pivot.FindBiPivots(bis, segs, cfg)
	pivot.UpdateZSInSeg(bis, v.Items)
	return v
}

// TrendView 封装走势序列。
type TrendView struct{ Items []types.Trend }

// Classify 分类走势。
func (v *TrendView) Classify(pivots []types.Pivot) *TrendView {
	v.Items = trend.ClassifyTrends(pivots)
	return v
}

// UpdateWithDeviations 用背驰更新走势完成状态。
func (v *TrendView) UpdateWithDeviations(devs []types.Deviation) *TrendView {
	trend.UpdateTrendsWithDeviations(v.Items, devs)
	return v
}

// Analysis 缠论分析入口，支持链式调用。
type Analysis struct {
	Config     types.Config
	Klines     KlineView
	Fractals   FractalView
	Bis        BiView
	Segments   SegmentView
	Pivots     PivotView
	Trends     TrendView
	Deviations []types.Deviation
	Signals    []types.Signal
	SegSignals *signal.SegSignalsCtx

	macdHist      []float64
	macdDif       []float64
	volumes       []float64
	turnovers     []float64
	closes        []float64
	demarkEngine  *kline.DemarkEngine
	DemarkSignals []types.DemarkSignal

	// 调试信息
	FractalWarnings []string

	// 逐 K 线增量模式 buffer
	buffer []types.Kline

	// 多级别链分析结果（由 BuildMultiLevelChain 设置）
	multiLevelResult *multi.MultiLevelResult
}

// NewAnalysis 创建分析实例。
func NewAnalysis(klines []types.Kline, cfg types.Config) *Analysis {
	return &Analysis{Config: cfg, Klines: KlineView{Items: klines}, buffer: nil}
}

// MergeKlines 包含处理。
func (a *Analysis) MergeKlines() *Analysis { a.Klines.Merge(a.Config.Inclusion); return a }

// CalcIndicators 计算技术指标（BOLL/RSI/KDJ/MA）。
// 在包含处理之后、分型识别之前调用，确保指标基于合并后的 K 线计算。
func (a *Analysis) CalcIndicators() *Analysis {
	if a.Config.CalBoll || a.Config.CalRsi || a.Config.CalKdj || len(a.Config.MeanMetrics) > 0 {
		kline.CalcIndicators(a.Klines.Items, a.Config)
	}
	return a
}

// DetectFractals 识别分型。
func (a *Analysis) DetectFractals() *Analysis {
	f := fractal.FindFractals(a.Klines.Items)
	bf := fractal.FilterForBi(f, a.Config.BiMinKLineCount)
	a.Fractals = FractalView{Items: bf, Klines: a.Klines.Items}

	// 调试：分型冲突检测
	if a.Config.DebugFractalCheck {
		a.FractalWarnings = fractal.ValidateFractals(f, bf)
	}
	return a
}

// BuildBis 构建笔。
func (a *Analysis) BuildBis() *Analysis {
	a.Bis.Build(a.Fractals.Klines, a.Fractals.Items, a.Config)
	return a
}

// BuildSegments 构建线段。
func (a *Analysis) BuildSegments() *Analysis {
	a.Segments.Build(a.Bis.Merged, a.Config.SegAlgo, a.Config.LeftSegMethod)
	return a
}

// DetectPivots 检测中枢。
func (a *Analysis) DetectPivots() *Analysis {
	a.Pivots.Detect(a.Bis.Merged, a.Segments.Items, a.Config)
	return a
}

// ClassifyTrends 走势分类。
func (a *Analysis) ClassifyTrends() *Analysis {
	a.Trends.Classify(a.Pivots.Items)
	if len(a.Config.TrendMetrics) > 0 {
		trend.CalcTrendMetrics(a.Trends.Items, a.Klines.Items, a.Config.TrendMetrics)
	}
	return a
}

// CalcDemark 计算 Demark 九转序列指标。
func (a *Analysis) CalcDemark() *Analysis {
	if !a.Config.CalDemark {
		return a
	}
	if len(a.closes) == 0 {
		a.closes = closesOf(a.Klines.Items)
	}
	a.demarkEngine = kline.DefaultDemarkEngine()
	for i, k := range a.Klines.Items {
		sigs := a.demarkEngine.Update(i, k.Close, k.High, k.Low)
		for _, s := range sigs {
			a.DemarkSignals = append(a.DemarkSignals, types.DemarkSignal{
				Type:  s.Type,
				Dir:   s.Dir,
				Idx:   s.Idx,
				Price: s.Price,
				KLIdx: i,
			})
		}
	}
	return a
}

// DetectDeviations 背驰检测。
func (a *Analysis) DetectDeviations() *Analysis {
	if len(a.closes) == 0 {
		a.closes = closesOf(a.Klines.Items)
	}
	if len(a.closes) <= a.Config.MACDSlowPeriod {
		return a
	}
	r, err := talib.MACD(a.closes, a.Config.MACDFastPeriod, a.Config.MACDSlowPeriod, a.Config.MACDSignalPeriod)
	if err != nil || r == nil {
		return a
	}
	a.macdHist = r.Histogram
	a.macdDif = r.MACD
	a.volumes = volumesOf(a.Klines.Items)
	a.turnovers = turnoversOf(a.Klines.Items)

	a.Deviations = deviation.DetectDeviations(a.Pivots.Items, a.macdHist, a.macdDif,
		a.volumes, a.turnovers, a.closes, a.Config)
	td := deviation.DetectTrendDeviations(a.Pivots.Items, a.Trends.Items,
		a.macdHist, a.macdDif, a.volumes, a.turnovers, a.closes, a.Config)
	a.Deviations = append(a.Deviations, td...)
	a.Trends.UpdateWithDeviations(td)
	return a
}

// DetectSignals 买卖点。
func (a *Analysis) DetectSignals() *Analysis {
	a.Signals = signal.DetectSignals(a.Pivots.Items, a.Bis.Merged, a.Segments.Items, a.Deviations, a.Config, a.macdHist)
	return a
}

// DetectSegSignals 线段级买卖点。
// 将线段视为高级别的笔，重新走信号检测流程。
// 仅当 EnableSegBSP 为 true 时执行。
func (a *Analysis) DetectSegSignals() *Analysis {
	if a.Config.EnableSegBSP {
		a.SegSignals = signal.DetectSegSignals(a.Segments.Items, a.Config)
		if a.SegSignals != nil && len(a.macdHist) > 0 && len(a.SegSignals.Pivots) > 0 {
			segDevs := deviation.DetectSegmentDeviations(
				a.SegSignals.Pivots, a.SegSignals.Merged,
				a.macdHist, a.macdDif, a.volumes, a.turnovers, a.closes, a.Config)
			// 存储线段级背驰到 ctx 并重新检测（使偏差参与信号判定）
			a.SegSignals.Deviations = segDevs
			segCfg := signal.SegBspConfig(a.Config)
			a.SegSignals.Signals = signal.DetectSignals(
				a.SegSignals.Pivots, a.SegSignals.Merged,
				a.SegSignals.Segments, segDevs, segCfg, a.macdHist)
		}
	}
	return a
}

// ScoreSignals 信号评分。
func (a *Analysis) ScoreSignals() *Analysis {
	liq := liquidityOf(a.Klines.Items)
	weights := a.Config.ScoreWeights
	levelCount := 1
	if a.multiLevelResult != nil {
		levelCount = a.multiLevelResult.LevelCount()
	}
	for i := range a.Signals {
		s, _ := scoring.ScoreSignal(&scoring.ScoringContext{
			Signal: a.Signals[i], Deviations: a.Deviations, Pivots: a.Pivots.Items,
			MultiLevelCount: levelCount, LiquidityData: liq, ClosePrices: a.closes,
			Weights: &weights,
		})
		a.Signals[i].Strength = s
	}
	return a
}

// Run 执行完整分析链。
func (a *Analysis) Run() *types.Result {
	return a.MergeKlines().CalcIndicators().DetectFractals().BuildBis().BuildSegments().
		DetectPivots().ClassifyTrends().CalcDemark().DetectDeviations().
		DetectSignals().ScoreSignals().DetectSegSignals().Result()
}

// BuildMultiLevelChain 从最低级别到最高级别递归构建多级别缠论结构。
// levels 必须按时间周期从低到高排列，如 [{Interval:"1m", Klines:...}, {Interval:"5m", ...}]。
// 最低级别使用当前 Analysis 的 K 线数据，高级别使用传入的 levels 依次递归。
func (a *Analysis) BuildMultiLevelChain(levels []multi.LevelInput) *multi.MultiLevelResult {
	result := multi.BuildMultiLevelChain(levels, a.Config)
	a.multiLevelResult = result
	return result
}

// Step 逐根推送 K 线，执行增量分析。
// 首次调用会自动用首根 K 线初始化 buffer。
// 之后每调用一次追加一根 K 线并重新运行完整管道。
// 返回当前所有 K 线的完整分析结果。
// 注意：该方法执行全量重算而非增量维护，适合每来一根 K 线就重新分析的场景。
func (a *Analysis) Step(kline types.Kline) *types.Result {
	if a.buffer == nil {
		a.buffer = []types.Kline{kline}
	} else {
		a.buffer = append(a.buffer, kline)
	}
	// 每次重新创建 Analysis 实例，确保从头到尾全量计算
	step := NewAnalysis(a.buffer, a.Config)
	return step.Run()
}

// Result 返回结果。
func (a *Analysis) Result() *types.Result {
	return &types.Result{
		MergedKlines: a.Klines.Items, Fractals: a.Fractals.Items,
		Bis: a.Bis.Items, MergedBis: a.Bis.Merged, Segments: a.Segments.Items,
		Pivots: a.Pivots.Items, Trends: a.Trends.Items,
		Deviations: a.Deviations, Signals: a.Signals,
		DemarkSignals:   a.DemarkSignals,
		VirtualBi:       a.Bis.VirtualBi,
		FractalWarnings: a.FractalWarnings,
		SegSignals:      a.segSignals(),
	}
}

func (a *Analysis) segSignals() []types.Signal {
	if a.SegSignals != nil {
		return a.SegSignals.Signals
	}
	return nil
}

func closesOf(k []types.Kline) []float64 {
	p := make([]float64, len(k))
	for i, c := range k {
		p[i] = c.Close
	}
	return p
}

func volumesOf(k []types.Kline) []float64 {
	v := make([]float64, len(k))
	for i, c := range k {
		v[i] = c.BaseVolume
	}
	return v
}

func turnoversOf(k []types.Kline) []float64 {
	t := make([]float64, len(k))
	for i, c := range k {
		switch {
		case c.Turnover > 0:
			t[i] = c.Turnover
		case c.QuoteVolume > 0:
			t[i] = c.QuoteVolume
		default:
			t[i] = c.BaseVolume
		}
	}
	return t
}

func liquidityOf(k []types.Kline) []float64 {
	v := make([]float64, len(k))
	for i, c := range k {
		switch {
		case c.Turnover > 0:
			v[i] = c.Turnover
		case c.QuoteVolume > 0:
			v[i] = c.QuoteVolume
		default:
			v[i] = c.BaseVolume
		}
	}
	return v
}
