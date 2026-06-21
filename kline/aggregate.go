package kline

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bambuo/chan/types"
)

// ── 周期定义 ──

// Period 描述一个 K 线周期。
type Period struct {
	Name    string // 周期名称，如 "5m"
	Minutes int    // 分钟数
	Label   string // 中文标签
}

// KnownPeriods 所有已知周期及其分钟数。
var KnownPeriods = []Period{
	{Name: "1m", Minutes: 1, Label: "1分钟"},
	{Name: "5m", Minutes: 5, Label: "5分钟"},
	{Name: "15m", Minutes: 15, Label: "15分钟"},
	{Name: "30m", Minutes: 30, Label: "30分钟"},
	{Name: "1h", Minutes: 60, Label: "1小时"},
	{Name: "2h", Minutes: 120, Label: "2小时"},
	{Name: "4h", Minutes: 240, Label: "4小时"},
	{Name: "6h", Minutes: 360, Label: "6小时"},
	{Name: "12h", Minutes: 720, Label: "12小时"},
	{Name: "1d", Minutes: 1440, Label: "日线"},
	{Name: "3d", Minutes: 4320, Label: "3日线"},
	{Name: "1w", Minutes: 10080, Label: "周线"},
	{Name: "1mo", Minutes: 43200, Label: "月线"},
}

// PeriodOf 根据名称查找周期定义。
func PeriodOf(name string) (Period, bool) {
	for _, p := range KnownPeriods {
		if p.Name == name {
			return p, true
		}
	}
	return Period{}, false
}

// ParsePeriodMinutes 解析周期字符串为分钟数。
// 支持格式："1m","5m","15m","30m","1h","2h","4h","1d","1w","1mo" 等。
func ParsePeriodMinutes(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	// 先查已知周期表
	if p, ok := PeriodOf(s); ok {
		return p.Minutes, nil
	}
	// 通用解析
	if len(s) < 2 {
		return 0, fmt.Errorf("无法解析周期 %q", s)
	}
	unit := s[len(s)-1:]
	numStr := s[:len(s)-1]
	var n int
	if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("无法解析周期 %q", s)
	}
	switch unit {
	case "m":
		return n, nil
	case "h":
		return n * 60, nil
	case "d":
		return n * 1440, nil
	case "w":
		return n * 10080, nil
	default:
		return 0, fmt.Errorf("不支持的周期单位 %q", unit)
	}
}

// ── 聚合核心 ──

// AggregateKlines 将低级别 K 线序列按目标周期聚合为高级别 K 线。
//
// 参数：
//   - source: 源 K 线序列（必须按时间升序排列）
//   - targetPeriod: 目标周期，如 "5m", "1h", "4h", "1d"
//
// 返回值：
//   - 聚合后的高级别 K 线（按时间升序）
//   - 若无法解析周期或源数据不足，返回错误
func AggregateKlines(source []types.Kline, targetPeriod string) ([]types.Kline, error) {
	if len(source) == 0 {
		return nil, errors.New("源 K 线序列为空")
	}
	periodMinutes, err := ParsePeriodMinutes(targetPeriod)
	if err != nil {
		return nil, fmt.Errorf("聚合失败: %w", err)
	}

	// 确保源 K 线已按时间排序
	// 假设调用方已排好序；若未排序可在此处排序（但为性能考虑不做默认排序）

	// 按周期对齐时间分组聚合
	type buf struct {
		open   float64
		high   float64
		low    float64
		close  float64
		vol    float64
		qVol   float64
		turn   float64
		trade  int64
		count  int64
		hasSet bool
	}

	buckets := make(map[int64]*buf) // 对齐时间戳 → 聚合缓冲区
	var keys []int64                // 保持插入顺序

	for _, k := range source {
		t := alignTime(k.Time, periodMinutes)
		b, ok := buckets[t]
		if !ok {
			b = &buf{}
			buckets[t] = b
			keys = append(keys, t)
		}

		if !b.hasSet {
			b.open = k.Open
			b.low = k.Low
			b.high = k.High
			b.close = k.Close
			b.vol = k.BaseVolume
			b.qVol = k.QuoteVolume
			b.turn = k.Turnover
			b.trade = k.TradeCount
			b.count = 1
			b.hasSet = true
		} else {
			b.high = math.Max(b.high, k.High)
			b.low = math.Min(b.low, k.Low)
			b.close = k.Close // 最后一根的收盘价
			b.vol += k.BaseVolume
			b.qVol += k.QuoteVolume
			b.turn += k.Turnover
			b.trade += k.TradeCount
			b.count++
		}
	}

	// 产出结果
	result := make([]types.Kline, len(keys))
	for i, t := range keys {
		b := buckets[t]
		result[i] = types.Kline{
			Time:        types.DateTime{Time: time.Unix(t, 0).UTC()},
			Open:        b.open,
			High:        b.high,
			Low:         b.low,
			Close:       b.close,
			BaseVolume:  b.vol,
			QuoteVolume: b.qVol,
			Turnover:    b.turn,
			TradeCount:  b.trade,
		}
	}
	return result, nil
}

// alignTime 将时间对齐到周期边界（按分钟）。返回 Unix 秒。
//
// 例：periodMinutes=5 时，12:03 对齐到 12:00，12:05 对齐到 12:05
func alignTime(dt types.DateTime, periodMinutes int) int64 {
	unix := dt.Unix()
	// 按分钟对齐：将秒数除以 (periodMinutes*60) 取整再乘回
	interval := int64(periodMinutes) * 60
	if interval <= 0 {
		interval = 60
	}
	return (unix / interval) * interval
}

// ── 便利函数 ──

// AggregateAll 将低级别 K 线同时聚合为多个目标周期。
// 返回周期名称 → 聚合后 K 线的映射。
func AggregateAll(source []types.Kline, targets ...string) (map[string][]types.Kline, error) {
	result := make(map[string][]types.Kline, len(targets))
	for _, t := range targets {
		r, err := AggregateKlines(source, t)
		if err != nil {
			return nil, fmt.Errorf("聚合 %s 失败: %w", t, err)
		}
		result[t] = r
	}
	return result, nil
}

// MustAggregateKlines 聚合 K 线，失败时 panic。用于测试和确定参数场景。
func MustAggregateKlines(source []types.Kline, targetPeriod string) []types.Kline {
	r, err := AggregateKlines(source, targetPeriod)
	if err != nil {
		panic(err)
	}
	return r
}
