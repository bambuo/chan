package chanlun

import (
	"fmt"
	"sync"
)

// ──────────────────────────────────────────────
// StreamEngine: O(1) 增量缠论引擎
// ──────────────────────────────────────────────
//
// StreamEngine 是有状态的增量引擎，内部使用链表维护
// 已合并 K 线、笔、线段的增量状态。每根新 K 线到达后，
// 只需 O(1) 操作即可更新全量分析结果。
//
// 底层核心算法（MergeKlines/FindFractals/BuildBis/...）
// 保持函数式不变，仅在 Init 全量初始化时调用。
//
// 使用方式：
//
//	engine, _ := NewStreamEngine(DefaultConfig())
//	engine.Init(historicalKlines)       // 全量初始化
//	for _, k := range realtimeKlines {
//	    inc := engine.AddKline(k)       // O(1) 增量更新
//	    // 处理 inc.NewSignals 等增量变化
//	}
//	snapshot := engine.Snapshot()       // 获取全量快照

// StreamEngine 是 O(1) 增量缠论引擎。
type StreamEngine struct {
	config Config
	mu     sync.RWMutex

	// 全量原始 K 线（仅追加）
	rawKlines []Kline

	// 已合并 K 线链表（双向链表）
	mergedHead  *mergedNode
	mergedTail  *mergedNode
	mergedCount int

	// 分型追踪：避免重复报告同一分型
	lastFractalMidIndex int // 最近报告的中间节点 mergedCount
	lastFractalType     FractalType

	// 笔链表
	biHead  *biNode
	biTail  *biNode
	biCount int

	// 虚拟笔端状态
	virtualBiSavedEnd *Bi // 虚拟笔端前的真实端点（用于回退）
	hasVirtualBi      bool

	// 最近一个有效分型（用于笔的构建）
	lastConfirmedFractal *Fractal

	// 线段链表
	segHead  *segNode
	segTail  *segNode
	segCount int

	// 高级结构（中枢/走势/背驰/信号，低频变化，用 slice）
	pivots     []Pivot
	trends     []Trend
	deviations []Deviation
	signals    []Signal

	// MACD 增量状态
	macd *macdIncremental

	// 技术指标计算器
	indicators *IndicatorCalculator

	// 最近快照缓存
	lastResult *Result
}

// NewStreamEngine 创建增量引擎实例。
func NewStreamEngine(config Config) (*StreamEngine, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("chanlun stream: %w", err)
	}
	return &StreamEngine{
		config:     config,
		macd:       newMacdIncremental(config.MACDFastPeriod, config.MACDSlowPeriod, config.MACDSignalPeriod),
		indicators: NewIndicatorCalculator(config),
	}, nil
}

// Init 使用历史 K 线进行全量初始化。
// 内部调用 Engine.Process 获取基准结果，然后构建链表状态。
func (s *StreamEngine) Init(klines []Kline) (*Result, error) {
	if err := ValidateKlines(klines); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 使用批量管道获取基准结果
	engine := &Engine{config: s.config}
	result, err := engine.processInternal(klines)
	if err != nil {
		return nil, err
	}

	// 存储原始 K 线
	s.rawKlines = copyKlines(klines)

	// 构建已合并 K 线链表
	s.buildMergedChain(result.MergedKlines)

	// 构建笔链表
	s.buildBiChain(result.Bis)

	// 构建线段链表
	s.buildSegChain(result.MergedBis, result.Segments)

	// 复制高级结构
	s.pivots = copyPivots(result.Pivots)
	s.trends = copyTrends(result.Trends)
	s.deviations = copyDeviations(result.Deviations)
	s.signals = copySignals(result.Signals)

	// 初始化 MACD
	s.macd = newMacdIncremental(s.config.MACDFastPeriod, s.config.MACDSlowPeriod, s.config.MACDSignalPeriod)
	closes := extractClose(klines)
	s.macd.addBatch(closes)

	// 设置分型追踪
	s.lastFractalMidIndex = -1
	if len(result.BiFractals) > 0 {
		lastF := result.BiFractals[len(result.BiFractals)-1]
		s.lastFractalMidIndex = lastF.Index
		s.lastFractalType = lastF.Type
	}

	s.lastResult = result
	return result, nil
}

// AddKline 追加一根新 K 线，O(1) 增量更新。
// 返回本次增量变化。
func (s *StreamEngine) AddKline(k Kline) *IncrementalResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	inc := &IncrementalResult{}

	// 1. 追加到原始 K 线
	s.rawKlines = append(s.rawKlines, k)

	// 2. 增量 MACD
	s.macd.addOne(k.Close)

	// 2b. 增量技术指标计算
	kWithIndicators := s.indicators.AddKline(k)

	// 3. 包含处理
	merged := s.tryMergeKline(kWithIndicators)
	if !merged {
		inc.NewMergedKlines = []Kline{s.mergedTail.Kline}
	}

	// 4. 分型检测（仅当新 K 线未被合并时）
	if !merged {
		fractal := s.updateFractalState()
		if fractal != nil && fractal.Index != s.lastFractalMidIndex {
			s.lastFractalMidIndex = fractal.Index
			s.lastFractalType = fractal.Type
			inc.NewFractal = fractal

			// 5. 笔的更新
			s.processNewFractal(fractal, inc)
		}
	}

	// 6. 中枢/走势/背驰/信号增量更新
	if inc.NewSegment != nil {
		s.updatePivotsAndTrends(inc)
	}

	// 7. 构建快照
	s.lastResult = s.buildSnapshot()
	inc.Snapshot = s.lastResult

	return inc
}

// Snapshot 获取当前全量状态快照。
func (s *StreamEngine) Snapshot() *Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastResult != nil {
		return s.lastResult
	}
	return s.buildSnapshot()
}

// processNewFractal 处理新分型，尝试构建/确认笔。
func (s *StreamEngine) processNewFractal(f *Fractal, inc *IncrementalResult) {
	if s.biTail == nil {
		// 第一笔的第一个分型
		s.lastConfirmedFractal = f
		return
	}

	// 同向分型：取更极端的
	if s.lastConfirmedFractal != nil && f.Type == s.lastConfirmedFractal.Type {
		if f.Type == TopFractal && f.High > s.lastConfirmedFractal.High {
			s.lastConfirmedFractal = f
			// 如果当前笔是向上的且未确认，更新虚拟笔端
			if s.biTail != nil && s.biTail.Bi.Direction == DirUp && !s.biTail.isSure {
				s.updateVirtualBiEnd(f, inc)
			}
		} else if f.Type == BottomFractal && f.Low < s.lastConfirmedFractal.Low {
			s.lastConfirmedFractal = f
			// 如果当前笔是向下的且未确认，更新虚拟笔端
			if s.biTail != nil && s.biTail.Bi.Direction == DirDown && !s.biTail.isSure {
				s.updateVirtualBiEnd(f, inc)
			}
		}
		return
	}

	// 反向分型：检查是否可以成笔
	if s.lastConfirmedFractal != nil {
		// 检查间隔
		gap := independentKCountBetweenFractals(*s.lastConfirmedFractal, *f)
		if gap >= s.config.BiMinKLineCount {
			// 确认上一笔
			s.confirmCurrentBi(inc)

			// 开始新笔
			newBi := buildBiFromRange(s.rawKlines, *s.lastConfirmedFractal, *f,
				directionFromFractals(s.lastConfirmedFractal, f))

			// 新笔标准检查
			if s.config.BiMinKLineCount >= 5 && s.config.NewBiMinPriceRatio > 0 {
				avgPrice := (newBi.StartPrice + newBi.EndPrice) / 2
				if avgPrice > 0 && newBi.Length/avgPrice < s.config.NewBiMinPriceRatio {
					s.lastConfirmedFractal = f
					return
				}
			}

			node := &biNode{
				Bi:     newBi,
				pre:    s.biTail,
				isSure: false, // 新笔初始未确认
			}
			if s.biTail != nil {
				s.biTail.next = node
			} else {
				s.biHead = node
			}
			s.biTail = node
			s.biCount++

			inc.NewBi = &newBi
			s.lastConfirmedFractal = f

			// 尝试更新线段
			s.processNewBi(node, inc)
			return
		}
	}

	// 间隔不够，更新最近分型
	s.lastConfirmedFractal = f
}

// directionFromFractals 根据起始和结束分型确定笔方向。
func directionFromFractals(start, end *Fractal) Direction {
	if start.Type == BottomFractal && end.Type == TopFractal {
		return DirUp
	}
	if start.Type == TopFractal && end.Type == BottomFractal {
		return DirDown
	}
	return DirNone
}

// confirmCurrentBi 确认当前最后一笔。
func (s *StreamEngine) confirmCurrentBi(inc *IncrementalResult) {
	if s.biTail == nil {
		return
	}
	if s.biTail.virtual {
		// 恢复虚拟笔端再确认
		s.restoreVirtualBiEnd()
	}
	s.biTail.isSure = true
}

// updatePivotsAndTrends 基于新线段更新中枢和走势。
func (s *StreamEngine) updatePivotsAndTrends(inc *IncrementalResult) {
	if inc.NewSegment == nil {
		return
	}

	newSeg := *inc.NewSegment

	// 尝试延伸最后一个中枢
	if len(s.pivots) > 0 {
		lastPivot := &s.pivots[len(s.pivots)-1]
		if lastPivot.State != PivotDestroyed {
			if newSeg.Top >= lastPivot.ZD && newSeg.Bottom <= lastPivot.ZG {
				// 在中枢区间内：延伸
				lastPivot.Segments = append(lastPivot.Segments, newSeg)
				lastPivot.OverlapCount++
				lastPivot.EndIndex = newSeg.EndIndex
				if newSeg.Top > lastPivot.GG {
					lastPivot.GG = newSeg.Top
				}
				if newSeg.Bottom < lastPivot.DD {
					lastPivot.DD = newSeg.Bottom
				}
				if lastPivot.OverlapCount >= 9 {
					lastPivot.State = PivotExpanded
					lastPivot.Level++
				} else if lastPivot.State == PivotFormed {
					lastPivot.State = PivotExtending
				}
				pivotCopy := *lastPivot
				inc.UpdatedPivot = &pivotCopy
				return
			}
		}
	}

	// 尝试形成新中枢（需要最近 3 个线段有重叠）
	segSlice := s.collectSegments()
	if len(segSlice) >= 3 {
		n := len(segSlice)
		s0, s1, s2 := segSlice[n-3], segSlice[n-2], segSlice[n-1]
		overlapHigh := min(min(s0.Top, s1.Top), s2.Top)
		overlapLow := max(max(s0.Bottom, s1.Bottom), s2.Bottom)

		if overlapHigh >= overlapLow {
			pivotDir := determinePivotDirection(s0, s1, s2)
			zg, zd := calcZGZD(s0, s1, s2, pivotDir)
			gg := max(max(s0.Top, s1.Top), s2.Top)
			dd := min(min(s0.Bottom, s1.Bottom), s2.Bottom)

			pivot := Pivot{
				StartIndex:   s0.StartIndex,
				EndIndex:     s2.EndIndex,
				ZG:           zg,
				ZD:           zd,
				GG:           gg,
				DD:           dd,
				Segments:     []Segment{s0, s1, s2},
				OverlapCount: 3,
				Level:        1,
				SourceLevel:  "segment",
				IsProxy:      true,
				State:        PivotFormed,
				Direction:    pivotDir,
			}
			s.pivots = append(s.pivots, pivot)
			pivotCopy := pivot
			inc.NewPivot = &pivotCopy
		}
	}
}

// collectSegments 从链表收集所有线段。
func (s *StreamEngine) collectSegments() []Segment {
	if s.segHead == nil {
		return nil
	}
	result := make([]Segment, 0, s.segCount)
	for node := s.segHead; node != nil; node = node.next {
		result = append(result, node.Segment)
	}
	return result
}

// ──────────────────────────────────────────────
// 链表构建辅助（Init 使用）
// ──────────────────────────────────────────────

func (s *StreamEngine) buildMergedChain(merged []Kline) {
	s.mergedHead = nil
	s.mergedTail = nil
	s.mergedCount = 0
	for _, k := range merged {
		node := &mergedNode{Kline: k, pre: s.mergedTail}
		if s.mergedTail != nil {
			s.mergedTail.next = node
		} else {
			s.mergedHead = node
		}
		s.mergedTail = node
		s.mergedCount++
	}
}

func (s *StreamEngine) buildBiChain(bis []Bi) {
	s.biHead = nil
	s.biTail = nil
	s.biCount = 0
	for _, b := range bis {
		node := &biNode{Bi: b, pre: s.biTail, isSure: true}
		if s.biTail != nil {
			s.biTail.next = node
		} else {
			s.biHead = node
		}
		s.biTail = node
		s.biCount++
	}
}

func (s *StreamEngine) buildSegChain(mergedBis []MergedBi, segments []Segment) {
	s.segHead = nil
	s.segTail = nil
	s.segCount = 0
	for _, seg := range segments {
		node := &segNode{Segment: seg, pre: s.segTail}
		// 构建特征序列缓存
		node.featureCache = buildFeatureSeq(
			s.findMergedBisForSeg(mergedBis, seg), seg.Direction)
		if s.segTail != nil {
			s.segTail.next = node
		} else {
			s.segHead = node
		}
		s.segTail = node
		s.segCount++
	}
}

// findMergedBisForSeg 找到属于指定线段的笔列表。
func (s *StreamEngine) findMergedBisForSeg(mergedBis []MergedBi, seg Segment) []MergedBi {
	result := make([]MergedBi, 0)
	for _, mb := range mergedBis {
		if mb.StartIndex >= seg.StartIndex && mb.EndIndex <= seg.EndIndex {
			result = append(result, mb)
		}
	}
	return result
}

// buildSnapshot 从链表状态构建全量 Result。
func (s *StreamEngine) buildSnapshot() *Result {
	r := &Result{}

	// 已合并 K 线
	r.MergedKlines = make([]Kline, 0, s.mergedCount)
	for node := s.mergedHead; node != nil; node = node.next {
		r.MergedKlines = append(r.MergedKlines, node.Kline)
	}

	// 分型（从已合并 K 线重新检测）
	if len(r.MergedKlines) >= 3 {
		r.Fractals = FindFractals(r.MergedKlines, 1)
		r.BiFractals = FilterFractalsForBi(r.Fractals, s.config.BiMinKLineCount)
	}

	// 笔
	r.Bis = make([]Bi, 0, s.biCount)
	for node := s.biHead; node != nil; node = node.next {
		r.Bis = append(r.Bis, node.Bi)
	}

	// MergedBis（简化：每笔独立）
	r.MergedBis = make([]MergedBi, len(r.Bis))
	for i, b := range r.Bis {
		r.MergedBis[i] = MergedBi{Bi: b, OriginalCount: 1}
	}

	// 线段
	r.Segments = s.collectSegments()

	// 高级结构
	r.Pivots = copyPivots(s.pivots)
	r.Trends = copyTrends(s.trends)
	r.Deviations = copyDeviations(s.deviations)
	r.Signals = copySignals(s.signals)

	return r
}
