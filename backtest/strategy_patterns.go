package backtest

import (
	"fmt"
	"math"
)

type PatternsParams struct {
	Lookback       int     `yaml:"lookback" json:"lookback"`
	PivotN         int     `yaml:"pivot_n" json:"pivot_n"`
	EqualTolPct    float64 `yaml:"equal_tol_pct" json:"equal_tol_pct"`
	BreakPct       float64 `yaml:"break_pct" json:"break_pct"`
	StopBufferPct  float64 `yaml:"stop_buffer_pct" json:"stop_buffer_pct"`
	TargetMultiple float64 `yaml:"target_multiple" json:"target_multiple"`

	EnableMTop              bool `yaml:"enable_m_top" json:"enable_m_top"`
	EnableWBottom           bool `yaml:"enable_w_bottom" json:"enable_w_bottom"`
	EnableHSTop             bool `yaml:"enable_hs_top" json:"enable_hs_top"`
	EnableHSBottom          bool `yaml:"enable_hs_bottom" json:"enable_hs_bottom"`
	EnableTriangleBreakout  bool `yaml:"enable_triangle_breakout" json:"enable_triangle_breakout"`
	EnableTriangleBreakdown bool `yaml:"enable_triangle_breakdown" json:"enable_triangle_breakdown"`
	EnableWaveUp            bool `yaml:"enable_wave_up" json:"enable_wave_up"`
	EnableWaveDown          bool `yaml:"enable_wave_down" json:"enable_wave_down"`

	TriangleLookback     int     `yaml:"triangle_lookback" json:"triangle_lookback"`
	TriangleMinPivots    int     `yaml:"triangle_min_pivots" json:"triangle_min_pivots"`
	TriangleMinBreakFrac float64 `yaml:"triangle_min_break_frac" json:"triangle_min_break_frac"`
	TriangleMaxBreakFrac float64 `yaml:"triangle_max_break_frac" json:"triangle_max_break_frac"`
}

func (p PatternsParams) withDefaults() PatternsParams {
	if p.Lookback <= 0 {
		p.Lookback = 220
	}
	if p.PivotN <= 0 {
		p.PivotN = 3
	}
	if p.EqualTolPct <= 0 {
		p.EqualTolPct = 0.02
	}
	if p.BreakPct <= 0 {
		p.BreakPct = 0.005
	}
	if p.StopBufferPct <= 0 {
		p.StopBufferPct = 0.005
	}
	if p.TargetMultiple <= 0 {
		p.TargetMultiple = 1.0
	}

	// Default enable set: all
	if !p.EnableMTop && !p.EnableWBottom && !p.EnableHSTop && !p.EnableHSBottom &&
		!p.EnableTriangleBreakout && !p.EnableTriangleBreakdown && !p.EnableWaveUp && !p.EnableWaveDown {
		p.EnableMTop = true
		p.EnableWBottom = true
		p.EnableHSTop = true
		p.EnableHSBottom = true
		p.EnableTriangleBreakout = true
		p.EnableTriangleBreakdown = true
		p.EnableWaveUp = true
		p.EnableWaveDown = true
	}

	if p.TriangleLookback <= 0 {
		p.TriangleLookback = 160
	}
	if p.TriangleMinPivots <= 0 {
		p.TriangleMinPivots = 3
	}
	if p.TriangleMinBreakFrac <= 0 {
		p.TriangleMinBreakFrac = 0.66
	}
	if p.TriangleMaxBreakFrac <= 0 || p.TriangleMaxBreakFrac > 0.95 {
		p.TriangleMaxBreakFrac = 0.75
	}
	if p.TriangleMinBreakFrac >= p.TriangleMaxBreakFrac {
		p.TriangleMinBreakFrac = 0.66
		p.TriangleMaxBreakFrac = 0.75
	}

	return p
}

type tradePlan struct {
	side   Side
	target float64
	stop   float64
	reason string
}

type PatternsStrategy struct {
	p PatternsParams

	lastPos     Side
	pendingPlan *tradePlan
	pendingAge  int
	activePlan  *tradePlan
}

func NewPatternsStrategy(p PatternsParams) *PatternsStrategy {
	pp := p.withDefaults()
	return &PatternsStrategy{p: pp, lastPos: SideFlat}
}

func (s *PatternsStrategy) Clone() Strategy {
	return NewPatternsStrategy(s.p)
}

func (s *PatternsStrategy) OnBar(i int, bars []Bar, pos Position) *Signal {
	if i <= 0 {
		return nil
	}
	// pending plan should fill next bar open; otherwise drop.
	if pos.Side == SideFlat && s.pendingPlan != nil {
		s.pendingAge++
		if s.pendingAge > 2 {
			s.pendingPlan = nil
			s.pendingAge = 0
		}
	}

	// Promote pending -> active when position becomes non-flat
	if pos.Side != SideFlat {
		if s.lastPos == SideFlat && s.pendingPlan != nil && s.pendingPlan.side == pos.Side {
			s.activePlan = s.pendingPlan
			s.pendingPlan = nil
			s.pendingAge = 0
		}
		s.lastPos = pos.Side
	} else {
		s.lastPos = SideFlat
		s.activePlan = nil
	}

	// Exits
	if pos.Side != SideFlat && s.activePlan != nil {
		closePx := bars[i].Close
		if closePx <= 0 {
			return nil
		}
		switch pos.Side {
		case SideLong:
			if s.activePlan.target > 0 && closePx >= s.activePlan.target {
				return &Signal{Time: bars[i].Time, Action: SignalSell, Reason: s.activePlan.reason + "|target"}
			}
			if s.activePlan.stop > 0 && closePx <= s.activePlan.stop {
				return &Signal{Time: bars[i].Time, Action: SignalSell, Reason: s.activePlan.reason + "|stop"}
			}
		case SideShort:
			if s.activePlan.target > 0 && closePx <= s.activePlan.target {
				return &Signal{Time: bars[i].Time, Action: SignalCover, Reason: s.activePlan.reason + "|target"}
			}
			if s.activePlan.stop > 0 && closePx >= s.activePlan.stop {
				return &Signal{Time: bars[i].Time, Action: SignalCover, Reason: s.activePlan.reason + "|stop"}
			}
		}
		return nil
	}

	// Entries
	if pos.Side == SideFlat {
		plan := s.detect(i, bars)
		if plan == nil {
			return nil
		}
		s.pendingPlan = plan
		s.pendingAge = 0
		switch plan.side {
		case SideLong:
			return &Signal{Time: bars[i].Time, Action: SignalBuy, Reason: plan.reason}
		case SideShort:
			return &Signal{Time: bars[i].Time, Action: SignalShort, Reason: plan.reason}
		default:
			return nil
		}
	}

	return nil
}

func (s *PatternsStrategy) detect(i int, bars []Bar) *tradePlan {
	start := i - s.p.Lookback
	if start < 0 {
		start = 0
	}
	pivots := collectPivotsAll(bars, start, i, s.p.PivotN)

	// Priority: HS > M/W > Triangle > Wave
	if s.p.EnableHSTop {
		if p := detectHSTop(i, bars, pivots, s.p); p != nil {
			return p
		}
	}
	if s.p.EnableHSBottom {
		if p := detectHSBottom(i, bars, pivots, s.p); p != nil {
			return p
		}
	}
	if s.p.EnableMTop {
		if p := detectMTop(i, bars, pivots, s.p); p != nil {
			return p
		}
	}
	if s.p.EnableWBottom {
		if p := detectWBottom(i, bars, pivots, s.p); p != nil {
			return p
		}
	}
	if s.p.EnableTriangleBreakout || s.p.EnableTriangleBreakdown {
		if p := detectTriangle(i, bars, s.p); p != nil {
			return p
		}
	}
	if s.p.EnableWaveUp {
		if p := detectWaveUp(i, bars, pivots, s.p); p != nil {
			return p
		}
	}
	if s.p.EnableWaveDown {
		if p := detectWaveDown(i, bars, pivots, s.p); p != nil {
			return p
		}
	}
	return nil
}

func validatePlan(p *tradePlan) *tradePlan {
	if p == nil {
		return nil
	}
	if p.target <= 0 || p.stop <= 0 {
		return nil
	}
	if p.side == SideLong && p.target <= p.stop {
		return nil
	}
	if p.side == SideShort && p.target >= p.stop {
		return nil
	}
	if math.IsNaN(p.target) || math.IsNaN(p.stop) || math.IsInf(p.target, 0) || math.IsInf(p.stop, 0) {
		return nil
	}
	if p.reason == "" {
		p.reason = fmt.Sprintf("pattern_%s", p.side)
	}
	return p
}
