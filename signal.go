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
	signals := make([]Signal, 0)

	// 1. 从背驰检测第一类买卖点
	for _, dev := range deviations {
		sig := detectFirstPoint(dev)
		if sig != nil {
			signals = append(signals, *sig)
		}
	}

	// 2. 从中枢破坏检测第三类买卖点
	for _, p := range pivots {
		if p.State == PivotDestroyed && len(p.Segments) > 0 {
			lastSeg := p.Segments[len(p.Segments)-1]
			sig := detectThirdPoint(lastSeg, p)
			if sig != nil {
				signals = append(signals, *sig)
			}
		}
	}

	// 3. 从一买/一卖后的次级别走势检测第二类买卖点
	for _, trend := range trends {
		if trend.IsComplete && len(trend.Pivots) > 0 {
			sigs := detectSecondPoints(trend, segments, deviations)
			signals = append(signals, sigs...)
		}
	}

	// 4. 去重（同一位置同类型信号只保留第一个）
	signals = dedupSignals(signals)

	// 5. 检测买卖点转化和合并
	signals = detectMergedSignals(signals, pivots)

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
func detectThirdPoint(lastSeg Segment, pivot Pivot) *Signal {
	sigType := BuyPoint3
	if lastSeg.Direction == DirDown {
		sigType = SellPoint3
	}

	price := lastSeg.Top
	if lastSeg.Direction == DirDown {
		price = lastSeg.Bottom
	}

	return &Signal{
		Type:     sigType,
		Level:    "本级别",
		Index:    lastSeg.EndIndex,
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
					enhanced[i].Strength = maxFloat(enhanced[i].Strength+0.3, 1.0)
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

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
