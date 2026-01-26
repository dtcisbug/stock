package backtest

import (
	"math"
	"sort"
)

type pivotKind uint8

const (
	pivotLow pivotKind = iota + 1
	pivotHigh
)

type pivotPoint struct {
	idx   int
	price float64
	kind  pivotKind
}

func collectPivotsAll(bars []Bar, start, end, n int) []pivotPoint {
	if n <= 0 {
		return nil
	}
	if start < 0 {
		start = 0
	}
	if end > len(bars) {
		end = len(bars)
	}
	if end-start <= 2*n {
		return nil
	}
	var out []pivotPoint
	for j := start + n; j < end-n; j++ {
		if isPivotLowBar(bars, j, n) {
			out = append(out, pivotPoint{idx: j, price: bars[j].Low, kind: pivotLow})
		}
		if isPivotHighBar(bars, j, n) {
			out = append(out, pivotPoint{idx: j, price: bars[j].High, kind: pivotHigh})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].idx < out[j].idx })
	return out
}

func isPivotLowBar(bars []Bar, idx, n int) bool {
	x := bars[idx].Low
	if x <= 0 {
		return false
	}
	for k := idx - n; k <= idx+n; k++ {
		if k == idx {
			continue
		}
		if bars[k].Low < x {
			return false
		}
	}
	return true
}

func isPivotHighBar(bars []Bar, idx, n int) bool {
	x := bars[idx].High
	if x <= 0 {
		return false
	}
	for k := idx - n; k <= idx+n; k++ {
		if k == idx {
			continue
		}
		if bars[k].High > x {
			return false
		}
	}
	return true
}

func approxEqual(a, b, tolPct float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	m := math.Min(a, b)
	if m <= 0 {
		return false
	}
	return math.Abs(a-b)/m <= tolPct
}

func lastPivot(pivots []pivotPoint, kind pivotKind, beforeIdx int) (pivotPoint, bool) {
	for i := len(pivots) - 1; i >= 0; i-- {
		if pivots[i].idx >= beforeIdx {
			continue
		}
		if pivots[i].kind == kind {
			return pivots[i], true
		}
	}
	return pivotPoint{}, false
}

func prevPivot(pivots []pivotPoint, from int, kind pivotKind) (pivotPoint, int, bool) {
	for i := from; i >= 0; i-- {
		if pivots[i].kind == kind {
			return pivots[i], i, true
		}
	}
	return pivotPoint{}, -1, false
}

func detectMTop(i int, bars []Bar, pivots []pivotPoint, p PatternsParams) *tradePlan {
	closePx := bars[i].Close
	if closePx <= 0 {
		return nil
	}
	// Find pattern H1 - L - H2, then close breaks below neckline(L)
	for idx := len(pivots) - 1; idx >= 0; idx-- {
		if pivots[idx].kind != pivotHigh {
			continue
		}
		h2 := pivots[idx]
		if h2.idx >= i {
			continue
		}
		l, lIdx, ok := prevPivot(pivots, idx-1, pivotLow)
		if !ok {
			continue
		}
		h1, _, ok := prevPivot(pivots, lIdx-1, pivotHigh)
		if !ok {
			continue
		}
		if !approxEqual(h1.price, h2.price, p.EqualTolPct) {
			continue
		}
		neck := l.price
		if neck <= 0 {
			continue
		}
		if closePx >= neck*(1.0-p.BreakPct) {
			continue
		}
		top := math.Max(h1.price, h2.price)
		height := top - neck
		if height <= 0 {
			continue
		}
		target := neck - height*p.TargetMultiple
		stop := top * (1.0 + p.StopBufferPct)
		return validatePlan(&tradePlan{side: SideShort, target: target, stop: stop, reason: "m_top"})
	}
	return nil
}

func detectWBottom(i int, bars []Bar, pivots []pivotPoint, p PatternsParams) *tradePlan {
	closePx := bars[i].Close
	if closePx <= 0 {
		return nil
	}
	for idx := len(pivots) - 1; idx >= 0; idx-- {
		if pivots[idx].kind != pivotLow {
			continue
		}
		l2 := pivots[idx]
		if l2.idx >= i {
			continue
		}
		h, hIdx, ok := prevPivot(pivots, idx-1, pivotHigh)
		if !ok {
			continue
		}
		l1, _, ok := prevPivot(pivots, hIdx-1, pivotLow)
		if !ok {
			continue
		}
		if !approxEqual(l1.price, l2.price, p.EqualTolPct) {
			continue
		}
		neck := h.price
		if closePx <= neck*(1.0+p.BreakPct) {
			continue
		}
		bot := math.Min(l1.price, l2.price)
		height := neck - bot
		if height <= 0 {
			continue
		}
		target := neck + height*p.TargetMultiple
		stop := bot * (1.0 - p.StopBufferPct)
		return validatePlan(&tradePlan{side: SideLong, target: target, stop: stop, reason: "w_bottom"})
	}
	return nil
}

func detectHSTop(i int, bars []Bar, pivots []pivotPoint, p PatternsParams) *tradePlan {
	closePx := bars[i].Close
	if closePx <= 0 {
		return nil
	}
	// Last 5 pivots: H (LS), L (T1), H (Head), L (T2), H (RS)
	rs, rsIdx, ok := prevPivot(pivots, len(pivots)-1, pivotHigh)
	if !ok || rs.idx >= i {
		return nil
	}
	t2, t2Idx, ok := prevPivot(pivots, rsIdx-1, pivotLow)
	if !ok {
		return nil
	}
	head, headIdx, ok := prevPivot(pivots, t2Idx-1, pivotHigh)
	if !ok {
		return nil
	}
	t1, t1Idx, ok := prevPivot(pivots, headIdx-1, pivotLow)
	if !ok {
		return nil
	}
	ls, _, ok := prevPivot(pivots, t1Idx-1, pivotHigh)
	if !ok {
		return nil
	}
	if head.price <= ls.price || head.price <= rs.price {
		return nil
	}
	if !approxEqual(ls.price, rs.price, p.EqualTolPct) {
		return nil
	}
	neck := lineAt(t1.idx, t1.price, t2.idx, t2.price, i)
	if neck <= 0 {
		return nil
	}
	if closePx >= neck*(1.0-p.BreakPct) {
		return nil
	}
	neckAtHead := lineAt(t1.idx, t1.price, t2.idx, t2.price, head.idx)
	if neckAtHead <= 0 {
		return nil
	}
	height := head.price - neckAtHead
	if height <= 0 {
		return nil
	}
	target := neck - height*p.TargetMultiple
	stop := math.Max(head.price, rs.price) * (1.0 + p.StopBufferPct)
	return validatePlan(&tradePlan{side: SideShort, target: target, stop: stop, reason: "hs_top"})
}

func detectHSBottom(i int, bars []Bar, pivots []pivotPoint, p PatternsParams) *tradePlan {
	closePx := bars[i].Close
	if closePx <= 0 {
		return nil
	}
	// Last 5 pivots: L (LS), H (P1), L (Head), H (P2), L (RS)
	rs, rsIdx, ok := prevPivot(pivots, len(pivots)-1, pivotLow)
	if !ok || rs.idx >= i {
		return nil
	}
	p2, p2Idx, ok := prevPivot(pivots, rsIdx-1, pivotHigh)
	if !ok {
		return nil
	}
	head, headIdx, ok := prevPivot(pivots, p2Idx-1, pivotLow)
	if !ok {
		return nil
	}
	p1, p1Idx, ok := prevPivot(pivots, headIdx-1, pivotHigh)
	if !ok {
		return nil
	}
	ls, _, ok := prevPivot(pivots, p1Idx-1, pivotLow)
	if !ok {
		return nil
	}
	if head.price >= ls.price || head.price >= rs.price {
		return nil
	}
	if !approxEqual(ls.price, rs.price, p.EqualTolPct) {
		return nil
	}
	neck := lineAt(p1.idx, p1.price, p2.idx, p2.price, i)
	if neck <= 0 {
		return nil
	}
	if closePx <= neck*(1.0+p.BreakPct) {
		return nil
	}
	neckAtHead := lineAt(p1.idx, p1.price, p2.idx, p2.price, head.idx)
	if neckAtHead <= 0 {
		return nil
	}
	height := neckAtHead - head.price
	if height <= 0 {
		return nil
	}
	target := neck + height*p.TargetMultiple
	stop := math.Min(head.price, rs.price) * (1.0 - p.StopBufferPct)
	return validatePlan(&tradePlan{side: SideLong, target: target, stop: stop, reason: "hs_bottom"})
}

func lineAt(x1 int, y1 float64, x2 int, y2 float64, x int) float64 {
	if x2 == x1 {
		return y2
	}
	t := float64(x-x1) / float64(x2-x1)
	return y1 + (y2-y1)*t
}

func detectTriangle(i int, bars []Bar, p PatternsParams) *tradePlan {
	if i <= 0 {
		return nil
	}
	start := i - p.TriangleLookback
	if start < 0 {
		start = 0
	}
	pivots := collectPivotsAll(bars, start, i, p.PivotN)
	var highs, lows []pivotPoint
	for _, pv := range pivots {
		switch pv.kind {
		case pivotHigh:
			highs = append(highs, pv)
		case pivotLow:
			lows = append(lows, pv)
		}
	}
	if len(highs) < p.TriangleMinPivots || len(lows) < p.TriangleMinPivots {
		return nil
	}

	uh1 := highs[0]
	uh2 := highs[len(highs)-1]
	lh1 := lows[0]
	lh2 := lows[len(lows)-1]
	if uh2.idx == uh1.idx || lh2.idx == lh1.idx {
		return nil
	}

	upperSlope := (uh2.price - uh1.price) / float64(uh2.idx-uh1.idx)
	lowerSlope := (lh2.price - lh1.price) / float64(lh2.idx-lh1.idx)
	// symmetrical triangle: upper down, lower up
	if upperSlope >= 0 || lowerSlope <= 0 {
		return nil
	}

	// intersection time (apex)
	// upper: y = uh1 + upperSlope*(t-uh1.idx)
	// lower: y = lh1 + lowerSlope*(t-lh1.idx)
	den := upperSlope - lowerSlope
	if den == 0 {
		return nil
	}
	tApex := (lh1.price - uh1.price + upperSlope*float64(uh1.idx) - lowerSlope*float64(lh1.idx)) / den
	if math.IsNaN(tApex) || math.IsInf(tApex, 0) {
		return nil
	}
	apexIdx := int(math.Round(tApex))
	startIdx := minInt(minInt(uh1.idx, lh1.idx), start)
	if apexIdx <= startIdx+10 {
		return nil
	}
	frac := float64(i-startIdx) / float64(apexIdx-startIdx)
	if frac < p.TriangleMinBreakFrac || frac > p.TriangleMaxBreakFrac {
		return nil
	}

	upper := lineAt(uh1.idx, uh1.price, uh2.idx, uh2.price, i)
	lower := lineAt(lh1.idx, lh1.price, lh2.idx, lh2.price, i)
	if upper <= 0 || lower <= 0 || upper <= lower {
		return nil
	}
	baseUpper := lineAt(uh1.idx, uh1.price, uh2.idx, uh2.price, startIdx)
	baseLower := lineAt(lh1.idx, lh1.price, lh2.idx, lh2.price, startIdx)
	height := baseUpper - baseLower
	if height <= 0 {
		return nil
	}

	closePx := bars[i].Close
	if closePx <= 0 {
		return nil
	}

	if p.EnableTriangleBreakdown && closePx < lower*(1.0-p.BreakPct) {
		target := lower - height*p.TargetMultiple
		stop := upper * (1.0 + p.StopBufferPct)
		return validatePlan(&tradePlan{side: SideShort, target: target, stop: stop, reason: "triangle_breakdown"})
	}
	if p.EnableTriangleBreakout && closePx > upper*(1.0+p.BreakPct) {
		target := upper + height*p.TargetMultiple
		stop := lower * (1.0 - p.StopBufferPct)
		return validatePlan(&tradePlan{side: SideLong, target: target, stop: stop, reason: "triangle_breakout"})
	}
	return nil
}

func detectWaveUp(i int, bars []Bar, pivots []pivotPoint, p PatternsParams) *tradePlan {
	closePx := bars[i].Close
	if closePx <= 0 {
		return nil
	}
	// Find A (pivot low), B (pivot high), C (pivot low) with C > A, then break above B confirms C.
	c, cIdx, ok := prevPivot(pivots, len(pivots)-1, pivotLow)
	if !ok || c.idx >= i {
		return nil
	}
	b, bIdx, ok := prevPivot(pivots, cIdx-1, pivotHigh)
	if !ok {
		return nil
	}
	a, _, ok := prevPivot(pivots, bIdx-1, pivotLow)
	if !ok {
		return nil
	}
	if c.price <= a.price {
		return nil
	}
	if closePx <= b.price*(1.0+p.BreakPct) {
		return nil
	}
	amp := b.price - a.price
	if amp <= 0 {
		return nil
	}
	target := c.price + amp*p.TargetMultiple
	stop := c.price * (1.0 - p.StopBufferPct)
	return validatePlan(&tradePlan{side: SideLong, target: target, stop: stop, reason: "wave_up"})
}

func detectWaveDown(i int, bars []Bar, pivots []pivotPoint, p PatternsParams) *tradePlan {
	closePx := bars[i].Close
	if closePx <= 0 {
		return nil
	}
	// Find A (pivot high), B (pivot low), C (pivot high) with C < A, then break below B confirms C.
	c, cIdx, ok := prevPivot(pivots, len(pivots)-1, pivotHigh)
	if !ok || c.idx >= i {
		return nil
	}
	b, bIdx, ok := prevPivot(pivots, cIdx-1, pivotLow)
	if !ok {
		return nil
	}
	a, _, ok := prevPivot(pivots, bIdx-1, pivotHigh)
	if !ok {
		return nil
	}
	if c.price >= a.price {
		return nil
	}
	if closePx >= b.price*(1.0-p.BreakPct) {
		return nil
	}
	amp := a.price - b.price
	if amp <= 0 {
		return nil
	}
	target := c.price - amp*p.TargetMultiple
	stop := c.price * (1.0 + p.StopBufferPct)
	return validatePlan(&tradePlan{side: SideShort, target: target, stop: stop, reason: "wave_down"})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
