package stockctl

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"stock/backtest"
)

func runScan(btConfigPath, serviceConfigPath, outPath string, jsonOut bool, onlySignal bool, scanDays int, scanChart bool, scanChartDir string, scanChartBars int) error {
	cfg, err := loadScanRunConfig(btConfigPath, serviceConfigPath)
	if err != nil {
		return err
	}
	window := applyScanDays(&cfg, scanDays)
	cfg.ScanChart = scanChart
	cfg.ScanChartDir = scanChartDir
	cfg.ScanChartBars = scanChartBars

	runner := backtest.NewRunner()
	results, err := runner.Scan(cfg)
	if err != nil {
		return err
	}
	results = enrichScanNames(results)
	if onlySignal {
		filtered := make([]backtest.ScanResult, 0, len(results))
		for _, r := range results {
			if len(r.Errors) > 0 || r.NextAction != "" {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	var w io.Writer = os.Stdout
	var f *os.File
	if strings.TrimSpace(outPath) != "" {
		if err := ensureParentDir(outPath); err != nil {
			return fmt.Errorf("prepare output dir: %w", err)
		}
		f, err = os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create output: %w", err)
		}
		defer f.Close()
		w = f
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	// text table
	if window != "" {
		fmt.Fprintln(w, window)
	}
	fmt.Fprintf(w, "%-10s %-10s %-12s %-10s %-8s %-10s %-10s %-10s %s\n", "SYMBOL", "NAME", "LAST_DATE", "LAST_CLOSE", "POS", "SIGNAL", "STOP", "TARGET", "REASON")
	for _, r := range results {
		if len(r.Errors) > 0 {
			fmt.Fprintf(w, "%-10s %-10s %-12s %-10s %-8s %-10s %-10s %-10s %s\n", r.Symbol, "-", "-", "-", "-", "ERROR", "-", "-", r.Errors[0])
			continue
		}
		sig := ""
		if r.NextAction != "" {
			sig = string(r.NextAction)
		} else {
			sig = "-"
		}
		stop := "-"
		target := "-"
		if r.SuggestedStop > 0 {
			stop = fmt.Sprintf("%.2f", r.SuggestedStop)
		}
		if r.SuggestedTarget > 0 {
			target = fmt.Sprintf("%.2f", r.SuggestedTarget)
		}
		name := r.Name
		if name == "" {
			name = "-"
		}
		fmt.Fprintf(w, "%-10s %-10s %-12s %-10.2f %-8s %-10s %-10s %-10s %s\n", r.Symbol, name, r.LastDate, r.LastClose, r.PositionSide, sig, stop, target, r.Reason)
		if r.PositionSide != backtest.SideFlat {
			fmt.Fprintf(w, "  entry: %s @ %.2f qty=%.2f\n", r.EntryDate, r.EntryPrice, r.PositionQty)
		}
		if strings.TrimSpace(r.ChartPath) != "" {
			fmt.Fprintf(w, "  chart: %s\n", r.ChartPath)
		}
	}
	return nil
}
