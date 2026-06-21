package multi

import "github.com/bambuo/chan/types"

// LevelInput 定义单个级别的输入。
type LevelInput struct {
	Interval string        `json:"interval"` // 如 "1m", "5m", "15m", "1h"
	Klines   []types.Kline `json:"klines"`   // 该级别的原始 K 线数据
}

// LevelResult 存储单个交易所 interval 的分析结果。
type LevelResult struct {
	Interval string        `json:"interval"`
	Klines   []types.Kline `json:"klines,omitempty"`
	Result   *types.Result `json:"result,omitempty"`
}

// MultiLevelResult 包含多级别联立的完整分析结果。
// Levels 按时间周期从低到高排列，即 Levels[0] 为最低级别（如 1m）。
type MultiLevelResult struct {
	Levels    []LevelResult                `json:"levels"`
	Resonance int                          `json:"resonance"` // 跨级别总共振数
	Nesting   *types.IntervalNestingResult `json:"nesting,omitempty"`
}

// LevelCount 返回参与分析的级别数量。
func (m *MultiLevelResult) LevelCount() int {
	if m == nil || len(m.Levels) == 0 {
		return 1
	}
	return len(m.Levels)
}

// HighestResult 返回最高级别的分析结果。
func (m *MultiLevelResult) HighestResult() *types.Result {
	if m == nil || len(m.Levels) == 0 {
		return nil
	}
	return m.Levels[len(m.Levels)-1].Result
}

// LowestResult 返回最低级别的分析结果。
func (m *MultiLevelResult) LowestResult() *types.Result {
	if m == nil || len(m.Levels) == 0 {
		return nil
	}
	return m.Levels[0].Result
}

// LevelByInterval 按 interval 名称查找级别结果。
func (m *MultiLevelResult) LevelByInterval(interval string) *LevelResult {
	if m == nil {
		return nil
	}
	for i := range m.Levels {
		if m.Levels[i].Interval == interval {
			return &m.Levels[i]
		}
	}
	return nil
}
