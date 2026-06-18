package chanlun

import (
	"math"

	"github.com/bambuo/talib"
)

// ──────────────────────────────────────────────
// §7  背驰（Deviation / Bei Chi）
// ──────────────────────────────────────────────
//
// 背驰是价格与力度之间的背离，是判断走势是否衰竭的核心依据。
//
// 三级分类：
//   笔背驰（信号最弱）→ 线段背驰（信号中等）→ 走势背驰（信号最强）
//
// MACD 三要素确认条件（需同时满足）：
//   1. MACD 面积缩小（后段面积 < 前段面积）
//   2. MACD 黄白线高度降低（后段 DIF 极值 < 前段 DIF 极值）
//   3. 价格创新高/低（后段绝对价格位置超越前段，但力度反之）

// DetectDeviations 在指定级别的走势/线段中检测背驰。
// segments: 待检测的线段列表
// macdMACD: MACD 线值（长度与原始蜡烛一致，前导值为 NaN）
// macdSignal: 信号线值
// macdHist: 柱状图值
//
// 检测线段背驰（盘整背驰 + 潜在趋势背驰）：
//
//	相邻同向线段的力度对比，由 MACD 三要素确认。
//	信号强度分级：走势背驰 > 线段背驰（盘整背驰）> 笔背驰
//	最终的结构验证（a+A+b+B+c）在 DetectTrendDeviations 中完成。
func DetectDeviations(segments []Segment, macdMACD, macdSignal, macdHist []float64) []Deviation {
	if len(segments) < 2 {
		return nil
	}

	deviations := make([]Deviation, 0)

	// 在相邻线段之间检测背驰
	for i := 1; i < len(segments); i++ {
		prev := segments[i-1]
		curr := segments[i]

		// 同向线段才进行背驰比较
		if prev.Direction != curr.Direction {
			continue
		}

		dev := compareSegments(prev, i-1, curr, i, macdMACD, macdSignal, macdHist)
		if dev != nil {
			dev.Type = "range" // 线段间背驰，默认为盘整背驰（具体类型由上层决定）
			deviations = append(deviations, *dev)
		}
	}

	return deviations
}

// DetectTrendDeviations 基于 a+A+b+B+c 结构检测趋势背驰。
// 文档 §7.3：趋势背驰需要完整的走势结构。
//
//	结构：a(进入段) + A(中枢1) + b(中间段) + B(中枢2) + c(离开段)
//	条件：
//	  1. A 和 B 为同级别同向中枢
//	  2. c 段离开 B 时包含第三类买卖点
//	  3. c 段的力度 < a 段的力度
//	  4. 价格创新高/低
//
// pivots 和 trends 用于验证结构完整性。
func DetectTrendDeviations(segments []Segment, pivots []Pivot, trends []Trend, macdMACD, macdSignal, macdHist []float64) []Deviation {
	if len(pivots) < 2 || len(trends) == 0 {
		return nil
	}

	deviations := make([]Deviation, 0)

	// 遍历每个走势，检查是否有完整的 a+A+b+B+c 结构
	for _, trend := range trends {
		if len(trend.Pivots) < 2 {
			continue
		}

		// 取走势中的最后两个中枢作为 A 和 B
		aPivot := trend.Pivots[0]                   // A
		bPivot := trend.Pivots[len(trend.Pivots)-1] // B

		// 确保 B 被破坏（包含第三类买卖点）
		if bPivot.State != PivotDestroyed {
			continue
		}

		// 找到进入段 a：A 之前的第一个同向段
		var aSeg *Segment
		var cSeg *Segment

		for i := range segments {
			if segments[i].EndIndex <= aPivot.StartIndex &&
				segments[i].Direction == trendTypeToDir(trend.Type) {
				aSeg = &segments[i]
			}
			if segments[i].StartIndex >= bPivot.EndIndex &&
				segments[i].Direction == trendTypeToDir(trend.Type) {
				cSeg = &segments[i]
				break
			}
		}

		if aSeg == nil || cSeg == nil {
			continue
		}

		// 比较 c 段和 a 段的力度
		aIdx := findSegIndex(segments, aSeg)
		cIdx := findSegIndex(segments, cSeg)
		dev := compareSegments(*aSeg, aIdx, *cSeg, cIdx, macdMACD, macdSignal, macdHist)
		if dev != nil {
			dev.Level = TrendDeviation // 标记为走势级别背驰（最强）
			dev.Type = "trend"
			deviations = append(deviations, *dev)
		}
	}

	return deviations
}

// compareSegments 比较两段同向线段的力度，检测背驰。
func compareSegments(prev Segment, prevIdx int, curr Segment, currIdx int, macdMACD, macdSignal, macdHist []float64) *Deviation {
	// 价格是否创新高/低
	priceBreak := false
	devDir := DirNone

	if prev.Direction == DirUp {
		// 向上线段：当前段的最高价比前段高
		if curr.Top > prev.Top {
			priceBreak = true
			devDir = DirUp // 顶背驰
		}
	} else {
		// 向下线段：当前段的最低价破前段的低
		if curr.Bottom < prev.Bottom {
			priceBreak = true
			devDir = DirDown // 底背驰
		}
	}

	if !priceBreak {
		return nil
	}

	// 检查 MACD 三要素（面积缩小 AND 黄白线降低 AND 回抽 0 轴）
	areaBefore := calcMACDArea(prev, macdHist)
	areaAfter := calcMACDArea(curr, macdHist)
	diffBefore := calcMACDDiff(prev, macdMACD)
	diffAfter := calcMACDDiff(curr, macdMACD)

	// 根据配置选择背驰确认方法
	if !checkMACDDeviation(areaBefore, areaAfter, diffBefore, diffAfter, prev, curr, macdMACD, macdSignal) {
		return nil
	}

	// 计算力度值（净价格变化幅度 / K线数量）
	forceBefore := calcForce(prev)
	forceAfter := calcForce(curr)

	return &Deviation{
		Type:           "", // 由调用方设置（"trend" 或 "range"）
		Level:          SegmentDeviation,
		Direction:      devDir,
		SegmentBefore:  &prev,
		SegmentAfter:   &curr,
		SegBeforeIdx:   prevIdx,
		SegAfterIdx:    currIdx,
		PriceHigh:      curr.Top,
		ForceBefore:    forceBefore,
		ForceAfter:     forceAfter,
		MACDAreaBefore: areaBefore,
		MACDAreaAfter:  areaAfter,
		MACDDiffBefore: diffBefore,
		MACDDiffAfter:  diffAfter,
	}
}

// calcForce 计算线段的趋势力度。
// 文档 §7.3: 趋势力度 = 价格变化幅度 / 时间（K 线数量）
// 价格变化幅度 = 线段起点到终点的净价格变化，非段内总振幅。
func calcForce(seg Segment) float64 {
	if len(seg.BiList) == 0 {
		return 0
	}
	// 从首笔起点到末笔终点的净价格变化
	startPrice := seg.BiList[0].StartPrice
	endPrice := seg.BiList[len(seg.BiList)-1].EndPrice
	netChange := endPrice - startPrice
	if netChange < 0 {
		netChange = -netChange
	}
	kCount := seg.EndIndex - seg.StartIndex + 1
	if kCount <= 0 {
		return 0
	}
	return netChange / float64(kCount)
}

// ──────────────────────────────────────────────
// §7.5  区间套定位
// ──────────────────────────────────────────────
func calcMACDArea(seg Segment, macdHist []float64) float64 {
	area := 0.0
	for i := seg.StartIndex; i <= seg.EndIndex && i < len(macdHist); i++ {
		if i >= 0 && !math.IsNaN(macdHist[i]) {
			area += math.Abs(macdHist[i])
		}
	}
	return area
}

// calcMACDDiff 计算线段对应的 MACD DIF 极值（绝对值）。
func calcMACDDiff(seg Segment, macdMACD []float64) float64 {
	extreme := 0.0
	for i := seg.StartIndex; i <= seg.EndIndex && i < len(macdMACD); i++ {
		if i >= 0 && !math.IsNaN(macdMACD[i]) {
			val := math.Abs(macdMACD[i])
			if val > extreme {
				extreme = val
			}
		}
	}
	return extreme
}

// checkMACDDeviation 检查 MACD 三要素是否同时满足背驰确认条件。
// 文档 §7.4：三要素需同时满足：
//  1. MACD 面积缩小（后段面积 < 前段面积）
//  2. MACD 黄白线高度降低（后段 DIF 极值 < 前段 DIF 极值）
//  3. DIF 有效回抽 0 轴
func checkMACDDeviation(areaBefore, areaAfter, diffBefore, diffAfter float64, prev, curr Segment, macdMACD, macdSignal []float64) bool {
	// 1. MACD 面积缩小
	areaShrunk := areaAfter < areaBefore

	// 2. MACD 黄白线高度降低
	diffReduced := diffAfter < diffBefore

	// 3. DIF 回抽 0 轴检查
	zeroReturn := checkDIFReturn(prev, curr, macdMACD, macdSignal)

	return areaShrunk && diffReduced && zeroReturn
}

// checkDIFReturn 检查 DIF 是否有效回抽 0 轴。
// 文档 §7.4：
//   - DIF 与 DEA 交叉（DIF * DEA <= 0）视为有效回抽
//   - 或 |DIF| < ATR(DIF, N) × 阈值
func checkDIFReturn(prev, curr Segment, macdMACD, macdSignal []float64) bool {
	// 检查两段之间的过渡区域是否有 DIF 回抽 0 轴
	transitionStart := prev.StartIndex
	if curr.StartIndex < transitionStart {
		transitionStart = curr.StartIndex
	}

	for i := transitionStart; i <= curr.EndIndex && i < len(macdMACD) && i < len(macdSignal); i++ {
		if i >= 0 && !math.IsNaN(macdMACD[i]) && !math.IsNaN(macdSignal[i]) {
			// DIF * DEA <= 0 表示穿越 0 轴附近
			if macdMACD[i]*macdSignal[i] <= 0 {
				return true
			}
		}
	}
	return false
}

// ──────────────────────────────────────────────
// §7.5  区间套定位
// ──────────────────────────────────────────────

// MultiLevelDataProvider 为区间套提供多级别数据。
// LevelData 存储单个时间周期的区间套所需数据。
type LevelData struct {
	Segments   []Segment `json:"segments"`
	MACD       []float64 `json:"macd"`
	MACDSignal []float64 `json:"macdSignal"`
	MACDHist   []float64 `json:"macdHist"`
}

// MultiLevelDataProvider 为区间套提供多级别数据。
type MultiLevelDataProvider struct {
	Levels []string             `json:"levels"` // 级别名称，如 ["1h", "30m", "5m"]
	Data   map[string]LevelData `json:"data"`   // 每个级别的数据
}

// PerformIntervalNesting 执行区间套定位。
// 从最高级别开始逐级下钻，直到所有级别都确认背驰或某一级别未能确认。
//
// 注意：不同级别的 K 线索引空间不同。当前实现假定较高频的数据
// 具有更多的 K 线，因此使用索引范围作为近似时间对齐手段。
// 精确实现应使用时间戳进行跨级别对齐。
func PerformIntervalNesting(provider *MultiLevelDataProvider) *IntervalNestingResult {
	if provider == nil || len(provider.Levels) < 2 {
		return nil
	}

	result := &IntervalNestingResult{
		Levels:        make([]string, 0),
		Confirmations: make([]Deviation, 0),
	}

	// 从最高级别逐级向下确认
	for i, level := range provider.Levels {
		data, ok := provider.Data[level]
		if !ok {
			break
		}

		// 检测当前级别的背驰
		deviations := DetectDeviations(data.Segments, data.MACD, data.MACDSignal, data.MACDHist)
		if len(deviations) == 0 {
			// 当前级别未确认背驰，区间套链条断裂
			break
		}

		result.Levels = append(result.Levels, level)
		result.Confirmations = append(result.Confirmations, deviations[len(deviations)-1])

		if i == 0 {
			// 最高级别：记录初始位置
			lastDev := deviations[len(deviations)-1]
			if lastDev.SegmentAfter != nil {
				result.FinalIndex = lastDev.SegmentAfter.EndIndex
				result.FinalPrice = lastDev.SegmentAfter.Top
				if lastDev.Direction == DirDown {
					result.FinalPrice = lastDev.SegmentAfter.Bottom
				}
			}
		}

		if i < len(provider.Levels)-1 {
			// 更新最终位置为当前级别确认的位置
			lastDev := deviations[len(deviations)-1]
			if lastDev.SegmentAfter != nil {
				result.FinalIndex = lastDev.SegmentAfter.EndIndex
				if lastDev.Direction == DirUp {
					result.FinalPrice = lastDev.SegmentAfter.Top
				} else {
					result.FinalPrice = lastDev.SegmentAfter.Bottom
				}
			}
		}
	}

	// 计算精度
	if len(result.Levels) > 0 {
		result.Accuracy = float64(len(result.Levels)) / float64(len(provider.Levels))
	}

	return result
}

// CalculateMACD 包装 talib.MACD 调用，方便外部使用。
func CalculateMACD(close []float64, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, hist []float64, err error) {
	result, err := talib.MACD(close, fastPeriod, slowPeriod, signalPeriod)
	if err != nil {
		return nil, nil, nil, err
	}
	return result.MACD, result.Signal, result.Histogram, nil
}

// trendTypeToDir 将 TrendType 转换为对应的 Direction。
func trendTypeToDir(t TrendType) Direction {
	switch t {
	case TrendUp:
		return DirUp
	case TrendDown:
		return DirDown
	default:
		return DirNone
	}
}

// findSegIndex 在 segments 切片中查找与指定线段匹配的索引。
// 使用业务字段（起止索引 + 方向）而非指针比较，避免切片复制后指针失效。
func findSegIndex(segments []Segment, seg *Segment) int {
	if seg == nil || len(segments) == 0 {
		return -1
	}
	for i := range segments {
		if segments[i].StartIndex == seg.StartIndex &&
			segments[i].EndIndex == seg.EndIndex &&
			segments[i].Direction == seg.Direction {
			return i
		}
	}
	return -1
}
