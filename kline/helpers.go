package kline

import "github.com/bambuo/chan/types"

type containResult int

const (
	containCombine containResult = iota
	containUp
	containDown
)

// testContainment 判定 last 与 curr 的包含关系。
// allowTopEqual: 1=顶相等（High 相等）不合并、-1=底相等（Low 相等）不合并、0=标准模式。
//
// 返回值：
//   - containCombine: curr 被 last 包含或包含 last，应执行合并
//   - containUp: curr 在 last 上方（无包含），应保留 curr
//   - containDown: curr 在 last 下方（无包含），应保留 curr
func testContainment(last, curr types.Kline, allowTopEqual int) containResult {
	// 先处理 allowTopEqual 相等边界，避免被后续 >= / <= 分支吞掉。
	// 顶相等（High 相等且 last.Low 严格高于 curr.Low）：allowTopEqual=1 时不合并。
	if allowTopEqual == 1 && last.High == curr.High && last.Low > curr.Low {
		return containDown
	}
	// 底相等（Low 相等且 last.High 严格低于 curr.High）：allowTopEqual=-1 时不合并。
	if allowTopEqual == -1 && last.Low == curr.Low && last.High < curr.High {
		return containUp
	}

	// last 包含 curr（last 区间覆盖 curr）
	if last.High >= curr.High && last.Low <= curr.Low {
		return containCombine
	}
	// curr 包含 last（curr 区间覆盖 last）
	if last.High <= curr.High && last.Low >= curr.Low {
		return containCombine
	}
	// 无包含：curr 完全在 last 上方
	if last.High < curr.High && last.Low < curr.Low {
		return containUp
	}
	// 无包含：curr 完全在 last 下方
	if last.High > curr.High && last.Low > curr.Low {
		return containDown
	}
	return containCombine
}

func updateDirectionFromRaw(prev, curr types.Kline, fallback types.Direction) types.Direction {
	if curr.High > prev.High {
		return types.DirUp
	}
	if curr.Low < prev.Low {
		return types.DirDown
	}
	return fallback
}

func mergePair(a, b types.Kline, dir types.Direction) types.Kline {
	t := a.Time
	if b.Time.After(a.Time.Time) {
		t = b.Time
	}
	m := mergeMeta(a, b, t)
	// 缠论包含处理取极值规则（对齐 docs/K线包含处理算法.md）：
	//   - 方向向上（处于上升段）：High 取较大、Low 取较大（保留上沿）
	//   - 方向向下（处于下降段）：High 取较小、Low 取较小（保留下沿）
	// 当方向无法判定（DirNone）时，按初始上升方向处理。
	if dir == types.DirDown {
		m.High = min(a.High, b.High)
		m.Low = min(a.Low, b.Low)
	} else {
		// DirUp 与 DirNone 均按上升方向取极值，避免丢弃 b 的价格信息。
		m.High = max(a.High, b.High)
		m.Low = max(a.Low, b.Low)
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
