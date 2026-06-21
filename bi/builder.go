package bi

import "github.com/bambuo/chan/types"

// BuildBis 从分型列表构建笔序列（状态机，对齐 chan.py CBiList）。
func BuildBis(klines []types.Kline, fractals []types.Fractal, config types.Config) []types.Bi {
	return buildSM(klines, fractals, config)
}

// MergeBis 对笔序列进行包含处理。
func MergeBis(bis []types.Bi) []types.MergedBi {
	if len(bis) < 2 {
		r := make([]types.MergedBi, len(bis))
		for i, b := range bis {
			r[i] = types.MergedBi{Bi: b, OriginalCount: 1}
		}
		return r
	}
	r := make([]types.MergedBi, 0, len(bis))
	r = append(r, types.MergedBi{Bi: bis[0], OriginalCount: 1, MergedFrom: []int{0}})
	idx := 1
	for idx < len(bis) {
		last := &r[len(r)-1]
		cur := bis[idx]
		if last.Direction != cur.Direction {
			r = append(r, types.MergedBi{Bi: cur, OriginalCount: 1, MergedFrom: []int{idx}})
			idx++
			continue
		}
		if contain(cur, last.Bi) {
			d := biDir(r)
			m := mergePair(last.Bi, cur, d)
			last.Bi = m
			last.OriginalCount++
			last.MergedFrom = append(last.MergedFrom, idx)
			idx++
			continue
		}
		if contain(last.Bi, cur) {
			d := biDir(r[:len(r)-1])
			m := mergePair(cur, last.Bi, d)
			r[len(r)-1] = types.MergedBi{Bi: m, OriginalCount: last.OriginalCount + 1, MergedFrom: append(last.MergedFrom, idx)}
			idx++
			continue
		}
		r = append(r, types.MergedBi{Bi: cur, OriginalCount: 1, MergedFrom: []int{idx}})
		idx++
	}
	return r
}

// ── 状态机实现 ──

type machine struct {
	cfg     types.Config
	klines  []types.Kline
	bis     []types.Bi
	lastEnd *types.Fractal
	free    []types.Fractal
	sure    []bool
	used    []bool
	ends    [][]types.Fractal
}

func buildSM(klines []types.Kline, fractals []types.Fractal, cfg types.Config) []types.Bi {
	m := &machine{cfg: cfg, klines: klines}
	for i := range fractals {
		m.step(&fractals[i])
	}
	return m.done()
}

func (m *machine) step(f *types.Fractal) {
	m.delVirt()
	if m.lastEnd == nil || len(m.bis) == 0 {
		m.first(f)
		return
	}
	if f.Type == m.lastEnd.Type {
		m.updEnd(f)
		return
	}
	if m.canMake(f, *m.lastEnd, false) {
		m.add(*m.lastEnd, *f, true)
		m.lastEnd = f
		return
	}
	m.updPeak(f)
}

func (m *machine) delVirt() {
	if len(m.bis) == 0 {
		return
	}
	li := len(m.bis) - 1
	if m.sure[li] {
		return
	}
	if len(m.ends[li]) > 0 {
		se := m.ends[li][0]
		m.bis[li].EndIndex = se.Index
		m.bis[li].EndPrice = fxPrice(&se)
		m.bis[li].KLineCount = m.bis[li].EndIndex - m.bis[li].StartIndex + 1
		m.sure[li] = true
		m.used[li] = true
		m.lastEnd = &types.Fractal{Index: m.bis[li].EndIndex, Type: m.bis[li].EndFractalType()}
		for j := 1; j < len(m.ends[li]); j++ {
			m.add(m.bis[len(m.bis)-1].ToEndFractal(), m.ends[li][j], true)
		}
		m.ends[li] = nil
	} else {
		m.bis = m.bis[:li]
		m.sure = m.sure[:li]
		m.used = m.used[:li]
		m.ends = m.ends[:li]
	}
	if len(m.bis) > 0 {
		m.lastEnd = &types.Fractal{Index: m.bis[len(m.bis)-1].EndIndex, Type: m.bis[len(m.bis)-1].EndFractalType()}
	} else {
		m.lastEnd = nil
	}
}

func (m *machine) first(f *types.Fractal) {
	for _, e := range m.free {
		if e.Type == f.Type {
			continue
		}
		if m.canMake(f, e, false) {
			m.add(e, *f, true)
			m.lastEnd = f
			m.free = nil
			return
		}
	}
	m.free = append(m.free, *f)
	m.lastEnd = f
}

func (m *machine) updEnd(f *types.Fractal) {
	if len(m.bis) == 0 {
		return
	}
	l := &m.bis[len(m.bis)-1]
	vit := !m.sure[len(m.bis)-1]
	if l.IsUp() && f.Type != types.TopFractal {
		return
	}
	if l.IsDown() && f.Type != types.BottomFractal {
		return
	}
	if l.IsUp() && f.High < l.EndPrice {
		return
	}
	if l.IsDown() && f.Low > l.EndPrice {
		return
	}
	if vit {
		m.saveEnd(len(m.bis)-1, types.Fractal{Index: l.EndIndex, Type: l.EndFractalType()})
	}
	l.EndIndex = f.Index
	l.EndPrice = fxPrice(f)
	l.KLineCount = l.EndIndex - l.StartIndex + 1
	m.sure[len(m.bis)-1] = false
	m.lastEnd = f
}

func (m *machine) saveEnd(idx int, f types.Fractal) {
	for len(m.ends) <= idx {
		m.ends = append(m.ends, nil)
	}
	m.ends[idx] = append(m.ends[idx], f)
}

func (m *machine) add(start, end types.Fractal, sure bool) {
	dir := types.DirDown
	if start.Type == types.BottomFractal && end.Type == types.TopFractal {
		dir = types.DirUp
	}
	sp := start.Low
	if start.Type == types.TopFractal {
		sp = start.High
	}
	ep := end.Low
	if end.Type == types.TopFractal {
		ep = end.High
	}
	hi, lo := sp, sp
	for i := start.Index; i <= end.Index && i < len(m.klines); i++ {
		if i < 0 {
			continue
		}
		if m.klines[i].High > hi {
			hi = m.klines[i].High
		}
		if m.klines[i].Low < lo {
			lo = m.klines[i].Low
		}
	}
	ln := ep - sp
	if ln < 0 {
		ln = -ln
	}
	kc := end.Index - start.Index + 1
	sl := 0.0
	if kc > 0 {
		sl = ln / float64(kc)
	}
	m.bis = append(m.bis, types.Bi{
		StartIndex: start.Index, EndIndex: end.Index, Direction: dir,
		StartPrice: sp, EndPrice: ep, High: hi, Low: lo,
		Length: ln, Slope: sl, KLineCount: kc,
	})
	m.sure = append(m.sure, sure)
	m.used = append(m.used, sure)
	for len(m.ends) < len(m.bis) {
		m.ends = append(m.ends, nil)
	}
}

func (m *machine) canMake(f *types.Fractal, last types.Fractal, virt bool) bool {
	if m.cfg.BiAlgo != "fx" {
		span := f.Index - last.Index
		if m.cfg.GapAsKl && span < 4 {
			for i := last.Index; i < f.Index && i < len(m.klines)-1; i++ {
				if i < 0 {
					continue
				}
				if m.klines[i].High < m.klines[i+1].Low || m.klines[i].Low > m.klines[i+1].High {
					span++
				}
			}
		}
		if m.cfg.BiStrict && span < 4 {
			return false
		}
		if !m.cfg.BiStrict && span < 3 {
			return false
		}
	}
	if !fxValid(m.klines, last, *f, m.cfg.BiFxCheck) {
		return false
	}
	if m.cfg.BiEndIsPeak && !virt && !endPeak(m.klines, last, *f) {
		return false
	}
	return true
}

func (m *machine) updPeak(f *types.Fractal) {
	if m.cfg.BiAllowSubPeak || len(m.bis) < 2 {
		return
	}
	l := &m.bis[len(m.bis)-1]
	p := &m.bis[len(m.bis)-2]
	if (l.IsDown() && f.High < l.StartPrice) || (l.IsUp() && f.Low > l.StartPrice) {
		return
	}
	pb := toFractal(p)
	if !endPeak(m.klines, pb, *f) {
		return
	}
	if (l.IsDown() && l.EndPrice >= p.StartPrice) || (l.IsUp() && l.EndPrice <= p.StartPrice) {
		return
	}
	if (l.IsDown() && f.High >= l.EndPrice) || (l.IsUp() && f.Low <= l.EndPrice) {
		return
	}
	// Pop last, try update prev
	lastBi := m.bis[len(m.bis)-1]
	lastSure := m.sure[len(m.bis)-1]
	lastUsed := m.used[len(m.bis)-1]
	m.bis = m.bis[:len(m.bis)-1]
	m.sure = m.sure[:len(m.bis)]
	m.used = m.used[:len(m.bis)]
	m.ends = m.ends[:len(m.bis)]
	if len(m.bis) > 0 {
		pr := &m.bis[len(m.bis)-1]
		if (pr.IsUp() && f.Type == types.TopFractal && f.High >= pr.EndPrice) ||
			(pr.IsDown() && f.Type == types.BottomFractal && f.Low <= pr.EndPrice) {
			pr.EndIndex = f.Index
			pr.EndPrice = fxPrice(f)
			pr.KLineCount = pr.EndIndex - pr.StartIndex + 1
			m.lastEnd = f
			return
		}
	}
	m.bis = append(m.bis, lastBi)
	m.sure = append(m.sure, lastSure)
	m.used = append(m.used, lastUsed)
	m.ends = append(m.ends, nil)
}

func (m *machine) done() []types.Bi {
	m.delVirt()
	// 对齐 Python try_add_virtual_bi：处理最后K线之后的剩余K线
	if len(m.klines) > 0 && len(m.bis) > 0 {
		lastBi := m.bis[len(m.bis)-1]
		if lastKlIdx := len(m.klines) - 1; lastKlIdx > lastBi.EndIndex {
			m.tryAddVirtualBi(lastKlIdx)
		}
	}
	return m.bis
}

// tryAddVirtualBi 对齐 Python CBiList.try_add_virtual_bi。
// 检查最后K线是否能延伸最后一笔或构成新的虚拟笔。
func (m *machine) tryAddVirtualBi(lastKlIdx int) {
	if len(m.bis) == 0 {
		return
	}
	li := len(m.bis) - 1
	l := &m.bis[li]
	if lastKlIdx == l.EndIndex {
		return
	}
	// 同向延伸
	if (l.IsUp() && m.klines[lastKlIdx].High >= m.klines[l.EndIndex].High) ||
		(l.IsDown() && m.klines[lastKlIdx].Low <= m.klines[l.EndIndex].Low) {
		l.EndIndex = lastKlIdx
		l.EndPrice = m.klines[lastKlIdx].Close
		l.KLineCount = l.EndIndex - l.StartIndex + 1
		m.sure[li] = false
		return
	}
	// 反向：尝试构成新笔
	tmpIdx := lastKlIdx
	for tmpIdx > l.EndIndex {
		vf := types.Fractal{
			Index: tmpIdx,
			Type:  types.BottomFractal,
			High:  m.klines[tmpIdx].High,
			Low:   m.klines[tmpIdx].Low,
		}
		if l.IsUp() {
			vf.Type = types.TopFractal
		} else {
			vf.Type = types.BottomFractal
		}
		le := types.Fractal{
			Index: l.EndIndex,
			Type:  l.EndFractalType(),
			High:  m.klines[l.EndIndex].High,
			Low:   m.klines[l.EndIndex].Low,
		}
		if m.canMake(&vf, le, true) {
			m.add(le, vf, false)
			return
		}
		if m.tryUpdPeakVirtual(vf) {
			return
		}
		tmpIdx--
	}
}

// tryUpdPeakVirtual 虚拟笔的次高点/低点替换（对齐 Python update_peak for_virtual=True）。
func (m *machine) tryUpdPeakVirtual(f types.Fractal) bool {
	if !m.cfg.BiAllowSubPeak || len(m.bis) < 2 {
		return false
	}
	l := &m.bis[len(m.bis)-1]
	p := &m.bis[len(m.bis)-2]
	if (l.IsDown() && f.High < l.StartPrice) || (l.IsUp() && f.Low > l.StartPrice) {
		return false
	}
	pb := toFractal(p)
	if !endPeak(m.klines, pb, f) {
		return false
	}
	if (l.IsDown() && l.EndPrice >= p.StartPrice) || (l.IsUp() && l.EndPrice <= p.StartPrice) {
		return false
	}
	// Pop last, try update prev
	m.bis = m.bis[:len(m.bis)-1]
	m.sure = m.sure[:len(m.sure)]
	m.used = m.used[:len(m.used)]
	m.ends = m.ends[:len(m.ends)]
	if len(m.bis) > 0 {
		pr := &m.bis[len(m.bis)-1]
		if (pr.IsUp() && f.Type == types.TopFractal && f.High >= pr.EndPrice) ||
			(pr.IsDown() && f.Type == types.BottomFractal && f.Low <= pr.EndPrice) {
			pr.EndIndex = f.Index
			pr.EndPrice = fxPrice(&f)
			pr.KLineCount = pr.EndIndex - pr.StartIndex + 1
			m.lastEnd = &f
			return true
		}
	}
	return false
}

func fxPrice(f *types.Fractal) float64 {
	if f.Type == types.TopFractal {
		return f.High
	}
	return f.Low
}

func toFractal(bi *types.Bi) types.Fractal {
	if bi.IsUp() {
		return types.Fractal{Index: bi.StartIndex, Type: types.BottomFractal, High: bi.EndPrice, Low: bi.StartPrice}
	}
	return types.Fractal{Index: bi.StartIndex, Type: types.TopFractal, High: bi.StartPrice, Low: bi.EndPrice}
}

func fxValid(klines []types.Kline, start, end types.Fractal, method string) bool {
	if start.Index < 0 || start.Index >= len(klines) || end.Index < 0 || end.Index >= len(klines) {
		return true
	}
	if start.Type == types.TopFractal {
		var eh, sl float64
		switch method {
		case "half":
			eh = maxf(klineHigh(klines, end.Index-1), end.High)
			sl = minf(start.Low, klineLow(klines, start.Index+1))
		case "loss":
			eh, sl = end.High, start.Low
		case "strict", "totally":
			eh = maxf(maxf(klineHigh(klines, end.Index-1), end.High), klineHigh(klines, end.Index+1))
			sl = minf(minf(start.Low, klineLow(klines, start.Index-1)), klineLow(klines, start.Index+1))
		default:
			eh = maxf(klineHigh(klines, end.Index-1), end.High)
			sl = minf(start.Low, klineLow(klines, start.Index+1))
		}
		if method == "totally" {
			return start.Low > eh
		}
		return start.High > eh && end.Low < sl
	} else {
		var el, sh float64
		switch method {
		case "half":
			el = minf(klineLow(klines, end.Index-1), end.Low)
			sh = maxf(start.High, klineHigh(klines, start.Index+1))
		case "loss":
			el, sh = end.Low, start.High
		case "strict", "totally":
			el = minf(minf(klineLow(klines, end.Index-1), end.Low), klineLow(klines, end.Index+1))
			sh = maxf(maxf(start.High, klineHigh(klines, start.Index-1)), klineHigh(klines, start.Index+1))
		default:
			el = minf(klineLow(klines, end.Index-1), end.Low)
			sh = maxf(start.High, klineHigh(klines, start.Index+1))
		}
		if method == "totally" {
			return start.High < el
		}
		return start.Low < el && end.High > sh
	}
}

func endPeak(klines []types.Kline, start, end types.Fractal) bool {
	if start.Type == types.BottomFractal {
		for i := start.Index + 1; i < end.Index && i < len(klines); i++ {
			if i < 0 {
				continue
			}
			if klines[i].High > end.High {
				return false
			}
		}
	} else {
		for i := start.Index + 1; i < end.Index && i < len(klines); i++ {
			if i < 0 {
				continue
			}
			if klines[i].Low < end.Low {
				return false
			}
		}
	}
	return true
}

func klineHigh(k []types.Kline, i int) float64 {
	if i >= 0 && i < len(k) {
		return k[i].High
	}
	return 0
}

func klineLow(k []types.Kline, i int) float64 {
	if i >= 0 && i < len(k) {
		return k[i].Low
	}
	return 1e18
}

// ── 辅助 ──

func contain(a, b types.Bi) bool { return a.High >= b.High && a.Low <= b.Low }

type Direction = types.Direction

func biDir(bis []types.MergedBi) Direction {
	for i := len(bis) - 1; i >= 1; i-- {
		p, c := bis[i-1].Bi, bis[i].Bi
		if !contain(p, c) && !contain(c, p) {
			if c.High > p.High && c.Low > p.Low {
				return types.DirUp
			}
			if c.High < p.High && c.Low < p.Low {
				return types.DirDown
			}
		}
	}
	return types.DirNone
}

func mergePair(a, b types.Bi, dir Direction) types.Bi {
	m := a
	m.EndIndex = maxInt(a.EndIndex, b.EndIndex)
	m.EndPrice = b.EndPrice
	m.KLineCount = m.EndIndex - m.StartIndex + 1
	switch dir {
	case types.DirUp:
		m.High = maxf(a.High, b.High)
		m.Low = maxf(a.Low, b.Low)
	case types.DirDown:
		m.High = minf(a.High, b.High)
		m.Low = minf(a.Low, b.Low)
	default:
		if a.Direction == types.DirUp {
			m.High = maxf(a.High, b.High)
			m.Low = maxf(a.Low, b.Low)
		} else {
			m.High = minf(a.High, b.High)
			m.Low = minf(a.Low, b.Low)
		}
	}
	m.Length = m.EndPrice - m.StartPrice
	if m.Length < 0 {
		m.Length = -m.Length
	}
	if m.KLineCount > 0 {
		m.Slope = m.Length / float64(m.KLineCount)
	}
	return m
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// BuildVirtualBi 从最后一笔的终点出发，在尾部 K 线中寻找反向极端价格，构造独立虚笔。
// 不修改现有笔列表。若无法构建则返回 nil。
//
// 逻辑对齐 index.html 的 buildVirtualBi：
//   - 最后一笔是向上笔（终点为顶分型）→ 在尾部K线中找最低点，若低于起点则构成向下虚笔
//   - 最后一笔是向下笔（终点为底分型）→ 在尾部K线中找最高点，若高于起点则构成向上虚笔
func BuildVirtualBi(klines []types.Kline, bis []types.Bi) *types.Bi {
	if len(bis) == 0 || len(klines) == 0 {
		return nil
	}
	lastBi := bis[len(bis)-1]
	if lastBi.EndIndex >= len(klines)-1 {
		return nil // 没有尾部 K 线
	}
	startPrice := lastBi.EndPrice
	startIdx := lastBi.EndIndex
	startType := lastBi.EndFractalType()

	tailStart := startIdx + 1
	if tailStart >= len(klines) {
		return nil
	}

	if startType == types.TopFractal {
		// 向上笔结束于顶分型，构造向下虚笔
		endIdx := tailStart
		endPrice := klines[tailStart].Low
		for i := tailStart + 1; i < len(klines); i++ {
			if klines[i].Low < endPrice {
				endPrice = klines[i].Low
				endIdx = i
			}
		}
		if endPrice >= startPrice {
			return nil // 没有更低点
		}
		hi, lo := startPrice, startPrice
		for i := startIdx; i <= endIdx; i++ {
			if klines[i].High > hi {
				hi = klines[i].High
			}
			if klines[i].Low < lo {
				lo = klines[i].Low
			}
		}
		ln := startPrice - endPrice
		kc := endIdx - startIdx + 1
		return &types.Bi{
			StartIndex: startIdx, EndIndex: endIdx,
			Direction:  types.DirDown,
			StartPrice: startPrice, EndPrice: endPrice,
			High: hi, Low: lo, Length: ln,
			Slope: ln / float64(kc), KLineCount: kc,
		}
	}

	// 向下笔结束于底分型，构造向上虚笔
	endIdx := tailStart
	endPrice := klines[tailStart].High
	for i := tailStart + 1; i < len(klines); i++ {
		if klines[i].High > endPrice {
			endPrice = klines[i].High
			endIdx = i
		}
	}
	if endPrice <= startPrice {
		return nil // 没有更高点
	}
	hi, lo := startPrice, startPrice
	for i := startIdx; i <= endIdx; i++ {
		if klines[i].High > hi {
			hi = klines[i].High
		}
		if klines[i].Low < lo {
			lo = klines[i].Low
		}
	}
	ln := endPrice - startPrice
	kc := endIdx - startIdx + 1
	return &types.Bi{
		StartIndex: startIdx, EndIndex: endIdx,
		Direction:  types.DirUp,
		StartPrice: startPrice, EndPrice: endPrice,
		High: hi, Low: lo, Length: ln,
		Slope: ln / float64(kc), KLineCount: kc,
	}
}
