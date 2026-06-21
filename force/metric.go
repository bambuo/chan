package force

import (
	"math"

	"github.com/bambuo/chan/types"
)

// MetricType 枚举力度指标类型。
type MetricType int

const (
	Area      MetricType = 0  // MACD 半面积
	Peak      MetricType = 1  // MACD 峰值
	FullArea  MetricType = 2  // MACD 全面积
	Diff      MetricType = 3  // MACD DIF 差值
	Slope     MetricType = 4  // 价格斜率
	Amp       MetricType = 5  // 价格振幅
	Amount    MetricType = 6  // 成交额总量
	Volume    MetricType = 7  // 成交量总量
	AmountAvg MetricType = 8  // 平均成交额
	VolumeAvg MetricType = 9  // 平均成交量
	Rsi       MetricType = 10 // RSI 极值
)

// ParseMetric 从字符串解析力度指标类型。
func ParseMetric(s string) MetricType {
	switch s {
	case "area":
		return Area
	case "peak":
		return Peak
	case "full_area":
		return FullArea
	case "diff":
		return Diff
	case "slope":
		return Slope
	case "amp":
		return Amp
	case "amount":
		return Amount
	case "volumn", "volume":
		return Volume
	case "amount_avg":
		return AmountAvg
	case "volumn_avg", "volume_avg":
		return VolumeAvg
	case "rsi":
		return Rsi
	default:
		return Peak
	}
}

// CalcMetric 计算指定笔/线段的力度指标值。
func CalcMetric(bi types.Bi, metric MetricType, isReverse bool,
	macdHist, macdDif, volumes, turnovers, closes []float64) float64 {
	switch metric {
	case Area:
		return calcHalfArea(bi, isReverse, macdHist)
	case Peak:
		return calcMACDPeak(bi, macdHist)
	case FullArea:
		return calcFullArea(bi, macdHist)
	case Diff:
		return calcDiffRange(bi, macdHist)
	case Slope:
		return calcSlope(bi)
	case Amp:
		return calcAmp(bi)
	case Amount:
		return calcTradeMetric(bi, turnovers, false)
	case Volume:
		return calcTradeMetric(bi, volumes, false)
	case AmountAvg:
		return calcTradeMetric(bi, turnovers, true)
	case VolumeAvg:
		return calcTradeMetric(bi, volumes, true)
	case Rsi:
		return calcRsiMetric(bi, closes)
	default:
		return calcMACDPeak(bi, macdHist)
	}
}

func calcHalfArea(bi types.Bi, isReverse bool, macdHist []float64) float64 {
	s := 1e-7
	if !isReverse {
		startKlu := bi.StartIndex
		peakMacd := safeIndex(macdHist, startKlu)
		for i := startKlu; i <= bi.EndIndex && i < len(macdHist); i++ {
			if i < 0 {
				continue
			}
			v := macdHist[i]
			if v*peakMacd > 0 {
				s += math.Abs(v)
			} else {
				break
			}
		}
	} else {
		endKlu := bi.EndIndex
		peakMacd := safeIndex(macdHist, endKlu)
		for i := endKlu; i >= bi.StartIndex && i >= 0; i-- {
			if i >= len(macdHist) {
				continue
			}
			v := macdHist[i]
			if v*peakMacd > 0 {
				s += math.Abs(v)
			} else {
				break
			}
		}
	}
	return s
}

func calcMACDPeak(bi types.Bi, macdHist []float64) float64 {
	peak := 1e-7
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(macdHist); i++ {
		if i < 0 {
			continue
		}
		v := macdHist[i]
		if math.Abs(v) > peak {
			if (bi.Direction == types.DirDown && v < 0) || (bi.Direction == types.DirUp && v > 0) {
				peak = math.Abs(v)
			}
		}
	}
	return peak
}

func calcFullArea(bi types.Bi, macdHist []float64) float64 {
	s := 1e-7
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(macdHist); i++ {
		if i < 0 {
			continue
		}
		v := macdHist[i]
		if (bi.Direction == types.DirDown && v < 0) || (bi.Direction == types.DirUp && v > 0) {
			s += math.Abs(v)
		}
	}
	return s
}

func calcDiffRange(bi types.Bi, macdHist []float64) float64 {
	maxVal := math.Inf(-1)
	minVal := math.Inf(1)
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(macdHist); i++ {
		if i < 0 {
			continue
		}
		v := macdHist[i]
		if v > maxVal {
			maxVal = v
		}
		if v < minVal {
			minVal = v
		}
	}
	if math.IsInf(maxVal, -1) || math.IsInf(minVal, 1) {
		return 0
	}
	return maxVal - minVal
}

func calcSlope(bi types.Bi) float64 {
	kCount := bi.EndIndex - bi.StartIndex + 1
	if kCount <= 0 {
		return 0
	}
	if bi.Direction == types.DirUp {
		if bi.EndPrice == 0 {
			return 0
		}
		return (bi.EndPrice - bi.StartPrice) / bi.EndPrice / float64(kCount)
	}
	if bi.StartPrice == 0 {
		return 0
	}
	return (bi.StartPrice - bi.EndPrice) / bi.StartPrice / float64(kCount)
}

func calcAmp(bi types.Bi) float64 {
	if bi.Direction == types.DirDown {
		if bi.StartPrice > 0 {
			return (bi.StartPrice - bi.EndPrice) / bi.StartPrice
		}
		return 0
	}
	if bi.StartPrice > 0 {
		return (bi.EndPrice - bi.StartPrice) / bi.StartPrice
	}
	return 0
}

func calcTradeMetric(bi types.Bi, data []float64, isAvg bool) float64 {
	if data == nil {
		return 0
	}
	s := 0.0
	count := 0
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(data); i++ {
		if i < 0 {
			continue
		}
		s += data[i]
		count++
	}
	if isAvg && count > 0 {
		return s / float64(count)
	}
	return s
}

func calcRsiMetric(bi types.Bi, closes []float64) float64 {
	if closes == nil || len(closes) == 0 {
		return 0
	}
	rsiMin := math.Inf(1)
	rsiMax := math.Inf(-1)
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(closes); i++ {
		if i < 1 || i < 0 {
			continue
		}
		change := closes[i] - closes[i-1]
		rsi := 50.0 + change*10
		if rsi < 0 {
			rsi = 0
		}
		if rsi > 100 {
			rsi = 100
		}
		if rsi < rsiMin {
			rsiMin = rsi
		}
		if rsi > rsiMax {
			rsiMax = rsi
		}
	}
	if bi.Direction == types.DirDown {
		if math.IsInf(rsiMin, 1) {
			return 0
		}
		return 10000.0 / (rsiMin + 1e-7)
	}
	if math.IsInf(rsiMax, -1) {
		return 0
	}
	return rsiMax
}

func safeIndex(arr []float64, idx int) float64 {
	if idx >= 0 && idx < len(arr) {
		return arr[idx]
	}
	return 0
}
