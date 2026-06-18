package chanlun

import "math"

// ──────────────────────────────────────────────
// 技术指标增量计算器
// ──────────────────────────────────────────────
//
// 支持 BOLL/RSI/KDJ/MA 等指标的增量计算。
// 每根新 K 线到达后，O(1) 更新所有已启用指标。
// 移植自 chan.py KLine_Unit.set_metric。

// IndicatorCalculator 管理所有技术指标的增量计算状态。
type IndicatorCalculator struct {
	config Config

	// MA 状态
	maSums map[int]float64   // 各周期的滑动窗口和
	maBuf  map[int][]float64 // 各周期的价格缓冲区

	// BOLL 状态
	bollSum   float64
	bollSumSq float64
	bollBuf   []float64
	bollCount int

	// RSI 状态
	rsiAvgGain   float64
	rsiAvgLoss   float64
	rsiCount     int
	rsiLastClose float64

	// KDJ 状态
	kdjRSV   float64
	kdjK     float64
	kdjD     float64
	kdjHighs []float64
	kdjLows  []float64
}

// NewIndicatorCalculator 创建指标计算器。
func NewIndicatorCalculator(config Config) *IndicatorCalculator {
	ic := &IndicatorCalculator{
		config: config,
		maSums: make(map[int]float64),
		maBuf:  make(map[int][]float64),
	}

	// 初始化 MA 缓冲区
	for _, period := range config.MeanMetrics {
		ic.maBuf[period] = make([]float64, 0, period)
	}

	// 初始化 BOLL 缓冲区
	if config.CalBoll {
		ic.bollBuf = make([]float64, 0, config.BollN)
	}

	return ic
}

// AddKline 增量计算所有已启用指标，返回更新后的 Kline。
func (ic *IndicatorCalculator) AddKline(k Kline) Kline {
	result := k

	// MA 计算
	if len(ic.config.MeanMetrics) > 0 {
		result.MA = make(map[int]float64)
		for _, period := range ic.config.MeanMetrics {
			ma := ic.updateMA(period, k.Close)
			result.MA[period] = ma
		}
	}

	// BOLL 计算
	if ic.config.CalBoll {
		boll := ic.updateBOLL(k.Close)
		result.BOLL = boll
	}

	// RSI 计算
	if ic.config.CalRsi {
		rsi := ic.updateRSI(k.Close)
		result.RSI = &rsi
	}

	// KDJ 计算
	if ic.config.CalKdj {
		kdj := ic.updateKDJ(k.High, k.Low, k.Close)
		result.KDJ = kdj
	}

	return result
}

// updateMA 增量更新移动平均线。
func (ic *IndicatorCalculator) updateMA(period int, close float64) float64 {
	buf := ic.maBuf[period]
	buf = append(buf, close)
	ic.maSums[period] += close

	if len(buf) > period {
		ic.maSums[period] -= buf[0]
		buf = buf[1:]
		ic.maBuf[period] = buf
	}

	if len(buf) < period {
		return 0
	}
	return ic.maSums[period] / float64(period)
}

// updateBOLL 增量更新布林带。
func (ic *IndicatorCalculator) updateBOLL(close float64) *BOLLValue {
	n := ic.config.BollN
	ic.bollBuf = append(ic.bollBuf, close)
	ic.bollSum += close
	ic.bollSumSq += close * close
	ic.bollCount++

	if len(ic.bollBuf) > n {
		old := ic.bollBuf[0]
		ic.bollSum -= old
		ic.bollSumSq -= old * old
		ic.bollBuf = ic.bollBuf[1:]
	}

	if len(ic.bollBuf) < n {
		return nil
	}

	mid := ic.bollSum / float64(n)
	variance := ic.bollSumSq/float64(n) - mid*mid
	if variance < 0 {
		variance = 0
	}
	stdDev := math.Sqrt(variance)

	return &BOLLValue{
		Upper: mid + 2*stdDev,
		Mid:   mid,
		Lower: mid - 2*stdDev,
	}
}

// updateRSI 增量更新 RSI。
func (ic *IndicatorCalculator) updateRSI(close float64) float64 {
	n := ic.config.RsiCycle
	ic.rsiCount++

	if ic.rsiCount <= 1 {
		ic.rsiLastClose = close
		return 50.0 // 默认中性值
	}

	change := close - ic.rsiLastClose
	ic.rsiLastClose = close

	gain := 0.0
	loss := 0.0
	if change > 0 {
		gain = change
	} else {
		loss = -change
	}

	if ic.rsiCount <= n+1 {
		// 初始累积期
		ic.rsiAvgGain += gain
		ic.rsiAvgLoss += loss
		if ic.rsiCount == n+1 {
			ic.rsiAvgGain /= float64(n)
			ic.rsiAvgLoss /= float64(n)
		} else {
			return 50.0
		}
	} else {
		// 平滑期
		ic.rsiAvgGain = (ic.rsiAvgGain*float64(n-1) + gain) / float64(n)
		ic.rsiAvgLoss = (ic.rsiAvgLoss*float64(n-1) + loss) / float64(n)
	}

	if ic.rsiAvgLoss == 0 {
		return 100.0
	}
	rs := ic.rsiAvgGain / ic.rsiAvgLoss
	return 100.0 - 100.0/(1.0+rs)
}

// updateKDJ 增量更新 KDJ 指标。
func (ic *IndicatorCalculator) updateKDJ(high, low, close float64) *KDJValue {
	n := ic.config.KdjCycle
	ic.kdjHighs = append(ic.kdjHighs, high)
	ic.kdjLows = append(ic.kdjLows, low)

	// 保持窗口大小为 n
	if len(ic.kdjHighs) > n {
		ic.kdjHighs = ic.kdjHighs[1:]
		ic.kdjLows = ic.kdjLows[1:]
	}

	if len(ic.kdjHighs) < n {
		// 数据不足
		return &KDJValue{K: 50, D: 50, J: 50}
	}

	// 计算 N 周期内最高价和最低价
	hh := ic.kdjHighs[0]
	ll := ic.kdjLows[0]
	for i := 1; i < n; i++ {
		if ic.kdjHighs[i] > hh {
			hh = ic.kdjHighs[i]
		}
		if ic.kdjLows[i] < ll {
			ll = ic.kdjLows[i]
		}
	}

	// RSV
	if hh == ll {
		ic.kdjRSV = 50
	} else {
		ic.kdjRSV = (close - ll) / (hh - ll) * 100
	}

	// K = 2/3 * K_prev + 1/3 * RSV
	ic.kdjK = 2.0/3.0*ic.kdjK + 1.0/3.0*ic.kdjRSV
	// D = 2/3 * D_prev + 1/3 * K
	ic.kdjD = 2.0/3.0*ic.kdjD + 1.0/3.0*ic.kdjK
	// J = 3*K - 2*D
	j := 3.0*ic.kdjK - 2.0*ic.kdjD

	return &KDJValue{K: ic.kdjK, D: ic.kdjD, J: j}
}
