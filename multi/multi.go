package multi

import "github.com/bambuo/chan/types"

// LevelResult 存储单个交易所 interval 的分析结果。
type LevelResult struct {
	Interval string        `json:"interval"`
	Klines   []types.Kline `json:"klines,omitempty"`
	Result   *types.Result `json:"result,omitempty"`
}

// MultiLevelResult 包含多级别联立的完整分析结果。
type MultiLevelResult struct {
	Levels     []LevelResult                `json:"levels"`
	Resonance  int                          `json:"resonance"`
	Deviations []types.Deviation            `json:"deviations,omitempty"`
	Signals    []types.Signal               `json:"signals,omitempty"`
	Nesting    *types.IntervalNestingResult `json:"nesting,omitempty"`
}
