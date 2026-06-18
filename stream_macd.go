package chanlun

import "math"

// ──────────────────────────────────────────────
// 增量 MACD 计算
// ──────────────────────────────────────────────
//
// MACD 基于 EMA（指数移动平均），天然支持增量计算。
// 每根新 K 线到达后，只需 O(1) 更新 fastEMA、slowEMA、signalEMA，
// 无需重新计算全量历史。
//
// EMA 公式：EMA_t = alpha * price + (1 - alpha) * EMA_{t-1}
// DIF = fastEMA - slowEMA
// DEA = signalEMA(DIF)
// MACD柱 = 2 * (DIF - DEA)

// macdIncremental 维护 MACD 的增量计算状态。
type macdIncremental struct {
	fastEMA    float64
	slowEMA    float64
	signalEMA  float64
	fastPeriod int
	slowPeriod int
	sigPeriod  int

	// 初始化计数（EMA 需要足够数据点才有效）
	count int

	// 全量历史（供背驰检测使用）
	dif  []float64 // DIF 线（MACD 线）
	dea  []float64 // DEA 线（Signal 线）
	hist []float64 // 柱状图 = 2*(DIF-DEA)

	initialized bool
}

// newMacdIncremental 创建增量 MACD 计算器。
func newMacdIncremental(fastPeriod, slowPeriod, sigPeriod int) *macdIncremental {
	return &macdIncremental{
		fastPeriod: fastPeriod,
		slowPeriod: slowPeriod,
		sigPeriod:  sigPeriod,
	}
}

// addOne 增量计算一根新 K 线的 MACD 值，O(1)。
func (m *macdIncremental) addOne(close float64) {
	m.count++

	fastAlpha := 2.0 / float64(m.fastPeriod+1)
	slowAlpha := 2.0 / float64(m.slowPeriod+1)
	sigAlpha := 2.0 / float64(m.sigPeriod+1)

	if !m.initialized {
		// 第一根 K 线：EMA 初始化为第一个价格
		m.fastEMA = close
		m.slowEMA = close
		m.signalEMA = 0
		m.initialized = true
		m.dif = append(m.dif, 0)
		m.dea = append(m.dea, 0)
		m.hist = append(m.hist, 0)
		return
	}

	// EMA 增量更新
	m.fastEMA = fastAlpha*close + (1-fastAlpha)*m.fastEMA
	m.slowEMA = slowAlpha*close + (1-slowAlpha)*m.slowEMA

	dif := m.fastEMA - m.slowEMA
	m.signalEMA = sigAlpha*dif + (1-sigAlpha)*m.signalEMA

	m.dif = append(m.dif, dif)
	m.dea = append(m.dea, m.signalEMA)
	m.hist = append(m.hist, 2*(dif-m.signalEMA))
}

// addBatch 批量添加收盘价，用于初始化。
func (m *macdIncremental) addBatch(closes []float64) {
	for _, c := range closes {
		m.addOne(c)
	}
}

// isValid 检查 MACD 数据是否已经过足够的预热期。
// EMA 通常需要约 2*period 根 K 线才能收敛。
func (m *macdIncremental) isValid() bool {
	return m.count >= m.slowPeriod+m.sigPeriod
}

// lastN 返回最近 N 个 MACD 值（用于背驰检测）。
func (m *macdIncremental) lastN(n int) (dif, dea, hist []float64) {
	if n <= 0 || len(m.dif) == 0 {
		return nil, nil, nil
	}
	start := len(m.dif) - n
	if start < 0 {
		start = 0
	}
	return m.dif[start:], m.dea[start:], m.hist[start:]
}

// calcAreaInRange 计算指定索引范围内的 MACD 柱状图面积（绝对值之和）。
func (m *macdIncremental) calcAreaInRange(startIdx, endIdx int) float64 {
	area := 0.0
	for i := startIdx; i <= endIdx && i < len(m.hist); i++ {
		if i >= 0 && !math.IsNaN(m.hist[i]) {
			area += math.Abs(m.hist[i])
		}
	}
	return area
}

// calcDiffExtremeInRange 计算指定索引范围内的 DIF 极值（绝对值最大）。
func (m *macdIncremental) calcDiffExtremeInRange(startIdx, endIdx int) float64 {
	extreme := 0.0
	for i := startIdx; i <= endIdx && i < len(m.dif); i++ {
		if i >= 0 && !math.IsNaN(m.dif[i]) {
			v := math.Abs(m.dif[i])
			if v > extreme {
				extreme = v
			}
		}
	}
	return extreme
}

// hasDIFCrossZero 检查指定范围内 DIF 与 DEA 是否交叉（穿越 0 轴附近）。
func (m *macdIncremental) hasDIFCrossZero(startIdx, endIdx int) bool {
	for i := startIdx; i <= endIdx && i < len(m.dif) && i < len(m.dea); i++ {
		if i >= 0 && !math.IsNaN(m.dif[i]) && !math.IsNaN(m.dea[i]) {
			if m.dif[i]*m.dea[i] <= 0 {
				return true
			}
		}
	}
	return false
}
