// Package chanlun 提供缠论算法引擎。
//
// 用法：
//
//	result := chanlun.NewAnalysis(klines, chanlun.DefaultConfig()).
//	    MergeKlines().
//	    DetectFractals().
//	    BuildBis().
//	    BuildSegments().
//	    DetectPivots().
//	    ClassifyTrends().
//	    DetectDeviations().
//	    DetectSignals().
//	    ScoreSignals().
//	    Result()
package chanlun

import (
	"github.com/bambuo/chan/types"
)

// ── 类型导出 ──

type (
	DateTime              = types.DateTime
	Kline                 = types.Kline
	BOLLValue             = types.BOLLValue
	KDJValue              = types.KDJValue
	Fractal               = types.Fractal
	FractalType           = types.FractalType
	Direction             = types.Direction
	Bi                    = types.Bi
	MergedBi              = types.MergedBi
	FeatureElement        = types.FeatureElement
	BreakType             = types.BreakType
	Segment               = types.Segment
	PivotState            = types.PivotState
	Pivot                 = types.Pivot
	TrendType             = types.TrendType
	Trend                 = types.Trend
	DeviationLevel        = types.DeviationLevel
	Deviation             = types.Deviation
	SignalType            = types.SignalType
	SignalSubType         = types.SignalSubType
	Signal                = types.Signal
	Result                = types.Result
	InclusionOption       = types.InclusionOption
	Config                = types.Config
	ZSUnit                = types.ZSUnit
	IntervalNestingResult = types.IntervalNestingResult
)

// ── 常量导出 ──

const (
	DirDown = types.DirDown
	DirNone = types.DirNone
	DirUp   = types.DirUp
)

const (
	FractalNone   = types.FractalNone
	TopFractal    = types.TopFractal
	BottomFractal = types.BottomFractal
)

const (
	BreakNone   = types.BreakNone
	BreakStd    = types.BreakStd
	BreakStroke = types.BreakStroke
)

const (
	PivotForming   = types.PivotForming
	PivotFormed    = types.PivotFormed
	PivotExtending = types.PivotExtending
	PivotExpanded  = types.PivotExpanded
	PivotEnlarged  = types.PivotEnlarged
	PivotDestroyed = types.PivotDestroyed
)

const (
	TrendUp   = types.TrendUp
	TrendDown = types.TrendDown
	RangeOnly = types.RangeOnly
)

const (
	BiDeviation      = types.BiDeviation
	SegmentDeviation = types.SegmentDeviation
	TrendDeviation   = types.TrendDeviation
)

const (
	BuyPoint1  = types.BuyPoint1
	BuyPoint2  = types.BuyPoint2
	BuyPoint3  = types.BuyPoint3
	SellPoint1 = types.SellPoint1
	SellPoint2 = types.SellPoint2
	SellPoint3 = types.SellPoint3
)

const (
	SubT1       = types.SubT1
	SubT1P      = types.SubT1P
	SubT2       = types.SubT2
	SubT2S      = types.SubT2S
	SubT3A      = types.SubT3A
	SubT3B      = types.SubT3B
	SubTSupport = types.SubTSupport
	SubTResist  = types.SubTResist
	SubTBreakUp = types.SubTBreakUp
	SubTBreakDn = types.SubTBreakDn
)

// ── 函数导出 ──

var (
	DefaultConfig    = types.DefaultConfig
	NewBiConfig      = types.NewBiConfig
	LooseBiConfig    = types.LooseBiConfig
	StandardBiConfig = types.StandardBiConfig
	StrictBiConfig   = types.StrictBiConfig
)
