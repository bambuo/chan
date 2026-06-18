package chanlun

import "math"

// ──────────────────────────────────────────────
// 力度指标计算（移植自 chan.py CBi.cal_macd_metric）
// ──────────────────────────────────────────────
//
// 支持 11 种力度指标，用于背驰检测和买卖点判定。
// 每种指标从不同角度衡量走势力度。

// ForceMetricType 枚举力度指标类型。
type ForceMetricType int

const (
	ForceArea      ForceMetricType = 0  // MACD 半面积（从起点到首次穿越零轴）
	ForcePeak      ForceMetricType = 1  // MACD 峰值（绝对值最大）
	ForceFullArea  ForceMetricType = 2  // MACD 全面积（全段绝对值之和）
	ForceDiff      ForceMetricType = 3  // MACD DIF 差值（最大值-最小值）
	ForceSlope     ForceMetricType = 4  // 价格斜率（价格变化/K线数/起始价）
	ForceAmp       ForceMetricType = 5  // 价格振幅（高低价差/起始价）
	ForceAmount    ForceMetricType = 6  // 成交额总量
	ForceVolume    ForceMetricType = 7  // 成交量总量
	ForceAmountAvg ForceMetricType = 8  // 平均成交额
	ForceVolumeAvg ForceMetricType = 9  // 平均成交量
	ForceRsi       ForceMetricType = 10 // RSI 极值
)

// ParseForceMetricType 从字符串解析力度指标类型。
func ParseForceMetricType(s string) ForceMetricType {
	switch s {
	case "area":
		return ForceArea
	case "peak":
		return ForcePeak
	case "full_area":
		return ForceFullArea
	case "diff":
		return ForceDiff
	case "slope":
		return ForceSlope
	case "amp":
		return ForceAmp
	case "amount":
		return ForceAmount
	case "volumn", "volume":
		return ForceVolume
	case "amount_avg":
		return ForceAmountAvg
	case "volumn_avg", "volume_avg":
		return ForceVolumeAvg
	case "rsi":
		return ForceRsi
	default:
		return ForcePeak
	}
}

// CalcForceMetric 计算指定笔/线段的力度指标值。
//
// 参数：
//   - bi: 目标笔（含 StartIndex/EndIndex/Direction）
//   - metric: 力度指标类型
//   - isReverse: 是否反向计算（从终点向起点）
//   - macdHist: MACD 柱状图值序列（与 K 线索引对齐）
//   - macdDif: MACD DIF 值序列
//   - volumes: 成交量序列
//   - turnovers: 成交额序列
//   - closes: 收盘价序列（用于 RSI）
func CalcForceMetric(bi Bi, metric ForceMetricType, isReverse bool,
	macdHist, macdDif, volumes, turnovers, closes []float64) float64 {

	switch metric {
	case ForceArea:
		return calcMACDHalfArea(bi, isReverse, macdHist)
	case ForcePeak:
		return calcMACDPeak(bi, macdHist)
	case ForceFullArea:
		return calcMACDFullArea(bi, macdHist)
	case ForceDiff:
		return calcMACDDiffRange(bi, macdHist)
	case ForceSlope:
		return calcSlope(bi)
	case ForceAmp:
		return calcAmp(bi)
	case ForceAmount:
		return calcTradeMetric(bi, turnovers, false)
	case ForceVolume:
		return calcTradeMetric(bi, volumes, false)
	case ForceAmountAvg:
		return calcTradeMetric(bi, turnovers, true)
	case ForceVolumeAvg:
		return calcTradeMetric(bi, volumes, true)
	case ForceRsi:
		return calcRsiMetric(bi, closes)
	default:
		return calcMACDPeak(bi, macdHist)
	}
}

// calcMACDHalfArea 计算 MACD 半面积（从起点到首次穿越零轴）。
// 移植自 chan.py CBi.Cal_MACD_half。
func calcMACDHalfArea(bi Bi, isReverse bool, macdHist []float64) float64 {
	s := 1e-7

	if !isReverse {
		// 正向：从起点开始，遇到同向 MACD 累加，遇到反向停止
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
		// 反向：从终点开始
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

// calcMACDPeak 计算 MACD 峰值（笔范围内绝对值最大的 MACD 柱值）。
func calcMACDPeak(bi Bi, macdHist []float64) float64 {
	peak := 1e-7
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(macdHist); i++ {
		if i < 0 {
			continue
		}
		v := macdHist[i]
		if math.Abs(v) > peak {
			if (bi.Direction == DirDown && v < 0) || (bi.Direction == DirUp && v > 0) {
				peak = math.Abs(v)
			}
		}
	}
	return peak
}

// calcMACDFullArea 计算 MACD 全面积（笔范围内所有柱值的绝对值之和）。
func calcMACDFullArea(bi Bi, macdHist []float64) float64 {
	s := 1e-7
	for i := bi.StartIndex; i <= bi.EndIndex && i < len(macdHist); i++ {
		if i < 0 {
			continue
		}
		v := macdHist[i]
		if (bi.Direction == DirDown && v < 0) || (bi.Direction == DirUp && v > 0) {
			s += math.Abs(v)
		}
	}
	return s
}

// calcMACDDiffRange 计算 MACD 柱值范围（最大值 - 最小值）。
func calcMACDDiffRange(bi Bi, macdHist []float64) float64 {
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

// calcSlope 计算价格斜率。
// 移植自 chan.py CBi.Cal_MACD_slope。
func calcSlope(bi Bi) float64 {
	kCount := bi.EndIndex - bi.StartIndex + 1
	if kCount <= 0 {
		return 0
	}
	if bi.Direction == DirUp {
		return (bi.High - bi.Low) / bi.High / float64(kCount)
	}
	return (bi.High - bi.Low) / bi.High / float64(kCount)
}

// calcAmp 计算价格振幅。
// 移植自 chan.py CBi.Cal_MACD_amp。
func calcAmp(bi Bi) float64 {
	if bi.Direction == DirDown {
		if bi.High > 0 {
			return (bi.High - bi.Low) / bi.High
		}
		return 0
	}
	if bi.Low > 0 {
		return (bi.High - bi.Low) / bi.Low
	}
	return 0
}

// calcTradeMetric 计算成交量/成交额指标。
func calcTradeMetric(bi Bi, data []float64, isAvg bool) float64 {
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

// calcRsiMetric 计算 RSI 极值指标。
// 向下笔取最小 RSI（倒数），向上笔取最大 RSI。
func calcRsiMetric(bi Bi, closes []float64) float64 {
	if closes == nil || len(closes) == 0 {
		return 0
	}
	// 简化版 RSI 计算（笔范围内收盘价变化的比例）
	rsiMin := math.Inf(1)
	rsiMax := math.Inf(-1)

	for i := bi.StartIndex; i <= bi.EndIndex && i < len(closes); i++ {
		if i < 1 || i < 0 {
			continue
		}
		change := closes[i] - closes[i-1]
		// 简单 RSI 近似
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

	if bi.Direction == DirDown {
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

// safeIndex 安全获取数组元素，越界返回 0。
func safeIndex(arr []float64, idx int) float64 {
	if idx >= 0 && idx < len(arr) {
		return arr[idx]
	}
	return 0
}
