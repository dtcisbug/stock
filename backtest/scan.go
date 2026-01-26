package backtest

import (
	"os"
	"path/filepath"
	"strings"
)

type ScanResult struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name,omitempty"`
	Instrument string  `json:"instrument"`
	LastDate   string  `json:"last_date"`
	LastClose  float64 `json:"last_close"`

	Support    float64 `json:"support,omitempty"`
	Resistance float64 `json:"resistance,omitempty"`

	PositionSide Side    `json:"position_side"`
	PositionQty  float64 `json:"position_qty"`
	EntryDate    string  `json:"entry_date,omitempty"`
	EntryPrice   float64 `json:"entry_price,omitempty"`

	NextAction      SignalAction `json:"next_action,omitempty"`
	Reason          string       `json:"reason,omitempty"`
	SuggestedStop   float64      `json:"suggested_stop,omitempty"`
	SuggestedTarget float64      `json:"suggested_target,omitempty"`

	ChartPath string `json:"chart_path,omitempty"`

	Errors []string `json:"errors,omitempty"`
}

func scanOne(inst Instrument, bars []Bar, cfg RunConfig) ScanResult {
	strategy := cfg.Strategy.Clone()

	cash := cfg.InitialCash
	var pos Position
	pos.Side = SideFlat

	var pending *Signal
	var lastSignal *Signal

	for i := 0; i < len(bars); i++ {
		// Execute pending order at next bar open (close-confirm model)
		if pending != nil && i >= 1 && bars[i-1].Time.Equal(pending.Time) {
			execPrice := applySlippage(bars[i].Open, cfg.SlippageBps, pending.Action)
			fee := 0.0

			switch pending.Action {
			case SignalBuy:
				if pos.Side == SideFlat && execPrice > 0 {
					qty := sizeQty(inst, cash, execPrice, cfg.PositionPct, cfg.FuturesMargin)
					if qty > 0 {
						notional := execPrice * qty * multiplier(inst)
						fee = notional * (cfg.CommissionBps / 10000.0)
						if inst.Type == InstrumentTypeFutures {
							margin := notional * cfg.FuturesMargin
							cash -= margin + fee
							pos = Position{Side: SideLong, Qty: qty, EntryTime: bars[i].Time, EntryPrice: execPrice, EntryFee: fee, Margin: margin}
						} else {
							cash -= notional + fee
							pos = Position{Side: SideLong, Qty: qty, EntryTime: bars[i].Time, EntryPrice: execPrice, EntryFee: fee}
						}
					}
				}
			case SignalSell:
				if pos.Side == SideLong && execPrice > 0 {
					notional := execPrice * pos.Qty * multiplier(inst)
					fee = notional * (cfg.CommissionBps / 10000.0)
					if inst.Type == InstrumentTypeFutures {
						cash += pos.Margin + settlePnL(inst, pos, execPrice) - fee
					} else {
						cash += notional - fee
					}
					pos = Position{Side: SideFlat}
				}
			case SignalShort:
				if inst.Type == InstrumentTypeFutures && inst.AllowShort && pos.Side == SideFlat && execPrice > 0 {
					qty := sizeQty(inst, cash, execPrice, cfg.PositionPct, cfg.FuturesMargin)
					if qty > 0 {
						notional := execPrice * qty * multiplier(inst)
						fee = notional * (cfg.CommissionBps / 10000.0)
						margin := notional * cfg.FuturesMargin
						cash -= margin + fee
						pos = Position{Side: SideShort, Qty: qty, EntryTime: bars[i].Time, EntryPrice: execPrice, EntryFee: fee, Margin: margin}
					}
				}
			case SignalCover:
				if pos.Side == SideShort && execPrice > 0 {
					fee = (execPrice * pos.Qty * multiplier(inst)) * (cfg.CommissionBps / 10000.0)
					cash += pos.Margin + settlePnL(inst, pos, execPrice) - fee
					pos = Position{Side: SideFlat}
				}
			}

			pending = nil
		}

		sig := strategy.OnBar(i, bars, pos)
		if sig != nil {
			lastSignal = sig
			if i+1 < len(bars) {
				pending = sig
			}
		}
	}

	last := bars[len(bars)-1]
	out := ScanResult{
		Symbol:       inst.Symbol,
		Instrument:   string(inst.Type),
		LastDate:     last.Time.Format("2006-01-02"),
		LastClose:    round2(last.Close),
		PositionSide: pos.Side,
		PositionQty:  round2(pos.Qty),
	}

	// Key levels at the latest bar (for context/overlay)
	switch st := strategy.(type) {
	case *TsaiSenStrategy:
		sup, res := levels(bars, len(bars)-1, st.p)
		if sup > 0 {
			out.Support = round2(sup)
		}
		if res > 0 {
			out.Resistance = round2(res)
		}
	}
	if pos.Side != SideFlat {
		out.EntryDate = pos.EntryTime.Format("2006-01-02")
		out.EntryPrice = round2(pos.EntryPrice)
	}
	// only care about latest bar's signal (next open execution)
	if lastSignal != nil && lastSignal.Time.Equal(last.Time) {
		out.NextAction = lastSignal.Action
		out.Reason = lastSignal.Reason

		// Best-effort stop/target extraction.
		switch st := strategy.(type) {
		case *PatternsStrategy:
			if st.pendingPlan != nil {
				out.SuggestedStop = round2(st.pendingPlan.stop)
				out.SuggestedTarget = round2(st.pendingPlan.target)
			}
		case *TsaiSenStrategy:
			if st.lastPlan != nil && st.lastPlan.time.Equal(last.Time) {
				out.SuggestedStop = round2(st.lastPlan.stop)
				out.SuggestedTarget = round2(st.lastPlan.target)
			}
		}
	}
	_ = cash // keep signature aligned with backtest simulation; future use: equity display
	return out
}

func (r *Runner) Scan(cfg RunConfig) ([]ScanResult, error) {
	if len(cfg.Instruments) == 0 {
		return nil, nil
	}

	var out []ScanResult
	chartDir := strings.TrimSpace(cfg.ScanChartDir)
	if cfg.ScanChart && chartDir == "" {
		chartDir = "scan_charts"
	}
	chartBars := cfg.ScanChartBars
	if chartBars <= 0 {
		chartBars = 220
	}

	for _, inst := range cfg.Instruments {
		bars, err := r.loadBars(inst, cfg)
		if err != nil {
			out = append(out, ScanResult{
				Symbol:     inst.Symbol,
				Instrument: string(inst.Type),
				Errors:     []string{err.Error()},
			})
			continue
		}
		if len(bars) == 0 {
			out = append(out, ScanResult{
				Symbol:     inst.Symbol,
				Instrument: string(inst.Type),
				Errors:     []string{"no bars"},
			})
			continue
		}

		res := scanOne(inst, bars, cfg)
		if cfg.ScanChart {
			_ = os.MkdirAll(chartDir, 0o755)

			view := bars
			if len(view) > chartBars {
				view = bars[len(bars)-chartBars:]
			}

			var lines []ChartLine
			if res.Support > 0 {
				lines = append(lines, ChartLine{Price: res.Support, Label: "Support", Color: "rgba(34,197,94,0.85)", Dash: false})
			}
			if res.Resistance > 0 {
				lines = append(lines, ChartLine{Price: res.Resistance, Label: "Resistance", Color: "rgba(239,68,68,0.85)", Dash: false})
			}
			if res.SuggestedStop > 0 {
				lines = append(lines, ChartLine{Price: res.SuggestedStop, Label: "Stop", Color: "rgba(148,163,184,0.85)", Dash: true})
			}
			if res.SuggestedTarget > 0 {
				lines = append(lines, ChartLine{Price: res.SuggestedTarget, Label: "Target", Color: "rgba(56,189,248,0.85)", Dash: true})
			}
			if res.EntryPrice > 0 {
				lines = append(lines, ChartLine{Price: res.EntryPrice, Label: "Entry", Color: "rgba(245,158,11,0.85)", Dash: true})
			}

			var points []ChartPoint
			if res.NextAction != "" {
				last := bars[len(bars)-1]
				points = append(points, ChartPoint{
					Date:  last.Time.Format("2006-01-02"),
					Price: last.Close,
					Label: string(res.NextAction),
					Color: "#a78bfa",
				})
			}

			svg, err := RenderCandlesSVG(inst.Symbol, view, lines, points, SVGChartOptions{})
			if err == nil && len(svg) > 0 {
				fn := sanitizeChartFilename(inst.Symbol) + ".svg"
				p := filepath.Join(chartDir, fn)
				if werr := os.WriteFile(p, svg, 0o644); werr == nil {
					res.ChartPath = p
				}
			}
		}

		out = append(out, res)
	}
	return out, nil
}

func sanitizeChartFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}
