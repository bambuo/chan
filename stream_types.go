package chanlun

// ──────────────────────────────────────────────
// 增量引擎链表节点类型
// ──────────────────────────────────────────────
//
// StreamEngine 使用双向链表维护各类结构的增量状态。
// 每个节点持有值类型 + prev/next 指针，支持 O(1) 追加和尾部回溯。
// 链表仅在尾部追加，不做中间插入或删除。

// mergedNode 是包含处理后 K 线的链表节点。
// 对应 MergeKlines 的输出，每根已合并 K 线对应一个节点。
type mergedNode struct {
	Kline Kline
	pre   *mergedNode
	next  *mergedNode
}

// biNode 是笔的链表节点。
type biNode struct {
	Bi      Bi
	pre     *biNode
	next    *biNode
	isSure  bool // 笔是否已确认（分型成立且后续 K 线验证）
	virtual bool // 是否为虚拟笔端（临时延伸，尚未确认）
}

// segNode 是线段的链表节点。
type segNode struct {
	Segment       Segment
	pre           *segNode
	next          *segNode
	featureCache  []FeatureElement // 增量特征序列缓存（已包含处理）
	gapPending    bool             // Case 2 二次确认状态
	gapFractalIdx int              // 缺口分型中间元素索引
}

// ──────────────────────────────────────────────
// 增量结果
// ──────────────────────────────────────────────

// IncrementalResult 包含一次 AddKline 调用产生的增量变化。
type IncrementalResult struct {
	// 新增的已合并 K 线（0 或 1 根；若被包含则为 0）
	NewMergedKlines []Kline `json:"newMergedKlines,omitempty"`

	// 新增的分型（0 或 1 个）
	NewFractal *Fractal `json:"newFractal,omitempty"`

	// 新增的笔（0 或 1 笔；若笔被确认则产出）
	NewBi *Bi `json:"newBi,omitempty"`

	// 笔端点更新（虚拟笔端或确认更新）
	UpdatedBiEnd *Bi `json:"updatedBiEnd,omitempty"`

	// 新增的线段（0 或 1 段）
	NewSegment *Segment `json:"newSegment,omitempty"`

	// 新增的中枢（0 或 1 个）
	NewPivot *Pivot `json:"newPivot,omitempty"`

	// 更新的中枢状态（延伸/扩展/破坏）
	UpdatedPivot *Pivot `json:"updatedPivot,omitempty"`

	// 新增的背驰信号
	NewDeviations []Deviation `json:"newDeviations,omitempty"`

	// 新增的买卖点信号
	NewSignals []Signal `json:"newSignals,omitempty"`

	// 当前全量快照（便于快速访问）
	Snapshot *Result `json:"-"`
}
