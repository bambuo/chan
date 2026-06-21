package types

import "math"

// InclusionOption 控制 K 线包含处理的精细行为。
type InclusionOption struct {
	ExcludeIncluded bool // 被包含不合并，直接跳过
	AllowTopEqual   int  // 1=顶相等不合并, -1=底相等不合并, 0=标准模式
}

// Config 包含缠论算法的可配置参数。
type Config struct {
	BiMinKLineCount   int  `json:"biMinKlineCount"`
	MACDFastPeriod    int  `json:"macdFastPeriod"`
	MACDSlowPeriod    int  `json:"macdSlowPeriod"`
	MACDSignalPeriod  int  `json:"macdSignalPeriod"`
	EnableBiInclusion bool `json:"enableBiInclusion"`
	EnableMultiLevel  bool `json:"enableMultiLevel"`
	UpdateWindowSize  int  `json:"updateWindowSize"`

	// ── 笔配置 ──
	BiAlgo         string `json:"biAlgo"`
	BiStrict       bool   `json:"biStrict"`
	BiFxCheck      string `json:"biFxCheck"`
	GapAsKl        bool   `json:"gapAsKl"`
	BiEndIsPeak    bool   `json:"biEndIsPeak"`
	BiAllowSubPeak bool   `json:"biAllowSubPeak"`

	// ── 线段配置 ──
	SegAlgo       string `json:"segAlgo"`
	LeftSegMethod string `json:"leftSegMethod"`

	// ── 中枢配置 ──
	ZsCombine     bool   `json:"zsCombine"`
	ZsCombineMode string `json:"zsCombineMode"`
	OneBiZs       bool   `json:"oneBiZs"`
	ZsAlgo        string `json:"zsAlgo"`

	// ── 买卖点配置 ──
	BspDivergenceRate float64 `json:"bspDivergenceRate"`
	BspMinZsCnt       int     `json:"bspMinZsCnt"`
	Bsp1OnlyMultiBiZs bool    `json:"bsp1OnlyMultiBiZs"`
	BspMaxBs2Rate     float64 `json:"bspMaxBs2Rate"`
	BspMacdAlgo       string  `json:"bspMacdAlgo"`
	BspMacdAlgoBuy    string  `json:"bspMacdAlgoBuy,omitempty"`  // 买点独立 MACD 算法
	BspMacdAlgoSell   string  `json:"bspMacdAlgoSell,omitempty"` // 卖点独立 MACD 算法
	Bsp1Peak          bool    `json:"bsp1Peak"`
	BspType           string  `json:"bspType"`
	Bsp2Follow1       bool    `json:"bsp2Follow1"`
	Bsp3Follow1       bool    `json:"bsp3Follow1"`
	Bsp3Peak          bool    `json:"bsp3Peak"`
	Bsp2sFollow2      bool    `json:"bsp2sFollow2"`
	StrictBsp3        bool    `json:"strictBsp3"`
	Bsp3aMaxZsCnt     int     `json:"bsp3aMaxZsCnt"`
	BspMaxBs2sLv      *int    `json:"bspMaxBs2sLv,omitempty"`

	// ── 技术指标配置 ──
	CalBoll      bool  `json:"calBoll"`
	BollN        int   `json:"bollN"`
	CalRsi       bool  `json:"calRsi"`
	RsiCycle     int   `json:"rsiCycle"`
	CalKdj       bool  `json:"calKdj"`
	KdjCycle     int   `json:"kdjCycle"`
	CalDemark    bool  `json:"calDemark"`
	MeanMetrics  []int `json:"meanMetrics"`
	TrendMetrics []int `json:"trendMetrics"`

	// ── 包含处理选项 ──
	Inclusion InclusionOption `json:"inclusion,omitempty"`

	// ── 线段级买卖点 ──
	EnableSegBSP         bool    `json:"enableSegBsp"`
	SegBspMacdAlgo       string  `json:"segBspMacdAlgo"`
	SegBspDivergenceRate float64 `json:"segBspDivergenceRate"`
	SegBsp1OnlyMultiBiZs bool    `json:"segBsp1OnlyMultiBiZs"`
	SegBspType           string  `json:"segBspType"`

	// ── 调试选项 ──
	DebugFractalCheck bool `json:"debugFractalCheck"` // 开启分型冲突检测日志
	TriggerStep       bool `json:"triggerStep"`       // 启用逐 K 线增量模式
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{
		BiMinKLineCount:   1,
		MACDFastPeriod:    12,
		MACDSlowPeriod:    26,
		MACDSignalPeriod:  9,
		EnableBiInclusion: true,
		EnableMultiLevel:  false,
		UpdateWindowSize:  300,

		BiAlgo:         "normal",
		BiStrict:       true,
		BiFxCheck:      "strict",
		GapAsKl:        true,
		BiEndIsPeak:    true,
		BiAllowSubPeak: true,

		SegAlgo:       "chan",
		LeftSegMethod: "peak",

		ZsCombine:     true,
		ZsCombineMode: "zs",
		OneBiZs:       false,
		ZsAlgo:        "normal",

		BspDivergenceRate: math.Inf(1),
		BspMinZsCnt:       1,
		Bsp1OnlyMultiBiZs: true,
		BspMaxBs2Rate:     0.9999,
		BspMacdAlgo:       "peak",
		Bsp1Peak:          true,
		BspType:           "1,1p,2,2s,3a,3b,support,resist,breakUp,breakDn",
		Bsp2Follow1:       true,
		Bsp3Follow1:       true,
		Bsp3Peak:          false,
		Bsp2sFollow2:      false,
		StrictBsp3:        false,
		Bsp3aMaxZsCnt:     1,

		CalBoll:  false,
		BollN:    20,
		CalRsi:   false,
		RsiCycle: 14,
		CalKdj:   false,
		KdjCycle: 9,

		EnableSegBSP:         false,
		SegBspMacdAlgo:       "slope",
		SegBspDivergenceRate: math.Inf(1),
		SegBsp1OnlyMultiBiZs: false,
		SegBspType:           "1,1p,2,2s,3a,3b",
	}
}

// NewBiConfig 返回新笔标准（严笔）配置。
func NewBiConfig() Config {
	cfg := DefaultConfig()
	cfg.BiMinKLineCount = 5
	return cfg
}

// LooseBiConfig 返回宽松笔配置（匹配 index.html loose 模式）。
func LooseBiConfig() Config {
	cfg := DefaultConfig()
	cfg.BiMinKLineCount = 2
	cfg.BiFxCheck = "none"
	cfg.BiEndIsPeak = false
	cfg.BiStrict = false
	return cfg
}

// StandardBiConfig 返回标准笔配置（匹配 index.html standard 模式）。
func StandardBiConfig() Config {
	cfg := DefaultConfig()
	cfg.BiMinKLineCount = 3
	cfg.BiFxCheck = "half"
	cfg.BiEndIsPeak = true
	cfg.BiStrict = true
	return cfg
}

// StrictBiConfig 返回严格笔配置（匹配 index.html strict 模式）。
func StrictBiConfig() Config {
	cfg := NewBiConfig()
	cfg.BiFxCheck = "strict"
	cfg.BiMinKLineCount = 4
	return cfg
}
