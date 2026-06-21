package force

import (
	"math"

	"github.com/bambuo/chan/types"
)

// epsilon 防零基值，用于避免除零和 log(0) 等数值问题。
// 统一引用 types.ForceEpsilon，避免跨包 epsilon 不一致导致假背驰信号。
const epsilon = types.ForceEpsilon

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
		return calcDiffRange(bi, macdDif)
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
		return calcRsiMetric(bi, defaultRSIPeriod, closes)
	default:
		return calcMACDPeak(bi, macdHist)
	}
}

// defaultRSIPeriod 为 CalcMetric 中 RSI 指标的默认周期。
// 与 kline.CalcIndicators 的 Config.RsiCycle 默认值（9）独立，
// 采用标准周期 14 以保证背驰力度判定的可比性。
const defaultRSIPeriod = 14

// CalcRSIMetric 使用可配置周期的 RSI 计算笔的力度指标。
// rsiPeriod 指定 Wilder 平滑周期（标准值 14）。
func CalcRSIMetric(bi types.Bi, rsiPeriod int, closes []float64) float64 {
	return calcRsiMetric(bi, rsiPeriod, closes)
}

func calcHalfArea(bi types.Bi, isReverse bool, macdHist []float64) float64 {
	s := epsilon
	if !isReverse {
		startKlu := bi.StartIndex
		peakMacd := safeIndex(macdHist, startKlu)
		// 当起点 MACD=0（零轴交叉），向前搜索第一个非零值
		startFrom := startKlu
		if peakMacd == 0 {
			for i := startKlu + 1; i <= bi.EndIndex && i < len(macdHist); i++ {
				if i < 0 {
					continue
				}
				if macdHist[i] != 0 {
					peakMacd = macdHist[i]
					startFrom = i
					break
				}
			}
			if peakMacd == 0 {
				return s // 笔内全为零
			}
		}
		for i := startFrom; i <= bi.EndIndex && i < len(macdHist); i++ {
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
		// 当终点 MACD=0（零轴交叉），向后搜索第一个非零值
		startFrom := endKlu
		if peakMacd == 0 {
			for i := endKlu - 1; i >= bi.StartIndex && i >= 0; i-- {
				if i >= len(macdHist) {
					continue
				}
				if macdHist[i] != 0 {
					peakMacd = macdHist[i]
					startFrom = i
					break
				}
			}
			if peakMacd == 0 {
				return s // 笔内全为零
			}
		}
		for i := startFrom; i >= bi.StartIndex && i >= 0; i-- {
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
	peak := epsilon
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
	s := epsilon
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

func calcDiffRange(bi types.Bi, macdDif []float64) float64 {
	maxVal := math.Inf(-1)
	minVal := math.Inf(1)
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(macdDif); i++ {
		if i < 0 {
			continue
		}
		v := macdDif[i]
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

// calcRsiMetric 使用 Wilder 平滑 RSI 公式计算笔的力度指标。
//
// 对每根 K 线（从 bi.StartIndex 开始）计算 RSI，然后在 bi 范围内：
//   - 向上笔：取 RSI 最大值（数值越高力度越强）
//   - 向下笔：取 RSI 最小值的倒数变换（数值越低 → 返回值越大，表示下跌力度越强）
func calcRsiMetric(bi types.Bi, rsiPeriod int, closes []float64) float64 {
	if closes == nil || len(closes) < rsiPeriod+1 || rsiPeriod <= 0 {
		return 0
	}
	rp := float64(rsiPeriod)

	// 计算价格变化并初始化平均涨跌
	var gains, losses []float64
	for i := 1; i < len(closes) && i <= bi.EndIndex; i++ {
		if i < 1 {
			continue
		}
		ch := closes[i] - closes[i-1]
		if ch > 0 {
			gains = append(gains, ch)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, -ch)
		}
	}
	if len(gains) < rsiPeriod {
		return 0
	}

	// Wilder 平滑 RSI
	avgGain := mean(gains[:rsiPeriod])
	avgLoss := mean(losses[:rsiPeriod])

	rsis := make([]float64, len(gains))
	rsi := 50.0
	if avgLoss > epsilon {
		rsi = 100.0 - 100.0/(1.0+avgGain/avgLoss)
	}
	rsis[rsiPeriod-1] = rsi

	for i := rsiPeriod; i < len(gains); i++ {
		avgGain = (avgGain*(rp-1) + gains[i]) / rp
		avgLoss = (avgLoss*(rp-1) + losses[i]) / rp
		if avgLoss > epsilon {
			rsi = 100.0 - 100.0/(1.0+avgGain/avgLoss)
		} else {
			rsi = 100.0
		}
		rsis[i] = rsi
	}

	// 在 bi 范围内取极值
	rsiMin := math.Inf(1)
	rsiMax := math.Inf(-1)
	start := bi.StartIndex
	if start < rsiPeriod {
		start = rsiPeriod - 1 // RSI 从第 rsiPeriod 个变化开始有效
	}
	for i := start; i <= bi.EndIndex && i < len(closes); i++ {
		ri := i - 1 // gains/losses/rsis 的索引偏移 1
		if ri < 0 || ri >= len(rsis) {
			continue
		}
		v := rsis[ri]
		if v < rsiMin {
			rsiMin = v
		}
		if v > rsiMax {
			rsiMax = v
		}
	}

	if math.IsInf(rsiMin, 1) || math.IsInf(rsiMax, -1) {
		return 0
	}

	if bi.Direction == types.DirDown {
		// 下跌力度与 RSI 值成反比：RSI 越低 → 力度越大
		denom := rsiMin + epsilon
		if denom <= 0 {
			denom = epsilon
		}
		return 100.0 / denom
	}
	return rsiMax
}

func mean(vals []float64) float64 {
	s := 0.0
	for _, v := range vals {
		s += v
	}
	return s / float64(len(vals))
}

func safeIndex(arr []float64, idx int) float64 {
	if idx >= 0 && idx < len(arr) {
		return arr[idx]
	}
	return 0
}
