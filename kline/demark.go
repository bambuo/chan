package kline

// DemarkEngine 实现 TD 九转序列（Demark 9-Sequence）。
// 对齐 chan.py Math/Demark.py CDemarkEngine。
//
// 用于在 K 线序列上识别 setup（准备）和 countdown（倒数）信号。
type DemarkEngine struct {
	KL                 []DemarkKL
	Series             []*DemarkSetup
	DemarkLen          int
	SetupBias          int
	CountdownBias      int
	MaxCountdown       int
	TiaoKongST         bool
	SetupCmp2Close     bool
	CountdownCmp2Close bool
}

// DemarkKL 单根 K 线数据（简化版，用于 Demark 计算）。
type DemarkKL struct {
	Idx   int
	Close float64
	High  float64
	Low   float64
}

// DemarkSignal 表示一个 Demark 信号。
type DemarkSignal struct {
	Type  string // "setup" or "countdown"
	Dir   int    // 1=up, -1=down
	Idx   int    // 信号索引 (1-based)
	Price float64
}

// NewDemarkEngine 创建 Demark 引擎。
func NewDemarkEngine(demarkLen, setupBias, countdownBias, maxCountdown int,
	tiaoKongST, setupCmp2Close, countdownCmp2Close bool) *DemarkEngine {
	return &DemarkEngine{
		DemarkLen:          demarkLen,
		SetupBias:          setupBias,
		CountdownBias:      countdownBias,
		MaxCountdown:       maxCountdown,
		TiaoKongST:         tiaoKongST,
		SetupCmp2Close:     setupCmp2Close,
		CountdownCmp2Close: countdownCmp2Close,
	}
}

// DefaultDemarkEngine 返回默认配置的 Demark 引擎。
func DefaultDemarkEngine() *DemarkEngine {
	return NewDemarkEngine(9, 4, 2, 13, true, true, true)
}

// Update 输入一根新 K 线，返回该 K 线触发的所有 Demark 信号。
// 对应 Python: CDemarkEngine.update
func (de *DemarkEngine) Update(idx int, close, high, low float64) []DemarkSignal {
	kl := DemarkKL{Idx: idx, Close: close, High: high, Low: low}
	de.KL = append(de.KL, kl)

	if len(de.KL) < de.SetupBias+2 {
		return nil
	}

	curr := &de.KL[len(de.KL)-1]
	ref := &de.KL[len(de.KL)-1-de.SetupBias]

	var signals []DemarkSignal

	if curr.Close < ref.Close {
		// 向下 setup 条件
		if !de.hasActiveSetup(-1) {
			de.Series = append(de.Series, newDemarkSetup(-1, de.KL, de.SetupBias))
		}
		// 关闭向上 setup
		for _, s := range de.Series {
			if s.Dir == 1 && s.Countdown == nil && !s.Finished {
				s.Finished = true
			}
		}
	} else if curr.Close > ref.Close {
		// 向上 setup 条件
		if !de.hasActiveSetup(1) {
			de.Series = append(de.Series, newDemarkSetup(1, de.KL, de.SetupBias))
		}
		// 关闭向下 setup
		for _, s := range de.Series {
			if s.Dir == -1 && s.Countdown == nil && !s.Finished {
				s.Finished = true
			}
		}
	}

	// 更新所有 series，收集信号
	for _, s := range de.Series {
		sig := s.Update(curr, de)
		signals = append(signals, sig...)
	}

	// 清理已完成且无 countdown 的 series
	de.cleanup()

	return signals
}

func (de *DemarkEngine) hasActiveSetup(dir int) bool {
	for _, s := range de.Series {
		if s.Dir == dir && !s.Finished {
			return true
		}
	}
	return false
}

func (de *DemarkEngine) cleanup() {
	var kept []*DemarkSetup
	for _, s := range de.Series {
		if s.Finished && s.Countdown == nil {
			continue
		}
		if s.Countdown != nil && s.Countdown.Finished {
			continue
		}
		kept = append(kept, s)
	}
	de.Series = kept
}

// DemarkSetup 表示一个 Demark setup 序列。
type DemarkSetup struct {
	Dir       int // 1=up, -1=down
	KLList    []DemarkKL
	PreKL     *DemarkKL
	Idx       int
	Finished  bool
	Countdown *DemarkCountdown
	TDSTPeak  float64
}

func newDemarkSetup(dir int, allKL []DemarkKL, setupBias int) *DemarkSetup {
	// 取 setupBias+1 根 K 线作为初始数据
	n := len(allKL)
	start := n - setupBias - 1
	if start < 0 {
		start = 0
	}
	kls := make([]DemarkKL, 0, setupBias+1)
	for i := start; i < n-1; i++ {
		kls = append(kls, allKL[i])
	}
	var preKL *DemarkKL
	if start-1 >= 0 {
		preKL = &allKL[start-1]
	}
	return &DemarkSetup{
		Dir:    dir,
		KLList: kls,
		PreKL:  preKL,
		Idx:    1, // setup 从 1 开始计数
	}
}

func (ds *DemarkSetup) Update(curr *DemarkKL, eng *DemarkEngine) []DemarkSignal {
	var signals []DemarkSignal
	if !ds.Finished {
		ds.KLList = append(ds.KLList, *curr)
		ref := &ds.KLList[len(ds.KLList)-1-eng.SetupBias]
		cmp := ref.Close
		if eng.SetupCmp2Close {
			cmp = ref.Close
		} else {
			cmp = ref.Close // simplified
		}

		valid := false
		if ds.Dir == -1 && curr.Close < cmp {
			valid = true
		} else if ds.Dir == 1 && curr.Close > cmp {
			valid = true
		}
		if valid {
			ds.Idx++
			signals = append(signals, DemarkSignal{
				Type: "setup", Dir: ds.Dir, Idx: ds.Idx, Price: curr.Close,
			})
		} else {
			ds.Finished = true
		}
	}

	// Setup 完成 → 创建 countdown
	if ds.Idx == eng.DemarkLen && !ds.Finished && ds.Countdown == nil {
		ds.calcTDSTPeak(eng)
		ds.Countdown = newDemarkCountdown(ds.Dir, ds.KLList, ds.TDSTPeak)
	}

	// Countdown 更新
	if ds.Countdown != nil {
		cdSig := ds.Countdown.Update(curr, eng)
		if cdSig != nil {
			signals = append(signals, *cdSig)
		}
	}

	return signals
}

func (ds *DemarkSetup) calcTDSTPeak(eng *DemarkEngine) {
	start := eng.SetupBias
	end := start + eng.DemarkLen
	if end > len(ds.KLList) {
		end = len(ds.KLList)
	}
	if start >= end {
		return
	}
	if ds.Dir == -1 {
		peak := ds.KLList[start].High
		for _, kl := range ds.KLList[start:end] {
			if kl.High > peak {
				peak = kl.High
			}
		}
		if eng.TiaoKongST && ds.PreKL != nil && ds.KLList[start].High < ds.PreKL.Close {
			if ds.PreKL.Close > peak {
				peak = ds.PreKL.Close
			}
		}
		ds.TDSTPeak = peak
	} else {
		peak := ds.KLList[start].Low
		for _, kl := range ds.KLList[start:end] {
			if kl.Low < peak {
				peak = kl.Low
			}
		}
		if eng.TiaoKongST && ds.PreKL != nil && ds.KLList[start].Low > ds.PreKL.Close {
			if ds.PreKL.Close < peak {
				peak = ds.PreKL.Close
			}
		}
		ds.TDSTPeak = peak
	}
}

// DemarkCountdown 表示 Demark countdown（倒数）阶段。
type DemarkCountdown struct {
	Dir      int
	KLList   []DemarkKL
	TDSTPeak float64
	Idx      int
	Finished bool
}

func newDemarkCountdown(dir int, klList []DemarkKL, tdstPeak float64) *DemarkCountdown {
	kls := make([]DemarkKL, len(klList))
	copy(kls, klList)
	return &DemarkCountdown{
		Dir:      dir,
		KLList:   kls,
		TDSTPeak: tdstPeak,
	}
}

func (cd *DemarkCountdown) Update(curr *DemarkKL, eng *DemarkEngine) *DemarkSignal {
	if cd.Finished {
		return nil
	}
	cd.KLList = append(cd.KLList, *curr)
	if len(cd.KLList) <= eng.CountdownBias+1 {
		return nil
	}
	if cd.Idx >= eng.MaxCountdown {
		cd.Finished = true
		return nil
	}
	// 检查是否突破 TDST
	if (cd.Dir == -1 && curr.High > cd.TDSTPeak) || (cd.Dir == 1 && curr.Low < cd.TDSTPeak) {
		cd.Finished = true
		return nil
	}
	// 检查 countdown 条件
	ref := &cd.KLList[len(cd.KLList)-1-eng.CountdownBias]
	cmp := ref.Close
	if eng.CountdownCmp2Close {
		cmp = ref.Close
	} else {
		cmp = ref.Close
	}
	if (cd.Dir == -1 && curr.Close < cmp) || (cd.Dir == 1 && curr.Close > cmp) {
		cd.Idx++
		return &DemarkSignal{
			Type: "countdown", Dir: cd.Dir, Idx: cd.Idx, Price: curr.Close,
		}
	}
	return nil
}
