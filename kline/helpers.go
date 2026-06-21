package kline

import "github.com/bambuo/chan/types"

type containResult int

const (
	containCombine containResult = iota
	containUp
	containDown
)

func testContainment(last, curr types.Kline, allowTopEqual int) containResult {
	if last.High >= curr.High && last.Low <= curr.Low {
		return containCombine
	}
	if last.High <= curr.High && last.Low >= curr.Low {
		if allowTopEqual == 1 && last.High == curr.High && last.Low > curr.Low {
			return containDown
		}
		if allowTopEqual == -1 && last.Low == curr.Low && last.High < curr.High {
			return containUp
		}
		return containCombine
	}
	if last.High < curr.High && last.Low < curr.Low {
		return containUp
	}
	if last.High > curr.High && last.Low > curr.Low {
		return containDown
	}
	return containCombine
}

func direction(klines []types.Kline) types.Direction {
	for i := len(klines) - 1; i >= 1; i-- {
		prev, curr := klines[i-1], klines[i]
		if !(prev.High <= curr.High && prev.Low >= curr.Low) &&
			!(curr.High <= prev.High && curr.Low >= prev.Low) {
			if curr.High >= prev.High {
				return types.DirUp
			}
			if curr.Low <= prev.Low {
				return types.DirDown
			}
		}
	}
	// 对齐 Python：无法确定方向时默认为 UP（Python 中第一个 CKLine 默认方向为 UP）
	return types.DirUp
}

func mergePair(a, b types.Kline, dir types.Direction) types.Kline {
	t := a.Time
	if b.Time.After(a.Time.Time) {
		t = b.Time
	}
	m := mergeMeta(a, b, t)
	switch dir {
	case types.DirUp:
		m.High = max(a.High, b.High)
		m.Low = max(a.Low, b.Low)
	case types.DirDown:
		m.High = min(a.High, b.High)
		m.Low = min(a.Low, b.Low)
	default:
		m.High = a.High
		m.Low = a.Low
		m.Close = a.Close
	}
	return m
}

func mergeMeta(a, b types.Kline, t types.DateTime) types.Kline {
	return types.Kline{
		Time: t, Open: a.Open, High: a.High, Low: a.Low, Close: b.Close,
		BaseVolume: a.BaseVolume + b.BaseVolume, QuoteVolume: a.QuoteVolume + b.QuoteVolume,
		Turnover: a.Turnover + b.Turnover, TradeCount: a.TradeCount + b.TradeCount,
		RawVolumeUnit:   pick(a.RawVolumeUnit, b.RawVolumeUnit),
		RawTurnoverUnit: pick(a.RawTurnoverUnit, b.RawTurnoverUnit),
	}
}

func pick(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
