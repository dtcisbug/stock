package stockctl

import (
	"fmt"
	"time"

	"stock/backtest"
)

// applyScanDays mutates cfg to use a rolling window of the last N calendar days (end=today).
// Returns a human-readable description for text outputs.
func applyScanDays(cfg *backtest.RunConfig, scanDays int) string {
	if cfg == nil || scanDays <= 0 {
		return ""
	}

	end := time.Now().In(time.Local)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	start := end.AddDate(0, 0, -scanDays)

	cfg.Start = start
	cfg.End = end

	// Ensure we fetch enough bars for the time-window filter (non-trading days exist).
	need := scanDays + 200
	if need > cfg.Days {
		cfg.Days = need
	}

	return fmt.Sprintf("[SCAN] window: %s ~ %s (last %d days, close-confirm -> next open exec)", start.Format("2006-01-02"), end.Format("2006-01-02"), scanDays)
}
