package chanlun

// ──────────────────────────────────────────────
// §2  分型
// ──────────────────────────────────────────────
//
// 分型由连续三根经包含处理后的 K 线客观决定。
//
// 顶分型：中间 K 线的最高点是三根中最高的，最低点也是三根中最高的。
// 底分型：中间 K 线的最低点是三根中最低的，最高点也是三根中最低的。
//
// 重要区分：
//   - 分型的存在：K 线形态一旦满足条件，分型即客观存在
//   - 分型用于笔：在构筑笔时，需满足交替出现规则和间隔要求

// FindFractals 在经包含处理后的 Kline 序列上识别所有客观分型。
// 分型一旦满足三根 K 线的几何关系即客观存在；成笔过滤由 FilterFractalsForBi 处理。
func FindFractals(klines []Kline, minGap int) []Fractal {
	if len(klines) < 3 {
		return nil
	}

	result := make([]Fractal, 0)

	for i := 1; i < len(klines)-1; i++ {
		prev := klines[i-1]
		mid := klines[i]
		next := klines[i+1]

		// 顶分型：中间 K 线高点最高、低点也最高
		if mid.High > prev.High && mid.High > next.High &&
			mid.Low > prev.Low && mid.Low > next.Low {
			result = append(result, Fractal{
				Type:     TopFractal,
				Index:    i,
				High:     mid.High,
				Low:      mid.Low,
				Strength: calcFractalStrength(mid, prev, next),
			})
			continue
		}

		// 底分型：中间 K 线低点最低、高点也最低
		if mid.Low < prev.Low && mid.Low < next.Low &&
			mid.High < prev.High && mid.High < next.High {
			result = append(result, Fractal{
				Type:     BottomFractal,
				Index:    i,
				High:     mid.High,
				Low:      mid.Low,
				Strength: calcFractalStrength(mid, prev, next),
			})
			continue
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// FilterFractalsForBi 将客观分型筛选为可用于成笔的有效顶底分型。
func FilterFractalsForBi(fractals []Fractal, minGap int) []Fractal {
	if len(fractals) == 0 {
		return nil
	}

	result := make([]Fractal, 0, len(fractals))
	result = append(result, fractals[0])

	for i := 1; i < len(fractals); i++ {
		last := &result[len(result)-1]
		curr := fractals[i]

		if curr.Type == last.Type {
			// 同向分型：取更极端的
			if curr.Type == TopFractal && curr.High > last.High {
				result[len(result)-1] = curr
			} else if curr.Type == BottomFractal && curr.Low < last.Low {
				result[len(result)-1] = curr
			}
		} else {
			if independentKCountBetweenFractals(*last, curr) >= minGap {
				result = append(result, curr)
			}
		}
	}

	return result
}

func independentKCountBetweenFractals(a, b Fractal) int {
	left := b.Index - 1
	right := a.Index + 1
	return left - right - 1
}

// calcFractalStrength 计算分型强度（0~1）。
// 文档 §2.4：
//   - 基础强度 = K 线实体比例（实体/总振幅）
//   - 跳空缺口增强：第三根 K 线若与中间 K 线之间有跳空 → 强度 +0.2
func calcFractalStrength(mid, prev, next Kline) float64 {
	// 基础强度：实体占比
	bodyRatio := abs(mid.Close-mid.Open) / (mid.High - mid.Low + 1e-10)

	// 跳空缺口检测：第三根 K 线与中间 K 线之间的跳空
	gapBoost := 0.0
	if (mid.Low > prev.High) || (mid.High < prev.Low) {
		gapBoost = 0.2 // 第一、二根之间有跳空
	}
	if (next.Low > mid.High) || (next.High < mid.Low) {
		gapBoost = 0.2 // 第二、三根之间有跳空
	}

	strength := bodyRatio + gapBoost
	if strength > 1.0 {
		strength = 1.0
	}
	return strength
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
