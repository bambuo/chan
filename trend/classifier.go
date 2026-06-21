package trend

import (
	"math"

	"github.com/bambuo/chan/types"
)

// ClassifyTrends 从中枢列表分析走势类型。
func ClassifyTrends(pivots []types.Pivot) []types.Trend {
	if len(pivots) == 0 {
		return nil
	}
	var trends []types.Trend
	i := 0
	for i < len(pivots) {
		t, next := classifyFrom(pivots, i)
		if t == nil {
			i++
			continue
		}
		trends = append(trends, *t)
		i = next
	}
	return trends
}

// UpdateTrendsWithDeviations 用背驰检测结果更新走势完成状态。
// 优先使用 TrendIndex 精准匹配（DetectTrendDeviations 已设置），
// 回退到 SegmentAfter 指针匹配（兼容旧调用路径）。
func UpdateTrendsWithDeviations(trends []types.Trend, deviations []types.Deviation) {
	for i := range deviations {
		dev := &deviations[i]
		// 精准匹配：TrendIndex 指向产生该背驰的走势（仅趋势背驰有效，-1 表示未关联）
		if dev.TrendIndex >= 0 && dev.TrendIndex < len(trends) {
			markComplete(&trends[dev.TrendIndex], dev)
			continue
		}
		// 回退匹配：SegmentAfter 指针
		if dev.SegmentAfter != nil {
			for j := range trends {
				if dev.SegmentAfter.EndIndex >= trends[j].StartIndex &&
					dev.SegmentAfter.EndIndex <= trends[j].EndIndex {
					markComplete(&trends[j], dev)
					break
				}
			}
		}
	}
}

func classifyFrom(pivots []types.Pivot, i int) (*types.Trend, int) {
	if i >= len(pivots) {
		return nil, i
	}
	if i == len(pivots)-1 {
		p := pivots[i]
		return &types.Trend{Type: types.RangeOnly, Pivots: []types.Pivot{p},
			StartIndex: p.StartIndex, EndIndex: p.EndIndex}, i + 1
	}
	p0, p1 := pivots[i], pivots[i+1]
	if p1.Direction != p0.Direction {
		return &types.Trend{Type: types.RangeOnly, Pivots: []types.Pivot{p0},
			StartIndex: p0.StartIndex, EndIndex: p0.EndIndex}, i + 1
	}
	if p1.DD > p0.GG {
		t := buildTrend(types.TrendUp, pivots, i)
		return t, i + len(t.Pivots)
	}
	if p1.GG < p0.DD {
		t := buildTrend(types.TrendDown, pivots, i)
		return t, i + len(t.Pivots)
	}
	return &types.Trend{Type: types.RangeOnly, Pivots: []types.Pivot{p0},
		StartIndex: p0.StartIndex, EndIndex: p0.EndIndex}, i + 1
}

func buildTrend(tp types.TrendType, pivots []types.Pivot, start int) *types.Trend {
	t := &types.Trend{Type: tp, Pivots: []types.Pivot{pivots[start]},
		StartIndex: pivots[start].StartIndex, EndIndex: pivots[start].EndIndex}
	for i := start + 1; i < len(pivots); i++ {
		prev := t.Pivots[len(t.Pivots)-1]
		curr := pivots[i]
		if curr.Direction != prev.Direction {
			break
		}
		match := (tp == types.TrendUp && curr.DD > prev.GG) ||
			(tp == types.TrendDown && curr.GG < prev.DD)
		if !match {
			break
		}
		t.Pivots = append(t.Pivots, curr)
		t.EndIndex = curr.EndIndex
	}
	if len(t.Pivots) > 0 {
		lp := t.Pivots[len(t.Pivots)-1]
		if lp.State == types.PivotDestroyed {
			t.CompleteReason = "中枢被第三类买卖点破坏（待背驰确认）"
		}
	}
	return t
}

func markComplete(t *types.Trend, dev *types.Deviation) {
	if t == nil || dev == nil {
		return
	}
	if t.Type == types.TrendUp && dev.Direction == types.DirUp {
		t.IsComplete = true
		t.CompleteReason = "顶背驰确认完成"
	} else if t.Type == types.TrendDown && dev.Direction == types.DirDown {
		t.IsComplete = true
		t.CompleteReason = "底背驰确认完成"
	} else if t.Type == types.RangeOnly && len(t.Pivots) > 0 && t.Pivots[0].State == types.PivotDestroyed {
		t.IsComplete = true
		t.CompleteReason = "第三类买卖点确认盘整完成"
	}
}

// CalcTrendMetrics 计算走势的趋势指标（均值/最高/最低）。
// 每个 Trend 的 Metrics map 存储 result_mean_N, result_max_N, result_min_N。
func CalcTrendMetrics(trends []types.Trend, klines []types.Kline, metrics []int) {
	if len(trends) == 0 || len(klines) == 0 || len(metrics) == 0 {
		return
	}
	closes := klineCloses(klines)
	highs := klineHighs(klines)
	lows := klineLows(klines)

	for i := range trends {
		t := &trends[i]
		if len(t.Pivots) == 0 {
			continue
		}
		start := max(0, t.StartIndex)
		end := min(len(klines)-1, t.EndIndex)
		if end <= start {
			continue
		}
		for _, n := range metrics {
			if n <= 0 {
				continue
			}
			bgn := max(start, end-n+1)
			if t.Metrics == nil {
				t.Metrics = make(map[int]float64)
			}
			sum := 0.0
			cnt := 0
			mx := math.Inf(-1)
			mn := math.Inf(1)
			for j := bgn; j <= end; j++ {
				sum += closes[j]
				if highs[j] > mx {
					mx = highs[j]
				}
				if lows[j] < mn {
					mn = lows[j]
				}
				cnt++
			}
			if cnt > 0 {
				t.Metrics[n] = sum / float64(cnt)
				t.Metrics[n*10+1] = mx
				t.Metrics[n*10+2] = mn
			}
		}
	}
}

func klineCloses(k []types.Kline) []float64 {
	r := make([]float64, len(k))
	for i, c := range k {
		r[i] = c.Close
	}
	return r
}

func klineHighs(k []types.Kline) []float64 {
	r := make([]float64, len(k))
	for i, c := range k {
		r[i] = c.High
	}
	return r
}

func klineLows(k []types.Kline) []float64 {
	r := make([]float64, len(k))
	for i, c := range k {
		r[i] = c.Low
	}
	return r
}
