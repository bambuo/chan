package segment

import "github.com/bambuo/chan/types"

type mergeDir int

const (
	mergeNone mergeDir = 0
	mergeUp   mergeDir = 1
	mergeDown mergeDir = -1
)

type eigenElement struct {
	bis    []types.MergedBi
	high   float64
	low    float64
	dir    mergeDir
	gap    bool
	fx     types.FractalType
	biIdxs []int // 每个 bi 在全局 bis 切片中的索引
}

func newEigenElement(bi types.MergedBi, dir mergeDir, biIdx int) *eigenElement {
	return &eigenElement{
		bis:    []types.MergedBi{bi},
		high:   bi.High,
		low:    bi.Low,
		dir:    dir,
		biIdxs: []int{biIdx},
	}
}

func (e *eigenElement) tryAdd(bi types.MergedBi, biIdx int) mergeDir {
	aContainsB := bi.High <= e.high && bi.Low >= e.low
	bContainsA := bi.High >= e.high && bi.Low <= e.low
	if aContainsB || bContainsA {
		e.bis = append(e.bis, bi)
		e.biIdxs = append(e.biIdxs, biIdx)
		switch e.dir {
		case mergeUp:
			e.high = max(e.high, bi.High)
			e.low = max(e.low, bi.Low)
		case mergeDown:
			e.high = min(e.high, bi.High)
			e.low = min(e.low, bi.Low)
		default:
			e.high = max(e.high, bi.High)
			e.low = min(e.low, bi.Low)
		}
		return e.dir
	}
	if e.dir == mergeUp {
		return mergeDown
	}
	return mergeUp
}

func (e *eigenElement) updateFx(pre, next *eigenElement, excludeIncluded bool) {
	if pre == nil || next == nil {
		return
	}
	if pre.high < e.high && next.high < e.high && pre.low < e.low && next.low < e.low {
		e.fx = types.TopFractal
	} else if pre.high > e.high && next.high > e.high && pre.low > e.low && next.low > e.low {
		e.fx = types.BottomFractal
	} else {
		e.fx = types.FractalNone
	}
	if e.fx == types.TopFractal && pre.high < e.low {
		e.gap = true
	} else if e.fx == types.BottomFractal && pre.low > e.high {
		e.gap = true
	} else {
		e.gap = false
	}
}

func (e *eigenElement) getPeakIdx() int {
	if len(e.bis) == 0 || len(e.biIdxs) == 0 {
		return -1
	}
	// 对齐 Python CEigen.GetPeakBiIdx:
	//   bi_dir=UP → 取最低价最低的笔 (is_high=False), .idx-1
	//   bi_dir=DOWN → 取最高价最高的笔 (is_high=True), .idx-1
	// Go 中 mergeUp 等价于 UP 方向合并 (high=max, low=max)
	var peakBiPos int
	switch e.dir {
	case mergeUp:
		// low 取 max → 找 low 最大的笔 (= 元素 low)
		maxLow := e.low
		peakBiPos = 0
		for i, bi := range e.bis {
			if bi.Low >= maxLow {
				maxLow = bi.Low
				peakBiPos = i
			}
		}
	default: // mergeDown / mergeNone
		// high 取 min → 找 high 最小的笔 (= 元素 high)
		minHigh := e.high
		peakBiPos = 0
		for i, bi := range e.bis {
			if bi.High <= minHigh {
				minHigh = bi.High
				peakBiPos = i
			}
		}
	}
	if peakBiPos >= 0 && peakBiPos < len(e.biIdxs) {
		// Python 返回 .idx-1（峰前一笔），峰笔本身属于下一段
		return e.biIdxs[peakBiPos] - 1
	}
	return e.biIdxs[0] - 1
}

type eigenFXBreakResult int

const (
	eigenNoBreak eigenFXBreakResult = iota
	eigenBreakSure
	eigenBreakTentative
)

type eigenFX struct {
	dir               Direction
	elements          [3]*eigenElement
	mergeDir          mergeDir
	allBis            []types.MergedBi
	actualBreakFlag   bool
	lastEvidenceBi    *types.MergedBi
	lastEvidentBiSure bool
	gapPending        bool
}

type Direction = types.Direction

func newEigenFX(segDir Direction) *eigenFX {
	md := mergeUp
	if segDir == types.DirUp {
		md = mergeDown
	}
	return &eigenFX{
		dir:             segDir,
		mergeDir:        md,
		actualBreakFlag: true,
	}
}

func (fx *eigenFX) add(bi types.MergedBi, allBis []types.MergedBi, biIdx int) (eigenFXBreakResult, int) {
	fx.allBis = allBis
	if fx.elements[0] == nil {
		fx.elements[0] = newEigenElement(bi, fx.mergeDir, biIdx)
		return eigenNoBreak, -1
	}
	if fx.elements[1] == nil {
		combineDir := fx.elements[0].tryAdd(bi, biIdx)
		if combineDir != fx.mergeDir {
			fx.elements[1] = newEigenElement(bi, fx.mergeDir, biIdx)
			if (fx.dir == types.DirUp && fx.elements[1].high < fx.elements[0].high) ||
				(fx.dir == types.DirDown && fx.elements[1].low > fx.elements[0].low) {
				return fx.reset(biIdx)
			}
		}
		return eigenNoBreak, -1
	}
	if fx.elements[2] == nil {
		fx.lastEvidenceBi = &allBis[biIdx]
		fx.lastEvidentBiSure = true
		combineDir := fx.elements[1].tryAdd(bi, biIdx)
		if combineDir == fx.mergeDir {
			return eigenNoBreak, -1
		}
		fx.elements[2] = newEigenElement(bi, combineDir, biIdx)
		breakResult := fx.checkActualBreak(biIdx)
		if breakResult == eigenNoBreak {
			return fx.reset(biIdx)
		}
		fx.elements[1].updateFx(fx.elements[0], fx.elements[2], true)
		isFx := false
		if fx.dir == types.DirUp && fx.elements[1].fx == types.TopFractal {
			isFx = true
		}
		if fx.dir == types.DirDown && fx.elements[1].fx == types.BottomFractal {
			isFx = true
		}
		if !isFx {
			return fx.reset(biIdx)
		}
		peakIdx := fx.elements[1].getPeakIdx()
		if breakResult == eigenBreakTentative {
			return eigenBreakTentative, peakIdx
		}
		return eigenBreakSure, peakIdx
	}
	return eigenNoBreak, -1
}

func (fx *eigenFX) checkActualBreak(biIdx int) eigenFXBreakResult {
	if fx.elements[1] == nil || fx.elements[2] == nil {
		return eigenNoBreak
	}
	ele1 := fx.elements[1]
	ele2 := fx.elements[2]
	if (fx.dir == types.DirUp && ele2.low < ele1.low) ||
		(fx.dir == types.DirDown && ele2.high > ele1.high) {
		return eigenBreakSure
	}
	if biIdx+2 < len(fx.allBis) {
		ele2Bi := fx.allBis[biIdx]
		nextNextBi := fx.allBis[biIdx+2]
		if (fx.dir == types.DirUp && nextNextBi.Low < ele2Bi.Low) ||
			(fx.dir == types.DirDown && nextNextBi.High > ele2Bi.High) {
			fx.lastEvidenceBi = &fx.allBis[biIdx+2]
			fx.lastEvidentBiSure = true
			return eigenBreakSure
		}
		fx.actualBreakFlag = false
		return eigenBreakTentative
	}
	fx.actualBreakFlag = false
	return eigenBreakTentative
}

func (fx *eigenFX) reset(biIdx int) (eigenFXBreakResult, int) {
	fx.clear()
	return eigenNoBreak, -1
}

func (fx *eigenFX) clear() {
	fx.elements[0] = nil
	fx.elements[1] = nil
	fx.elements[2] = nil
	fx.actualBreakFlag = true
}

func (fx *eigenFX) canBeEnd(bis []types.MergedBi, beginIdx int) (shouldEnd, isSure bool) {
	if fx.elements[1] == nil {
		return false, false
	}
	if fx.elements[1].gap {
		peakIdx := fx.getPeakIdx()
		if peakIdx < 0 {
			peakIdx = beginIdx
		}
		biPos := -1
		for i := range bis {
			if bis[i].EndIndex == peakIdx {
				biPos = i
				break
			}
		}
		if biPos < 0 {
			biPos = beginIdx
		}
		return fx.findRevertFx(bis, biPos+1)
	}
	if !fx.actualBreakFlag {
		return true, false
	}
	return true, true
}

func (fx *eigenFX) findRevertFx(bis []types.MergedBi, beginIdx int) (shouldEnd, isSure bool) {
	if beginIdx >= len(bis) {
		return false, false
	}
	firstBiDir := bis[beginIdx].Direction
	subDir := types.DirDown
	if firstBiDir == types.DirDown {
		subDir = types.DirUp
	} else {
		subDir = types.DirDown
	}
	subFX := newEigenFX(subDir)
	for i := beginIdx; i < len(bis); i += 2 {
		bi := bis[i]
		result, _ := subFX.add(bi, bis, i)
		if result == eigenBreakSure || result == eigenBreakTentative {
			subShouldEnd, subIsSure := subFX.canBeEnd(bis, i)
			if !subFX.actualBreakFlag {
				subIsSure = false
			}
			if subShouldEnd || !subIsSure {
				if subFX.elements[2] != nil && len(subFX.elements[2].bis) > 0 {
					lastEvidence := subFX.elements[2].bis[len(subFX.elements[2].bis)-1]
					fx.lastEvidenceBi = &lastEvidence
					fx.lastEvidentBiSure = subIsSure
				}
				if subShouldEnd {
					return true, subIsSure
				}
				return true, false
			}
			_, _ = subFX.reset(i)
		}
	}
	return false, false
}

func (fx *eigenFX) getPeakIdx() int {
	if fx.elements[1] == nil {
		return -1
	}
	return fx.elements[1].getPeakIdx()
}

// HasElement 检查第 n 个特征序列元素是否已设置（0/1/2）。
func (fx *eigenFX) HasElement(idx int) bool {
	if idx < 0 || idx > 2 {
		return false
	}
	return fx.elements[idx] != nil
}

// IsGapElement 检查第 n 个特征序列元素是否为缺口分形（n=1）。
func (fx *eigenFX) IsGapElement(idx int) bool {
	if idx < 0 || idx > 2 || fx.elements[idx] == nil {
		return false
	}
	return fx.elements[idx].gap
}

// ActualBreakFlag 返回 actual_break_flag 状态。
func (fx *eigenFX) ActualBreakFlag() bool {
	return fx.actualBreakFlag
}
