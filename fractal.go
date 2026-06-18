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

// FindFractals 在经包含处理后的 K 线序列上识别所有分型。
// 返回的分型列表已按以下规则处理：
//  1. 相邻同类型分型取最极端的那个（最高顶或最低底）
//  2. 分型之间至少间隔 minGap 根独立 K 线
func FindFractals(candles []Candle, minGap int) []Fractal {
	if len(candles) < 3 {
		return nil
	}

	raw := make([]Fractal, 0)

	// 第一步：扫描所有原始分型
	for i := 1; i < len(candles)-1; i++ {
		prev := candles[i-1]
		mid := candles[i]
		next := candles[i+1]

			// 顶分型：中间 K 线高点最高、低点也最高
			if mid.High > prev.High && mid.High > next.High &&
				mid.Low > prev.Low && mid.Low > next.Low {
				raw = append(raw, Fractal{
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
				raw = append(raw, Fractal{
					Type:     BottomFractal,
					Index:    i,
					High:     mid.High,
					Low:      mid.Low,
					Strength: calcFractalStrength(mid, prev, next),
				})
				continue
			}
	}

	if len(raw) == 0 {
		return nil
	}

	// 第二步：同向相邻分型取极值，同时确保交替出现
	result := make([]Fractal, 0, len(raw))
	result = append(result, raw[0])

	for i := 1; i < len(raw); i++ {
		last := &result[len(result)-1]

		if raw[i].Type == last.Type {
			// 同向分型：取更极端的
			if raw[i].Type == TopFractal && raw[i].High > last.High {
				result[len(result)-1] = raw[i]
			} else if raw[i].Type == BottomFractal && raw[i].Low < last.Low {
				result[len(result)-1] = raw[i]
			}
		} else {
				// 异向分型：检查间隔
				gap := raw[i].Index - last.Index
				if gap < 0 {
					gap = -gap
				}
				gap -= 3 // 分型间的独立 K 线数（两个分型各占 3 根，需扣除）
				if gap >= minGap {
				result = append(result, raw[i])
			}
			// 间隔不足则跳过
		}
	}

	return result
}

// calcFractalStrength 计算分型强度（0~1）。
// 文档 §2.4：
//   - 基础强度 = K 线实体比例（实体/总振幅）
//   - 跳空缺口增强：第三根 K 线若与中间 K 线之间有跳空 → 强度 +0.2
func calcFractalStrength(mid, prev, next Candle) float64 {
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
