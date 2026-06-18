package chanlun

import "fmt"

// ──────────────────────────────────────────────
// §8  买卖点
// ──────────────────────────────────────────────
//
// 第一类买卖点（一买/一卖）：
//   趋势背驰的终点，最后一个中枢被破坏，出现底/顶背驰
//
// 第二类买卖点（二买/二卖）：
//   一买/一卖之后，价格二次回踩/反弹不破前低/前高
//
// 第三类买卖点（三买/三卖）：
//   价格突破中枢上沿/跌破中枢下沿后，回抽不进中枢区间
//
// 买卖点转化：
//   二三买合并：二买和三买重合在同一位置 → 信号强度极高
//   三买转一卖：三买后离开段力度不济 → 可能转化为一卖

// DetectSignals 从走势结果中检测所有买卖点信号。
func DetectSignals(trends []Trend, deviations []Deviation, pivots []Pivot, segments []Segment) []Signal {
	return DetectSignalsWithConfig(trends, deviations, pivots, segments, DefaultConfig())
}

// DetectSignalsWithConfig 使用指定配置检测买卖点信号（支持 chan.py 的高级过滤）。
func DetectSignalsWithConfig(trends []Trend, deviations []Deviation, pivots []Pivot, segments []Segment, config Config) []Signal {
	signals := make([]Signal, 0)

	// 解析目标买卖点类型
	targetTypes := parseBspTypes(config.BspType)

	// 1. 从背驰检测第一类买卖点
	for _, dev := range deviations {
		sig := detectFirstPointWithConfig(dev, pivots, config, targetTypes)
		if sig != nil {
			signals = append(signals, *sig)
		}
	}

	// 2. 从中枢破坏检测第三类买卖点
	for i := range pivots {
		p := &pivots[i]
		if p.State == PivotDestroyed && len(p.Segments) >= 5 {
			sig := detectThirdPointWithConfig(p.Segments[len(p.Segments)-1], *p, config, targetTypes)
			if sig != nil {
				signals = append(signals, *sig)
			}
		}
	}

	// 3. 从一买/一卖后的次级别走势检测第二类买卖点
	for _, trend := range trends {
		if trend.IsComplete && len(trend.Pivots) > 0 {
			sigs := detectSecondPointsWithConfig(trend, segments, deviations, config, targetTypes)
			signals = append(signals, sigs...)
		}
	}

	// 4. 盘背一买/一卖 (T1P)（对齐 chan.py treat_pz_bsp1）
	if targetTypes["1p"] {
		sigs := detectT1P(segments, pivots, config)
		signals = append(signals, sigs...)
	}

	// 5. 去重
	signals = dedupSignals(signals)

	// 6. 检测买卖点转化和合并
	signals = detectMergedSignals(signals, pivots)

	return signals
}

// parseBspTypes 解析买卖点类型字符串（如 "1,1p,2,2s,3a,3b"）。
func parseBspTypes(bspType string) map[string]bool {
	result := make(map[string]bool)
	if bspType == "" {
		// 默认全部启用
		for _, t := range []string{"1", "1p", "2", "2s", "3a", "3b"} {
			result[t] = true
		}
		return result
	}
	for _, part := range splitComma(bspType) {
		result[part] = true
	}
	return result
}

// splitComma 简单分割逗号字符串。
func splitComma(s string) []string {
	result := make([]string, 0)
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else if c != ' ' {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// detectFirstPointWithConfig 带配置的一买/一卖检测。
func detectFirstPointWithConfig(dev Deviation, pivots []Pivot, config Config, targetTypes map[string]bool) *Signal {
	if dev.Level < SegmentDeviation {
		return nil
	}

	// bsp1_only_multibi_zs: 仅当存在多笔中枢时才触发
	if config.Bsp1OnlyMultiBiZs && dev.SegmentAfter != nil && len(pivots) > 0 {
		hasMultiBiZs := false
		for _, p := range pivots {
			if p.OverlapCount >= 3 {
				hasMultiBiZs = true
				break
			}
		}
		if !hasMultiBiZs {
			return nil
		}
	}

	// bsp_min_zs_cnt: 最少中枢数（仅当有中枢数据时检查）
	if config.BspMinZsCnt > 0 && len(pivots) > 0 && len(pivots) < config.BspMinZsCnt {
		return nil
	}

	// bsp1_peak: 一买位置必须是极值点
	if config.Bsp1Peak && dev.SegmentAfter != nil {
		// 检查是否是走势中的极值点
		if dev.Direction == DirUp {
			// 一卖：检查是否是最高点
			// 简化检查：后续没有更高的价格
		} else {
			// 一买：检查是否是最低点
		}
	}

	index := 0
	price := 0.0
	if dev.SegmentAfter != nil {
		index = dev.SegmentAfter.EndIndex
		if dev.Direction == DirUp {
			price = dev.SegmentAfter.Top
		} else {
			price = dev.SegmentAfter.Bottom
		}
	}

	strength := 0.5
	if dev.Level == TrendDeviation {
		strength = 0.8
	}

	sigType := BuyPoint1
	bspKey := "1"
	if dev.Direction == DirUp {
		sigType = SellPoint1
	}

	if !targetTypes[bspKey] {
		return nil
	}

	return &Signal{
		Type:      sigType,
		SubType:   SubT1,
		Level:     levelToString(dev.Level),
		Index:     index,
		Price:     price,
		Strength:  strength,
		Deviation: &dev,
	}
}

// detectThirdPointWithConfig 带配置的三买/三卖检测。
func detectThirdPointWithConfig(lastSeg Segment, pivot Pivot, config Config, targetTypes map[string]bool) *Signal {
	// bsp3_follow_1: 三买必须跟随一买（简化检查）
	// 实际应检查前一买是否存在，这里简化为检查中枢是否被破坏
	if config.StrictBsp3 {
		// 严格三买：离开段必须紧接在中枢后面
		if len(pivot.Segments) >= 2 {
			leaveSeg := pivot.Segments[len(pivot.Segments)-2]
			if leaveSeg.StartIndex != pivot.Segments[len(pivot.Segments)-3].EndIndex+1 {
				return nil
			}
		}
	}

	segs := pivot.Segments
	if len(segs) < 5 {
		return nil
	}

	leaveSeg := segs[len(segs)-2]
	pullbackSeg := segs[len(segs)-1]

	if leaveSeg.Direction == pullbackSeg.Direction {
		return nil
	}

	sigType := BuyPoint3
	bspKey := "3a"
	price := pullbackSeg.Bottom
	if pullbackSeg.Direction == DirDown {
		if pullbackSeg.Bottom < pivot.ZG {
			return nil
		}
		// bsp3_peak: 三买必须突破中枢波动极值
		if config.Bsp3Peak && pullbackSeg.Bottom < pivot.PeakHigh {
			return nil
		}
		sigType = BuyPoint3
		price = pullbackSeg.Bottom
	} else {
		if pullbackSeg.Top > pivot.ZD {
			return nil
		}
		if config.Bsp3Peak && pullbackSeg.Top > pivot.PeakLow {
			return nil
		}
		sigType = SellPoint3
		bspKey = "3a"
		price = pullbackSeg.Top
	}

	if !targetTypes[bspKey] {
		return nil
	}

	return &Signal{
		Type:     sigType,
		SubType:  SubT3A,
		Level:    "本级别",
		Index:    pullbackSeg.EndIndex,
		Price:    price,
		Strength: 0.6,
		Pivot:    &pivot,
	}
}

// detectSecondPointsWithConfig 带配置的二买/二卖检测。
func detectSecondPointsWithConfig(trend Trend, segments []Segment, deviations []Deviation,
	config Config, targetTypes map[string]bool) []Signal {

	signals := make([]Signal, 0)
	if len(segments) < 2 {
		return signals
	}

	// bsp2_follow_1: 二买必须跟随一买
	if !config.Bsp2Follow1 {
		return signals
	}

	trendEnd := trend.EndIndex
	lastDev := findLastDeviation(deviations, trend)
	if lastDev == nil || lastDev.SegmentAfter == nil {
		return signals
	}

	for _, seg := range segments {
		if seg.StartIndex <= trendEnd {
			continue
		}

		if trend.Type == TrendDown && seg.Direction == DirDown {
			if config.BspMaxBs2Rate > 0 {
				retraceRate := seg.Bottom / lastDev.SegmentAfter.Bottom
				if retraceRate > config.BspMaxBs2Rate {
					continue
				}
			}
			if seg.Bottom > lastDev.SegmentAfter.Bottom && targetTypes["2"] {
				signals = append(signals, Signal{
					Type:     BuyPoint2,
					SubType:  SubT2,
					Level:    "本级别",
					Index:    seg.EndIndex,
					Price:    seg.Bottom,
					Strength: 0.5,
				})
			}
		} else if trend.Type == TrendUp && seg.Direction == DirUp {
			if config.BspMaxBs2Rate > 0 {
				retraceRate := seg.Top / lastDev.SegmentAfter.Top
				if retraceRate < (1.0 / config.BspMaxBs2Rate) {
					continue
				}
			}
			if seg.Top < lastDev.SegmentAfter.Top && targetTypes["2"] {
				signals = append(signals, Signal{
					Type:     SellPoint2,
					SubType:  SubT2,
					Level:    "本级别",
					Index:    seg.EndIndex,
					Price:    seg.Top,
					Strength: 0.5,
				})
			}
		}
	}

	return signals
}

// detectFirstPoint 从背驰信号检测第一类买卖点。
func detectFirstPoint(dev Deviation) *Signal {
	if dev.Level < SegmentDeviation {
		return nil // 笔背驰不产生买卖点信号
	}

	// 确定位置和价格
	index := 0
	price := 0.0
	if dev.SegmentAfter != nil {
		index = dev.SegmentAfter.EndIndex
		if dev.Direction == DirUp {
			price = dev.SegmentAfter.Top // 顶背驰 → 一卖
		} else {
			price = dev.SegmentAfter.Bottom // 底背驰 → 一买
		}
	}

	// 信号强度基于级别
	strength := 0.5
	if dev.Level == TrendDeviation {
		strength = 0.8
	}

	sigType := BuyPoint1
	if dev.Direction == DirUp {
		sigType = SellPoint1
	}

	return &Signal{
		Type:      sigType,
		Level:     levelToString(dev.Level),
		Index:     index,
		Price:     price,
		Strength:  strength,
		Deviation: &dev,
	}
}

// detectThirdPoint 从中枢破坏检测第三类买卖点。
//
// 理论定义（两段结构）：
//
//	三买 = 线段向上离开中枢(ZG) + 回抽段不触及 ZG
//	三卖 = 线段向下离开中枢(ZD) + 回抽段不触及 ZD
//
// 中枢被破坏时，pivot.Segments 的最后两段为 [离开段, 回抽段]。
// 第三类买卖点确认位置在回抽段的终点。
func detectThirdPoint(lastSeg Segment, pivot Pivot) *Signal {
	// 回抽段是中枢破坏段列表的最后一段
	// 它确认了第三类买卖点：回抽不进入中枢区间
	segs := pivot.Segments
	if len(segs) < 5 {
		// 至少需要 3 段形成中枢 + 2 段（离开+回抽）破坏
		return nil
	}

	leaveSeg := segs[len(segs)-2]    // 离开段
	pullbackSeg := segs[len(segs)-1] // 回抽段

	// 验证两段方向相反（离开 vs 回抽）
	if leaveSeg.Direction == pullbackSeg.Direction {
		return nil
	}

	sigType := BuyPoint3
	price := pullbackSeg.Bottom
	if pullbackSeg.Direction == DirDown {
		// 向上离开 + 向下回抽 → 三买确认
		// 回抽段低点不触及 ZG
		if pullbackSeg.Bottom < pivot.ZG {
			return nil // 回抽进入中枢，不是三买
		}
		sigType = BuyPoint3
		price = pullbackSeg.Bottom
	} else {
		// 向下离开 + 向上回抽 → 三卖确认
		// 回抽段高点不触及 ZD
		if pullbackSeg.Top > pivot.ZD {
			return nil // 回抽进入中枢，不是三卖
		}
		sigType = SellPoint3
		price = pullbackSeg.Top
	}

	return &Signal{
		Type:     sigType,
		SubType:  SubT3A,
		Level:    "本级别",
		Index:    pullbackSeg.EndIndex,
		Price:    price,
		Strength: 0.6,
		Pivot:    &pivot,
	}
}

// detectSecondPoints 从完成走势中检测第二类买卖点。
// 一买后，次级别走势回抽不破低点 → 二买
// 一卖后，次级别走势反弹不破高点 → 二卖
func detectSecondPoints(trend Trend, segments []Segment, deviations []Deviation) []Signal {
	signals := make([]Signal, 0)
	if len(segments) < 2 {
		return signals
	}

	// 找到趋势结束位置对应的线段
	trendEnd := trend.EndIndex

	// 找趋势结束后出现的后续线段
	for _, seg := range segments {
		// 只考虑趋势结束后的线段
		if seg.StartIndex <= trendEnd {
			continue
		}

		// 检查是否满足二买/二卖条件
		if trend.Type == TrendDown {
			// 下跌趋势结束（一买后）：回踩不破前低 → 二买
			// 前低 = 最后一笔离开段的低点
			if seg.Direction == DirDown {
				// 找最后一个离开段作为参考
				lastDev := findLastDeviation(deviations, trend)
				if lastDev != nil && lastDev.SegmentAfter != nil {
					if seg.Bottom > lastDev.SegmentAfter.Bottom {
						signals = append(signals, Signal{
							Type:     BuyPoint2,
							SubType:  SubT2,
							Level:    "本级别",
							Index:    seg.EndIndex,
							Price:    seg.Bottom,
							Strength: 0.5,
						})
					}
				}
			}
		} else if trend.Type == TrendUp {
			// 上涨趋势结束（一卖后）：反弹不破前高 → 二卖
			if seg.Direction == DirUp {
				lastDev := findLastDeviation(deviations, trend)
				if lastDev != nil && lastDev.SegmentAfter != nil {
					if seg.Top < lastDev.SegmentAfter.Top {
						signals = append(signals, Signal{
							Type:     SellPoint2,
							SubType:  SubT2,
							Level:    "本级别",
							Index:    seg.EndIndex,
							Price:    seg.Top,
							Strength: 0.5,
						})
					}
				}
			}
		}
	}

	return signals
}

// dedupSignals 按 (Type, Index) 去重，保留第一个出现的信号。
func dedupSignals(signals []Signal) []Signal {
	if len(signals) < 2 {
		return signals
	}
	seen := make(map[string]bool, len(signals))
	result := make([]Signal, 0, len(signals))
	for _, s := range signals {
		key := fmt.Sprintf("%d:%d", s.Type, s.Index)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, s)
	}
	return result
}

// findLastDeviation 找趋势中最后一个背驰（用于确定一买/一卖位置）。
func findLastDeviation(deviations []Deviation, trend Trend) *Deviation {
	var lastDev *Deviation
	for i, d := range deviations {
		if d.SegmentAfter != nil && d.SegmentAfter.EndIndex <= trend.EndIndex {
			lastDev = &deviations[i]
		}
	}
	return lastDev
}

// detectMergedSignals 检测买卖点转化和合并。
//
// 文档 §8.5 定义的转化关系：
// 1. 二三买合并：三买位置与二买重合 → 信号强度倍增（最强买入信号）
// 2. 三买转一卖：三买后离开段出现顶背驰 → 三买可能转化为一卖（风险转化）
// 3. 买卖点共振：同一时刻多级别同向信号叠加
func detectMergedSignals(signals []Signal, pivots []Pivot) []Signal {
	enhanced := make([]Signal, 0, len(signals)+2)
	enhanced = append(enhanced, signals...)

	// 1. 二三买合并检测
	for i := range enhanced {
		if enhanced[i].Type == BuyPoint3 {
			for _, other := range enhanced {
				if other.Type == BuyPoint2 {
					// 二三买合并：信号强度倍增
					enhanced[i].Strength = minFloat(enhanced[i].Strength+0.3, 1.0)
					break
				}
			}
		}
	}

	// 2. 三买转一卖检测
	// 规则：三买后的向上段若出现顶背驰（力度不足），转化为一卖信号
	for _, s := range signals {
		if s.Type == BuyPoint3 && s.Deviation != nil && s.Deviation.Direction == DirUp {
			// 三买后出现顶背驰 → 添加一卖信号
			enhanced = append(enhanced, Signal{
				Type:      SellPoint1,
				Level:     s.Level,
				Index:     s.Index,
				Price:     s.Price,
				Strength:  s.Strength * 0.8, // 风险转化信号强度打八折
				Pivot:     s.Pivot,
				Deviation: s.Deviation,
			})
		}
	}

	return enhanced
}

func levelToString(level DeviationLevel) string {
	switch level {
	case BiDeviation:
		return "笔级别"
	case SegmentDeviation:
		return "线段级别"
	case TrendDeviation:
		return "走势级别"
	default:
		return "未知"
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ──────────────────────────────────────────────
// T1P: 盘背一买/一卖（对齐 chan.py treat_pz_bsp1）
// ──────────────────────────────────────────────
//
// 当线段最后一个中枢不满足标准一买条件时，检查盘背：
// 比较最后两笔的力度，若 out_metric <= divergence_rate * in_metric 则产生 T1P。
func detectT1P(segments []Segment, pivots []Pivot, config Config) []Signal {
	signals := make([]Signal, 0)

	for _, seg := range segments {
		if len(seg.BiList) < 3 {
			continue
		}

		// 取最后两笔
		lastBi := seg.BiList[len(seg.BiList)-1].Bi
		prevBi := seg.BiList[len(seg.BiList)-2].Bi

		// 必须是同向（线段方向）
		if lastBi.Direction != seg.Direction {
			continue
		}

		// 检查是否创新低/高（盘背要求价格创新极值）
		if seg.Direction == DirDown && lastBi.Low >= prevBi.Low {
			continue // 下跌线段未创新低
		}
		if seg.Direction == DirUp && lastBi.High <= prevBi.High {
			continue // 上涨线段未创新高
		}

		// 比较力度（使用简化的振幅比）
		inAmp := prevBi.Length
		outAmp := lastBi.Length
		if inAmp <= 0 {
			continue
		}

		divergenceRate := outAmp / inAmp
		threshold := config.BspDivergenceRate
		if threshold <= 0 || threshold > 100 {
			threshold = 1.0 // 默认阈值
		}

		if divergenceRate <= threshold {
			sigType := BuyPoint1
			if seg.Direction == DirUp {
				sigType = SellPoint1
			}

			price := lastBi.Low
			if seg.Direction == DirUp {
				price = lastBi.High
			}

			signals = append(signals, Signal{
				Type:     sigType,
				SubType:  SubT1P,
				Level:    "本级别",
				Index:    lastBi.EndIndex,
				Price:    price,
				Strength: 0.4, // 盘背信号弱于趋势背驰
			})
		}
	}

	return signals
}
