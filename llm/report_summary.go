package llm

import (
	"encoding/json"
	"fmt"
	"sort"

	"stock/backtest"
)

type ReportSummary struct {
	TotalSymbols int                 `json:"total_symbols"`
	TotalTrades  int                 `json:"total_trades"`
	TotalWins    int                 `json:"total_wins"`
	AvgWinRate   float64             `json:"avg_win_rate_pct"`
	OverallWinRate float64           `json:"overall_win_rate_pct"`
	WorstMaxDD   float64             `json:"worst_max_drawdown_pct"`
	TotalNetPnL  float64             `json:"total_net_pnl"`
	TopByPnL     []SymbolPerformance `json:"top_by_net_pnl"`
	BottomByPnL  []SymbolPerformance `json:"bottom_by_net_pnl"`
	Symbols      []SymbolPerformance `json:"symbols"`
	Errors       []string            `json:"errors,omitempty"`
}

type SymbolPerformance struct {
	Symbol      string  `json:"symbol"`
	Instrument  string  `json:"instrument"`
	FinalEquity float64 `json:"final_equity"`
	TotalTrades int     `json:"total_trades"`
	WinRatePct  float64 `json:"win_rate_pct"`
	MaxDDPct    float64 `json:"max_drawdown_pct"`
	NetPnL      float64 `json:"net_pnl"`
	AvgTradeNetPnL float64 `json:"avg_trade_net_pnl"`
	BestTradeNetPnL float64 `json:"best_trade_net_pnl"`
	WorstTradeNetPnL float64 `json:"worst_trade_net_pnl"`
	Start       string  `json:"start,omitempty"`
	End         string  `json:"end,omitempty"`
	Error       string  `json:"error,omitempty"`
}

func SummarizeReport(results []backtest.Result) ReportSummary {
	var sum ReportSummary
	sum.TotalSymbols = len(results)

	perfs := make([]SymbolPerformance, 0, len(results))
	totalTrades := 0
	winRateSum := 0.0
	totalWins := 0
	worstDD := 0.0
	totalNetPnL := 0.0
	var errors []string

	for _, r := range results {
		p := SymbolPerformance{
			Symbol:      r.Symbol,
			Instrument:  r.Instrument,
			FinalEquity: r.FinalEquity,
			TotalTrades: r.TotalTrades,
			WinRatePct:  r.WinRatePct,
			MaxDDPct:    r.MaxDDPct,
		}
		if len(r.Errors) > 0 {
			p.Error = r.Errors[0]
			errors = append(errors, fmt.Sprintf("%s: %s", r.Symbol, r.Errors[0]))
		}
		if n := len(r.EquityCurve); n > 0 {
			p.Start = r.EquityCurve[0].Time
			p.End = r.EquityCurve[n-1].Time
		}
		net := 0.0
		best := 0.0
		worst := 0.0
		hasTrade := false
		for _, t := range r.Trades {
			net += t.NetPnL
			if !hasTrade {
				best = t.NetPnL
				worst = t.NetPnL
				hasTrade = true
			} else {
				if t.NetPnL > best {
					best = t.NetPnL
				}
				if t.NetPnL < worst {
					worst = t.NetPnL
				}
			}
			if t.NetPnL > 0 {
				totalWins++
			}
		}
		p.NetPnL = net
		if r.TotalTrades > 0 {
			p.AvgTradeNetPnL = net / float64(r.TotalTrades)
		}
		if hasTrade {
			p.BestTradeNetPnL = best
			p.WorstTradeNetPnL = worst
		}
		perfs = append(perfs, p)

		totalTrades += r.TotalTrades
		winRateSum += r.WinRatePct
		if r.MaxDDPct > worstDD {
			worstDD = r.MaxDDPct
		}
		totalNetPnL += net
	}

	sum.TotalTrades = totalTrades
	sum.TotalWins = totalWins
	if len(results) > 0 {
		sum.AvgWinRate = winRateSum / float64(len(results))
	}
	if totalTrades > 0 {
		sum.OverallWinRate = float64(totalWins) / float64(totalTrades) * 100
	}
	sum.WorstMaxDD = worstDD
	sum.TotalNetPnL = totalNetPnL
	sum.Symbols = perfs
	sum.Errors = errors

	byPnL := append([]SymbolPerformance(nil), perfs...)
	sort.Slice(byPnL, func(i, j int) bool { return byPnL[i].NetPnL > byPnL[j].NetPnL })
	if len(byPnL) > 5 {
		sum.TopByPnL = byPnL[:5]
	} else {
		sum.TopByPnL = byPnL
	}

	byPnLAsc := append([]SymbolPerformance(nil), perfs...)
	sort.Slice(byPnLAsc, func(i, j int) bool { return byPnLAsc[i].NetPnL < byPnLAsc[j].NetPnL })
	if len(byPnLAsc) > 5 {
		sum.BottomByPnL = byPnLAsc[:5]
	} else {
		sum.BottomByPnL = byPnLAsc
	}

	return sum
}

func (s ReportSummary) MarshalIndented() ([]byte, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return b, nil
}
