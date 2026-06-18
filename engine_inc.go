package chanlun

// ──────────────────────────────────────────────
// Engine 全量处理（供 StreamEngine.Init 使用）
// ──────────────────────────────────────────────
//
// 原有的窗口重算增量方案（Update 方法）已被 StreamEngine 替代。
// 本文件仅保留 Engine.Process 作为全量批量处理入口，
// 供 StreamEngine.Init 内部调用获取基准结果。
//
// 增量更新请使用 StreamEngine.AddKline。

// Process 全量处理 K 线序列，返回完整分析结果。
// 这是批量处理入口，适合一次性分析历史数据。
// 如需实时增量更新，请使用 StreamEngine。
func (e *Engine) Process(klines []Kline) (*Result, error) {
	return e.processInternal(klines)
}

// ──────────────────────────────────────────────
// 辅助复制函数（供 StreamEngine.Init 构建链表使用）
// ──────────────────────────────────────────────

func copyKlines(src []Kline) []Kline {
	dst := make([]Kline, len(src))
	copy(dst, src)
	return dst
}

func copyFractals(src []Fractal) []Fractal {
	dst := make([]Fractal, len(src))
	copy(dst, src)
	return dst
}

func copyBis(src []Bi) []Bi {
	dst := make([]Bi, len(src))
	copy(dst, src)
	return dst
}

func copyMergedBis(src []MergedBi) []MergedBi {
	dst := make([]MergedBi, len(src))
	copy(dst, src)
	return dst
}

func copySegments(src []Segment) []Segment {
	dst := make([]Segment, len(src))
	copy(dst, src)
	return dst
}

func copyPivots(src []Pivot) []Pivot {
	dst := make([]Pivot, len(src))
	copy(dst, src)
	return dst
}

func copyTrends(src []Trend) []Trend {
	dst := make([]Trend, len(src))
	copy(dst, src)
	return dst
}

func copyDeviations(src []Deviation) []Deviation {
	dst := make([]Deviation, len(src))
	copy(dst, src)
	return dst
}

func copySignals(src []Signal) []Signal {
	dst := make([]Signal, len(src))
	copy(dst, src)
	return dst
}
