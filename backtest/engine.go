package backtest

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"stock/fetcher"
)

type Strategy interface {
	OnBar(i int, bars []Bar, pos Position) *Signal
	Clone() Strategy
}

type Result struct {
	Symbol      string   `json:"symbol"`
	Instrument  string   `json:"instrument"`
	Trades      []Trade  `json:"trades"`
	FinalEquity float64  `json:"final_equity"`
	MaxDDPct    float64  `json:"max_drawdown_pct"`
	WinRatePct  float64  `json:"win_rate_pct"`
	TotalTrades int      `json:"total_trades"`
	EquityCurve []Point  `json:"equity_curve"`
	Errors      []string `json:"errors,omitempty"`
}

type Point struct {
	Time   string  `json:"time"`
	Equity float64 `json:"equity"`
}

type Runner struct {
	klineFetcher *fetcher.KLineFetcher
}

func NewRunner() *Runner {
	return &Runner{klineFetcher: fetcher.NewKLineFetcher()}
}

func (r *Runner) Run(cfg RunConfig) ([]Result, error) {
	if len(cfg.Instruments) == 0 {
		return nil, fmt.Errorf("no instruments configured")
	}

	var out []Result
	for _, inst := range cfg.Instruments {
		bars, err := r.loadBars(inst, cfg)
		if err != nil {
			out = append(out, Result{
				Symbol:     inst.Symbol,
				Instrument: string(inst.Type),
				Errors:     []string{err.Error()},
			})
			continue
		}
		res := runOne(inst, bars, cfg)
		out = append(out, res)
	}
	return out, nil
}

func (r *Runner) loadBars(inst Instrument, cfg RunConfig) ([]Bar, error) {
	var kl []fetcher.KLine
	var err error

	switch inst.Type {
	case InstrumentTypeStock:
		kl, err = r.klineFetcher.FetchStockKLine(inst.Symbol, cfg.Days)
	case InstrumentTypeFutures:
		kl, err = r.klineFetcher.FetchFuturesKLine(inst.Symbol, cfg.Days)
	default:
		return nil, fmt.Errorf("unknown instrument type: %s", inst.Type)
	}
	if err != nil {
		return nil, err
	}

	bars := make([]Bar, 0, len(kl))
	for _, k := range kl {
		t, err := time.ParseInLocation("2006-01-02", k.Date, time.Local)
		if err != nil {
			continue
		}
		if !cfg.Start.IsZero() && t.Before(cfg.Start) {
			continue
		}
		if !cfg.End.IsZero() && t.After(cfg.End) {
			continue
		}
		bars = append(bars, Bar{
			Time:   t,
			Open:   k.Open,
			High:   k.High,
			Low:    k.Low,
			Close:  k.Close,
			Volume: k.Volume,
		})
	}

	sort.Slice(bars, func(i, j int) bool { return bars[i].Time.Before(bars[j].Time) })
	if len(bars) < 50 {
		return nil, fmt.Errorf("not enough bars: %d", len(bars))
	}
	return bars, nil
}

func runOne(inst Instrument, bars []Bar, cfg RunConfig) Result {
	strategy := cfg.Strategy.Clone()

	cash := cfg.InitialCash
	var pos Position
	pos.Side = SideFlat

	var trades []Trade
	var pending *Signal
	var lastEntryReason string

	equityCurve := make([]Point, 0, len(bars))
	peakEquity := cash
	maxDD := 0.0

	for i := 0; i < len(bars); i++ {
		bar := bars[i]

		// Execute pending order at next bar open (close-confirm model)
		if pending != nil {
			if i == 0 {
				// can't execute on first bar
			} else if bars[i].Time.Equal(pending.Time) {
				// safety: should never happen (pending.Time uses signal bar time)
			} else if i >= 1 && bars[i-1].Time.Equal(pending.Time) {
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
							lastEntryReason = pending.Reason
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
						trades = append(trades, closeTrade(inst, pos, bars[i].Time, execPrice, fee, lastEntryReason, pending.Reason))
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
							lastEntryReason = pending.Reason
						}
					}
				case SignalCover:
					if pos.Side == SideShort && execPrice > 0 {
						fee = (execPrice * pos.Qty * multiplier(inst)) * (cfg.CommissionBps / 10000.0)
						cash += pos.Margin + settlePnL(inst, pos, execPrice) - fee
						trades = append(trades, closeTrade(inst, pos, bars[i].Time, execPrice, fee, lastEntryReason, pending.Reason))
						pos = Position{Side: SideFlat}
					}
				}

				pending = nil
			}
		}

		// Generate new signal at close
		sig := strategy.OnBar(i, bars, pos)
		if sig != nil && i+1 < len(bars) {
			pending = sig
		}

		equity := cash + markToMarket(inst, pos, bar.Close)
		equityCurve = append(equityCurve, Point{Time: bar.Time.Format("2006-01-02"), Equity: equity})

		if equity > peakEquity {
			peakEquity = equity
		}
		if peakEquity > 0 {
			dd := (peakEquity - equity) / peakEquity
			if dd > maxDD {
				maxDD = dd
			}
		}
	}

	// Force close at last close
	if pos.Side != SideFlat {
		last := bars[len(bars)-1]
		exitPrice := last.Close
		fee := (exitPrice * pos.Qty * multiplier(inst)) * (cfg.CommissionBps / 10000.0)
		trades = append(trades, closeTrade(inst, pos, last.Time, exitPrice, fee, lastEntryReason, "force_close_end"))
		if inst.Type == InstrumentTypeFutures {
			cash = cash + pos.Margin + settlePnL(inst, pos, exitPrice) - fee
		} else {
			cash = cash + exitPrice*pos.Qty*multiplier(inst) - fee
		}
		pos = Position{Side: SideFlat}
	}

	finalEquity := cash
	if len(equityCurve) > 0 {
		finalEquity = equityCurve[len(equityCurve)-1].Equity
	}

	win := 0
	for _, t := range trades {
		if t.NetPnL > 0 {
			win++
		}
	}
	winRate := 0.0
	if len(trades) > 0 {
		winRate = float64(win) / float64(len(trades)) * 100
	}

	return Result{
		Symbol:      inst.Symbol,
		Instrument:  string(inst.Type),
		Trades:      trades,
		FinalEquity: round2(finalEquity),
		MaxDDPct:    round2(maxDD * 100),
		WinRatePct:  round2(winRate),
		TotalTrades: len(trades),
		EquityCurve: equityCurve,
	}
}

func WriteResultsJSON(w io.Writer, results []Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func applySlippage(price, bps float64, action SignalAction) float64 {
	if price <= 0 || bps <= 0 {
		return price
	}
	x := bps / 10000.0
	switch action {
	case SignalBuy, SignalCover:
		return price * (1 + x)
	case SignalSell, SignalShort:
		return price * (1 - x)
	default:
		return price
	}
}

func multiplier(inst Instrument) float64 {
	if inst.Type == InstrumentTypeFutures && inst.Multiplier > 0 {
		return inst.Multiplier
	}
	return 1
}

func sizeQty(inst Instrument, cash, price, pct, futuresMargin float64) float64 {
	if cash <= 0 || price <= 0 || pct <= 0 {
		return 0
	}
	target := cash * pct
	switch inst.Type {
	case InstrumentTypeStock:
		lot := inst.LotSize
		if lot <= 0 {
			lot = 100
		}
		q := math.Floor(target / price / float64(lot))
		return q * float64(lot)
	case InstrumentTypeFutures:
		m := futuresMargin
		if m <= 0 || m > 1 {
			m = 1
		}
		q := math.Floor(target / (price * multiplier(inst) * m))
		return q
	default:
		return 0
	}
}

func markToMarket(inst Instrument, pos Position, closePrice float64) float64 {
	if pos.Side == SideFlat || pos.Qty <= 0 || closePrice <= 0 {
		return 0
	}
	switch inst.Type {
	case InstrumentTypeStock:
		return closePrice * pos.Qty
	case InstrumentTypeFutures:
		return pos.Margin + settlePnL(inst, pos, closePrice)
	default:
		return 0
	}
}

func settlePnL(inst Instrument, pos Position, exitPrice float64) float64 {
	if pos.Side == SideFlat || pos.Qty <= 0 || exitPrice <= 0 {
		return 0
	}
	d := 1.0
	if pos.Side == SideShort {
		d = -1
	}
	return (exitPrice - pos.EntryPrice) * d * pos.Qty * multiplier(inst)
}

func closeTrade(inst Instrument, pos Position, exitTime time.Time, exitPrice float64, exitFee float64, entryReason, exitReason string) Trade {
	gross := settlePnL(inst, pos, exitPrice)
	net := gross - pos.EntryFee - exitFee
	retPct := 0.0
	if pos.EntryPrice > 0 {
		retPct = (exitPrice - pos.EntryPrice) / pos.EntryPrice * 100.0
		if pos.Side == SideShort {
			retPct = -retPct
		}
	}
	return Trade{
		Symbol:      inst.Symbol,
		Side:        pos.Side,
		EntryTime:   pos.EntryTime.Format("2006-01-02"),
		EntryPrice:  round2(pos.EntryPrice),
		ExitTime:    exitTime.Format("2006-01-02"),
		ExitPrice:   round2(exitPrice),
		Qty:         round2(pos.Qty),
		GrossPnL:    round2(gross),
		NetPnL:      round2(net),
		ReturnPct:   round2(retPct),
		ReasonEntry: entryReason,
		ReasonExit:  exitReason,
	}
}

func round2(x float64) float64 {
	return math.Round(x*100) / 100
}
