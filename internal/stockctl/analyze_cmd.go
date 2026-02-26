package stockctl

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"stock/backtest"
	appcfg "stock/config"
)

type analysisReport struct {
	GeneratedAt string `json:"generated_at"`

	Window struct {
		Mode      string `json:"mode"` // "days" | "bars"
		Days      int    `json:"days,omitempty"`
		Bars      int    `json:"bars,omitempty"`
		StartDate string `json:"start_date,omitempty"`
		EndDate   string `json:"end_date,omitempty"`
	} `json:"window"`

	Results []*instrumentAnalysis `json:"results"`

	Skipped []struct {
		Symbol string `json:"symbol"`
		Reason string `json:"reason"`
	} `json:"skipped,omitempty"`
}

type instrumentAnalysis struct {
	Symbol     string `json:"symbol"`
	Name       string `json:"name,omitempty"`
	Instrument string `json:"instrument"` // stock|futures

	BarsCount int    `json:"bars_count,omitempty"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`

	Support    float64 `json:"support,omitempty"`
	Resistance float64 `json:"resistance,omitempty"`

	VolumeMAN       int     `json:"volume_ma_n,omitempty"`
	LastVolume      int64   `json:"last_volume,omitempty"`
	LastVolumeMA    float64 `json:"last_volume_ma,omitempty"`
	LastVolumeRatio float64 `json:"last_volume_ratio,omitempty"`

	Latest backtest.ScanResult `json:"latest"`

	YearStats *backtest.Result `json:"year_stats,omitempty"`

	ChartPath string   `json:"chart_path,omitempty"`
	Errors    []string `json:"errors,omitempty"`
}

func runAnalyze(serviceConfigPath, btConfigPath, outDir string, windowDays int, bars int) error {
	if strings.TrimSpace(outDir) == "" {
		outDir = "runtime/analysis"
	}
	chartsDir := filepath.Join(outDir, "charts")
	if err := os.MkdirAll(chartsDir, 0o755); err != nil {
		return err
	}

	// Load symbols from config.yaml
	cfgPath := strings.TrimSpace(serviceConfigPath)
	if cfgPath == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			cfgPath = "config.yaml"
		}
	}
	if cfgPath == "" {
		return fmt.Errorf("missing -config (and ./config.yaml not found)")
	}
	svcCfg, err := appcfg.LoadFromFile(cfgPath)
	if err != nil {
		return err
	}
	stocks := append([]string(nil), svcCfg.Stocks...)
	futures := filterChinaFutures(svcCfg.Futures)

	// Load backtest config (strategy params source)
	btCfg, err := backtest.LoadRunConfig(btConfigPath)
	if err != nil {
		return err
	}

	ts, ok := btCfg.Strategy.(*backtest.TsaiSenStrategy)
	if !ok {
		return fmt.Errorf("strategy.type must be tsai_sen for -analyze (got %T)", btCfg.Strategy)
	}
	tsParams := ts.Params()

	// Merge instruments: bt-config + service config
	btCfg.Instruments = mergeInstruments(btCfg.Instruments, stocks, futures)

	// Apply analysis window
	now := time.Now().In(time.Local)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	mode := "days"
	if bars > 0 {
		mode = "bars"
		btCfg.Days = bars
		btCfg.Start = time.Time{}
		btCfg.End = time.Time{}
	} else {
		if windowDays <= 0 {
			windowDays = 365
		}
		start := end.AddDate(0, 0, -windowDays)
		btCfg.Start = start
		btCfg.End = end
		need := windowDays + 200
		if need > btCfg.Days {
			btCfg.Days = need
		}
	}

	runner := backtest.NewRunner()

	// Latest scan snapshot (no chart)
	scanCfg := btCfg
	scanCfg.ScanChart = false
	scanCfg.ScanChartDir = ""
	scanCfg.ScanChartBars = 0
	scanResults, err := runner.Scan(scanCfg)
	if err != nil {
		return err
	}
	scanResults = enrichScanNames(scanResults)
	scanBySym := map[string]backtest.ScanResult{}
	for _, r := range scanResults {
		if r.Symbol == "" {
			continue
		}
		scanBySym[r.Symbol] = r
	}

	// Year backtest stats (per instrument; keeps errors local)
	report := analysisReport{
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
	report.Window.Mode = mode
	if mode == "bars" {
		report.Window.Bars = bars
	} else {
		report.Window.Days = windowDays
		report.Window.StartDate = btCfg.Start.Format("2006-01-02")
		report.Window.EndDate = btCfg.End.Format("2006-01-02")
	}

	results := make([]*instrumentAnalysis, 0, len(btCfg.Instruments))
	for _, inst := range btCfg.Instruments {
		// backtest/scan are China-only; ignore hf_ just in case.
		if strings.HasPrefix(strings.ToLower(inst.Symbol), "hf_") {
			report.Skipped = append(report.Skipped, struct {
				Symbol string `json:"symbol"`
				Reason string `json:"reason"`
			}{Symbol: inst.Symbol, Reason: "hf_ realtime-only; no daily KLine support for analysis"})
			continue
		}

		out := &instrumentAnalysis{
			Symbol:     inst.Symbol,
			Instrument: string(inst.Type),
			VolumeMAN:  tsParams.VolMAN,
		}

		if r, ok := scanBySym[inst.Symbol]; ok {
			out.Latest = r
			out.Name = r.Name
			out.Support = r.Support
			out.Resistance = r.Resistance
		} else {
			out.Errors = append(out.Errors, "missing scan snapshot")
		}

		// Load bars for chart + volume stats.
		barsOne, berr := runner.LoadBars(inst, btCfg)
		if berr != nil {
			out.Errors = append(out.Errors, berr.Error())
			results = append(results, out)
			continue
		}
		out.BarsCount = len(barsOne)
		out.StartDate = barsOne[0].Time.Format("2006-01-02")
		out.EndDate = barsOne[len(barsOne)-1].Time.Format("2006-01-02")

		lastIdx := len(barsOne) - 1
		out.LastVolume = barsOne[lastIdx].Volume
		ma := backtest.VolumeMA(barsOne, lastIdx, tsParams.VolMAN)
		out.LastVolumeMA = round2(ma)
		if ma > 0 {
			out.LastVolumeRatio = round2(float64(out.LastVolume) / ma)
		}

		// Chart
		lines := make([]backtest.ChartLine, 0, 8)
		if out.Support > 0 {
			lines = append(lines, backtest.ChartLine{Price: out.Support, Label: "Support", Color: "rgba(34,197,94,0.85)", Dash: false})
		}
		if out.Resistance > 0 {
			lines = append(lines, backtest.ChartLine{Price: out.Resistance, Label: "Resistance", Color: "rgba(239,68,68,0.85)", Dash: false})
		}
		if out.Latest.SuggestedStop > 0 {
			lines = append(lines, backtest.ChartLine{Price: out.Latest.SuggestedStop, Label: "Stop", Color: "rgba(148,163,184,0.85)", Dash: true})
		}
		if out.Latest.SuggestedTarget > 0 {
			lines = append(lines, backtest.ChartLine{Price: out.Latest.SuggestedTarget, Label: "Target", Color: "rgba(56,189,248,0.85)", Dash: true})
		}
		if out.Latest.EntryPrice > 0 {
			lines = append(lines, backtest.ChartLine{Price: out.Latest.EntryPrice, Label: "Entry", Color: "rgba(245,158,11,0.85)", Dash: true})
		}

		var points []backtest.ChartPoint
		if out.Latest.NextAction != "" {
			points = append(points, backtest.ChartPoint{
				Date:  out.EndDate,
				Price: barsOne[lastIdx].Close,
				Label: strings.ToUpper(string(out.Latest.NextAction)),
				Color: "#a78bfa",
			})
		}

		svg, serr := backtest.RenderCandlesWithVolumeSVG(inst.Symbol, barsOne, lines, points, tsParams.VolMAN, backtest.SVGChartOptions{})
		if serr != nil {
			out.Errors = append(out.Errors, serr.Error())
		} else {
			fn := sanitizeFilename(inst.Symbol) + ".svg"
			p := filepath.Join(chartsDir, fn)
			if werr := os.WriteFile(p, svg, 0o644); werr != nil {
				out.Errors = append(out.Errors, werr.Error())
			} else {
				out.ChartPath = p
			}
		}

		// Backtest year stats for this instrument only.
		btOne := btCfg
		btOne.Instruments = []backtest.Instrument{inst}
		stats, rerr := runner.Run(btOne)
		if rerr != nil {
			out.Errors = append(out.Errors, rerr.Error())
		} else if len(stats) > 0 {
			// Runner returns errors per instrument, so check stats[0].Errors as well.
			res := stats[0]
			out.YearStats = &res
			if len(res.Errors) > 0 {
				out.Errors = append(out.Errors, res.Errors...)
			}
		}

		results = append(results, out)
	}

	// Stable order
	sort.Slice(results, func(i, j int) bool {
		if results[i].Instrument != results[j].Instrument {
			return results[i].Instrument < results[j].Instrument
		}
		return results[i].Symbol < results[j].Symbol
	})
	report.Results = results

	// Write JSON
	if err := writeJSON(filepath.Join(outDir, "analysis.json"), report); err != nil {
		return err
	}
	// Write HTML viewer
	if err := writeAnalysisHTML(outDir, report); err != nil {
		return err
	}
	// Write CSVs
	if err := writeAnalysisCSV(filepath.Join(outDir, "analysis.csv"), report); err != nil {
		return err
	}
	if err := writeTradesCSV(filepath.Join(outDir, "trades.csv"), report); err != nil {
		return err
	}

	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := ensureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func writeAnalysisCSV(path string, rep analysisReport) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{
		"symbol", "name", "instrument",
		"start_date", "end_date", "bars_count",
		"last_date", "last_close", "position", "latest_signal", "reason",
		"support", "resistance", "stop", "target",
		"vol_ma_n", "last_volume", "last_volume_ma", "last_volume_ratio",
		"final_equity", "max_dd_pct", "win_rate_pct", "total_trades",
		"chart_path", "errors",
	})

	for _, r := range rep.Results {
		lastClose := ""
		if r.Latest.LastClose > 0 {
			lastClose = fmt.Sprintf("%.2f", r.Latest.LastClose)
		}
		stop := ""
		if r.Latest.SuggestedStop > 0 {
			stop = fmt.Sprintf("%.2f", r.Latest.SuggestedStop)
		}
		target := ""
		if r.Latest.SuggestedTarget > 0 {
			target = fmt.Sprintf("%.2f", r.Latest.SuggestedTarget)
		}

		finalEquity := ""
		maxDD := ""
		winRate := ""
		totalTrades := ""
		if r.YearStats != nil {
			finalEquity = fmt.Sprintf("%.2f", r.YearStats.FinalEquity)
			maxDD = fmt.Sprintf("%.2f", r.YearStats.MaxDDPct)
			winRate = fmt.Sprintf("%.2f", r.YearStats.WinRatePct)
			totalTrades = fmt.Sprintf("%d", r.YearStats.TotalTrades)
		}

		_ = w.Write([]string{
			r.Symbol,
			r.Name,
			r.Instrument,
			r.StartDate,
			r.EndDate,
			fmt.Sprintf("%d", r.BarsCount),
			r.Latest.LastDate,
			lastClose,
			string(r.Latest.PositionSide),
			string(r.Latest.NextAction),
			r.Latest.Reason,
			fmt.Sprintf("%.2f", r.Support),
			fmt.Sprintf("%.2f", r.Resistance),
			stop,
			target,
			fmt.Sprintf("%d", r.VolumeMAN),
			fmt.Sprintf("%d", r.LastVolume),
			fmt.Sprintf("%.2f", r.LastVolumeMA),
			fmt.Sprintf("%.2f", r.LastVolumeRatio),
			finalEquity,
			maxDD,
			winRate,
			totalTrades,
			r.ChartPath,
			strings.Join(r.Errors, " | "),
		})
	}
	return w.Error()
}

func writeTradesCSV(path string, rep analysisReport) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{
		"symbol", "instrument", "side",
		"entry_time", "entry_price",
		"exit_time", "exit_price",
		"qty", "gross_pnl", "net_pnl", "return_pct",
		"reason_entry", "reason_exit",
	})

	for _, r := range rep.Results {
		if r.YearStats == nil {
			continue
		}
		for _, t := range r.YearStats.Trades {
			_ = w.Write([]string{
				t.Symbol,
				r.Instrument,
				string(t.Side),
				t.EntryTime,
				fmt.Sprintf("%.2f", t.EntryPrice),
				t.ExitTime,
				fmt.Sprintf("%.2f", t.ExitPrice),
				fmt.Sprintf("%.4f", t.Qty),
				fmt.Sprintf("%.2f", t.GrossPnL),
				fmt.Sprintf("%.2f", t.NetPnL),
				fmt.Sprintf("%.2f", t.ReturnPct),
				t.ReasonEntry,
				t.ReasonExit,
			})
		}
	}

	return w.Error()
}

func sanitizeFilename(s string) string {
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

func round2(x float64) float64 {
	if x == 0 {
		return 0
	}
	v := float64(int64(x*100+0.5)) / 100
	if x < 0 {
		v = float64(int64(x*100-0.5)) / 100
	}
	return v
}
