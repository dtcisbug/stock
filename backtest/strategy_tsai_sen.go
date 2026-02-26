package backtest

import (
	"math"
	"sort"
	"time"
)

type TsaiSenParams struct {
	// LevelMode:
	// - "pivots": use pivot-touch clustering to estimate support/resistance (closer to Tsai Sen style)
	// - "extremes": fallback to min(low)/max(high) in lookback window
	LevelMode   string  `yaml:"level_mode" json:"level_mode"`
	BoxLookback int     `yaml:"box_lookback" json:"box_lookback"`
	PivotN      int     `yaml:"pivot_n" json:"pivot_n"`
	TouchTolPct float64 `yaml:"touch_tol_pct" json:"touch_tol_pct"`
	MinTouches  int     `yaml:"min_touches" json:"min_touches"`
	MinRangePct float64 `yaml:"min_range_pct" json:"min_range_pct"`

	BreakPct       float64 `yaml:"break_pct" json:"break_pct"`
	ReclaimPct     float64 `yaml:"reclaim_pct" json:"reclaim_pct"`
	FlipMaxBars    int     `yaml:"flip_max_bars" json:"flip_max_bars"`
	EntryMode      string  `yaml:"entry_mode" json:"entry_mode"` // reclaim_support | stabilize_support | break_resistance
	StabilizeBars  int     `yaml:"stabilize_bars" json:"stabilize_bars"`
	StopBufferPct  float64 `yaml:"stop_buffer_pct" json:"stop_buffer_pct"`
	TargetMultiple float64 `yaml:"target_multiple" json:"target_multiple"`

	VolMAN       int     `yaml:"vol_ma_n" json:"vol_ma_n"`
	VolRatioMin  float64 `yaml:"vol_ratio_min" json:"vol_ratio_min"`
	EnableFakeBO bool    `yaml:"enable_fake_breakout" json:"enable_fake_breakout"`
	FakeMaxBars  int     `yaml:"fake_max_bars" json:"fake_max_bars"`
}

func (p TsaiSenParams) withDefaults() TsaiSenParams {
	if p.LevelMode == "" {
		p.LevelMode = "pivots"
	}
	if p.BoxLookback <= 0 {
		p.BoxLookback = 60
	}
	if p.PivotN <= 0 {
		p.PivotN = 3
	}
	if p.TouchTolPct <= 0 {
		p.TouchTolPct = 0.003
	}
	if p.MinTouches <= 0 {
		p.MinTouches = 3
	}
	if p.MinRangePct <= 0 {
		p.MinRangePct = 0.03
	}
	if p.BreakPct <= 0 {
		p.BreakPct = 0.005
	}
	if p.ReclaimPct < 0 {
		p.ReclaimPct = 0
	}
	if p.FlipMaxBars <= 0 {
		p.FlipMaxBars = 20
	}
	if p.EntryMode == "" {
		p.EntryMode = "reclaim_support"
	}
	if p.StabilizeBars <= 0 {
		p.StabilizeBars = 2
	}
	if p.StopBufferPct <= 0 {
		p.StopBufferPct = 0.005
	}
	if p.TargetMultiple <= 0 {
		p.TargetMultiple = 1.0
	}
	if p.VolMAN <= 0 {
		p.VolMAN = 20
	}
	if p.VolRatioMin <= 0 {
		p.VolRatioMin = 0
	}
	if p.FakeMaxBars <= 0 {
		p.FakeMaxBars = 10
	}
	return p
}

type TsaiSenStrategy struct {
	p TsaiSenParams

	breakActive  bool
	breakIndex   int
	breakSupport float64
	breakResist  float64
	breakLow     float64

	reclaimIndex int

	flipReady bool

	fakeActive  bool
	fakeIndex   int
	fakeResist  float64
	fakeSupport float64

	lastPlan *tsaiSenPlan
}

type tsaiSenPlan struct {
	time   time.Time
	stop   float64
	target float64
}

func NewTsaiSenStrategy(p TsaiSenParams) *TsaiSenStrategy {
	pp := p.withDefaults()
	return &TsaiSenStrategy{p: pp}
}

func (s *TsaiSenStrategy) Params() TsaiSenParams {
	return s.p
}

func (s *TsaiSenStrategy) Clone() Strategy {
	return NewTsaiSenStrategy(s.p)
}

func (s *TsaiSenStrategy) OnBar(i int, bars []Bar, pos Position) *Signal {
	if i <= 0 || i < s.p.BoxLookback {
		return nil
	}

	bar := bars[i]
	support, resist := levels(bars, i, s.p)
	if support <= 0 || resist <= 0 || resist <= support {
		return nil
	}

	// Volume filter (optional)
	if s.p.VolRatioMin > 0 {
		ma := volMA(bars, i, s.p.VolMAN)
		if ma > 0 && float64(bar.Volume)/ma < s.p.VolRatioMin {
			// allow exits even if volume low
			if pos.Side == SideFlat {
				return nil
			}
		}
	}

	// Exits (close-confirm)
	if pos.Side == SideLong {
		// Fake breakout: after breakout above resist, close back below resist => exit
		if s.p.EnableFakeBO {
			s.maybeUpdateFakeState(i, bars, support, resist)
			if s.fakeActive && i <= s.fakeIndex+s.p.FakeMaxBars {
				if bar.Close < s.fakeResist*(1.0-s.p.ReclaimPct) {
					s.fakeActive = false
					return &Signal{Time: bar.Time, Action: SignalSell, Reason: "fake_breakout_confirm"}
				}
			}
		}
		// Basic protection: close back below support => exit
		if bar.Close < support {
			return &Signal{Time: bar.Time, Action: SignalSell, Reason: "close_below_support"}
		}
	}
	if pos.Side == SideShort {
		// Basic protection: close back above resist => cover
		if bar.Close > resist {
			return &Signal{Time: bar.Time, Action: SignalCover, Reason: "close_above_resistance"}
		}
	}

	// Entries
	if pos.Side == SideFlat {
		s.maybeUpdateBreakState(i, bars, support, resist)

		// Break-bottom-flip (破底翻): break below support then reclaim above support within N bars
		if s.breakActive && i <= s.breakIndex+s.p.FlipMaxBars {
			if bar.Close > s.breakSupport*(1.0+s.p.ReclaimPct) {
				s.flipReady = true
				s.reclaimIndex = i
				if s.p.EntryMode == "reclaim_support" {
					s.lastPlan = s.planLong(bar.Time)
					s.resetBreak()
					return &Signal{Time: bar.Time, Action: SignalBuy, Reason: "break_bottom_flip_reclaim_support"}
				}
			}
		}

		// Stabilize entry (止稳确立买点): after reclaim, require N consecutive closes above breakSupport.
		if s.flipReady && s.p.EntryMode == "stabilize_support" {
			need := s.p.StabilizeBars
			if need > 0 && i >= s.reclaimIndex+need-1 && allClosesAbove(bars, i, need, s.breakSupport*(1.0+s.p.ReclaimPct)) {
				s.flipReady = false
				s.lastPlan = s.planLong(bar.Time)
				s.resetBreak()
				return &Signal{Time: bar.Time, Action: SignalBuy, Reason: "break_bottom_flip_stabilize_support"}
			}
		}

		// Entry on resistance break after flip
		if s.flipReady && s.p.EntryMode == "break_resistance" {
			// Use the resistance at breakdown time (closer to neckline/box top definition).
			if bar.Close > s.breakResist {
				s.flipReady = false
				s.lastPlan = s.planLong(bar.Time)
				s.resetBreak()
				return &Signal{Time: bar.Time, Action: SignalBuy, Reason: "break_bottom_flip_break_resistance"}
			}
		}

		// Futures-only: short on fake breakout confirm
		// (range breakout above resist then close back below resist)
		if s.p.EnableFakeBO {
			s.maybeUpdateFakeState(i, bars, support, resist)
			if s.fakeActive && i <= s.fakeIndex+s.p.FakeMaxBars {
				if bar.Close < s.fakeResist*(1.0-s.p.ReclaimPct) {
					s.fakeActive = false
					s.lastPlan = s.planShort(bar.Time)
					return &Signal{Time: bar.Time, Action: SignalShort, Reason: "fake_breakout_confirm"}
				}
			}
		}
	}

	return nil
}

func (s *TsaiSenStrategy) maybeUpdateBreakState(i int, bars []Bar, support, resist float64) {
	bar := bars[i]
	if s.breakActive {
		// expire
		if i > s.breakIndex+s.p.FlipMaxBars {
			s.resetBreak()
		}
		return
	}
	// breakdown condition: intraday low breaks below prior box support by BreakPct
	if bar.Low < support*(1.0-s.p.BreakPct) {
		s.breakActive = true
		s.breakIndex = i
		s.breakSupport = support
		s.breakResist = resist
		s.breakLow = bar.Low
		s.flipReady = false
		s.reclaimIndex = 0
	}
}

func (s *TsaiSenStrategy) maybeUpdateFakeState(i int, bars []Bar, support, resist float64) {
	bar := bars[i]
	if s.fakeActive {
		if i > s.fakeIndex+s.p.FakeMaxBars {
			s.fakeActive = false
		}
		return
	}
	// breakout above resistance (fake breakout candidate)
	if bar.High > resist*(1.0+s.p.BreakPct) && bar.Close > resist {
		s.fakeActive = true
		s.fakeIndex = i
		s.fakeResist = resist
		s.fakeSupport = support
	}
}

func (s *TsaiSenStrategy) resetBreak() {
	s.breakActive = false
	s.breakIndex = 0
	s.breakSupport = 0
	s.breakResist = 0
	s.breakLow = 0
	s.reclaimIndex = 0
}

func (s *TsaiSenStrategy) planLong(t time.Time) *tsaiSenPlan {
	boxHeight := s.breakResist - s.breakSupport
	target := s.breakResist + boxHeight*s.p.TargetMultiple
	stop := s.breakLow * (1.0 - s.p.StopBufferPct)
	if stop <= 0 {
		stop = s.breakSupport * (1.0 - s.p.StopBufferPct)
	}
	if target <= 0 {
		target = s.breakResist
	}
	return &tsaiSenPlan{time: t, stop: stop, target: target}
}

func (s *TsaiSenStrategy) planShort(t time.Time) *tsaiSenPlan {
	boxHeight := s.fakeResist - s.fakeSupport
	target := s.fakeSupport - boxHeight*s.p.TargetMultiple
	stop := s.fakeResist * (1.0 + s.p.StopBufferPct)
	if stop <= 0 {
		stop = s.fakeResist
	}
	if target <= 0 {
		target = s.fakeSupport
	}
	return &tsaiSenPlan{time: t, stop: stop, target: target}
}

func boxLevels(bars []Bar, i int, lookback int) (support, resist float64) {
	start := i - lookback
	if start < 0 {
		start = 0
	}
	lo := math.Inf(1)
	hi := math.Inf(-1)
	for j := start; j < i; j++ {
		if bars[j].Low < lo {
			lo = bars[j].Low
		}
		if bars[j].High > hi {
			hi = bars[j].High
		}
	}
	if !math.IsInf(lo, 0) && !math.IsInf(hi, 0) {
		return lo, hi
	}
	return 0, 0
}

func levels(bars []Bar, i int, p TsaiSenParams) (support, resist float64) {
	pp := p.withDefaults()
	switch pp.LevelMode {
	case "pivots":
		s, r := levelsByPivots(bars, i, pp)
		if s > 0 && r > 0 && r > s {
			return s, r
		}
		return boxLevels(bars, i, pp.BoxLookback)
	case "extremes":
		return boxLevels(bars, i, pp.BoxLookback)
	default:
		return boxLevels(bars, i, pp.BoxLookback)
	}
}

func levelsByPivots(bars []Bar, i int, p TsaiSenParams) (support, resist float64) {
	start := i - p.BoxLookback
	if start < 0 {
		start = 0
	}
	lows, highs := collectPivots(bars, start, i, p.PivotN)
	if len(lows) < p.MinTouches || len(highs) < p.MinTouches {
		return 0, 0
	}

	sup, supCnt := clusterLevel(lows, p.TouchTolPct)
	res, resCnt := clusterLevel(highs, p.TouchTolPct)
	if supCnt < p.MinTouches || resCnt < p.MinTouches || sup <= 0 || res <= 0 || res <= sup {
		return 0, 0
	}
	if (res-sup)/sup < p.MinRangePct {
		return 0, 0
	}
	return sup, res
}

func collectPivots(bars []Bar, start, end, n int) (pivotLows []float64, pivotHighs []float64) {
	if n <= 0 || end-start <= 2*n {
		return nil, nil
	}
	for j := start + n; j < end-n; j++ {
		if isPivotLow(bars, j, n) {
			pivotLows = append(pivotLows, bars[j].Low)
		}
		if isPivotHigh(bars, j, n) {
			pivotHighs = append(pivotHighs, bars[j].High)
		}
	}
	return pivotLows, pivotHighs
}

func isPivotLow(bars []Bar, idx, n int) bool {
	x := bars[idx].Low
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

func isPivotHigh(bars []Bar, idx, n int) bool {
	x := bars[idx].High
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

func clusterLevel(values []float64, tolPct float64) (level float64, count int) {
	if len(values) == 0 {
		return 0, 0
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)

	bestI, bestJ := 0, 0
	i := 0
	for j := 0; j < len(v); j++ {
		for i < j && v[j] > v[i]*(1.0+tolPct) {
			i++
		}
		if j-i > bestJ-bestI {
			bestI, bestJ = i, j
		}
	}
	bestCnt := bestJ - bestI + 1
	if bestCnt <= 0 {
		return 0, 0
	}
	sum := 0.0
	for k := bestI; k <= bestJ; k++ {
		sum += v[k]
	}
	return sum / float64(bestCnt), bestCnt
}

func allClosesAbove(bars []Bar, i int, n int, level float64) bool {
	if n <= 0 {
		return false
	}
	start := i - n + 1
	if start < 0 {
		start = 0
	}
	for j := start; j <= i; j++ {
		if bars[j].Close <= level {
			return false
		}
	}
	return true
}

func volMA(bars []Bar, i int, n int) float64 {
	if n <= 0 {
		return 0
	}
	start := i - n + 1
	if start < 0 {
		start = 0
	}
	sum := 0.0
	cnt := 0
	for j := start; j <= i; j++ {
		if bars[j].Volume <= 0 {
			continue
		}
		sum += float64(bars[j].Volume)
		cnt++
	}
	if cnt == 0 {
		return 0
	}
	return sum / float64(cnt)
}
