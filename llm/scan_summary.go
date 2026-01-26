package llm

import (
	"encoding/json"
	"sort"

	"stock/backtest"
)

type ScanSummary struct {
	TotalSymbols int                   `json:"total_symbols"`
	Signals      int                   `json:"signals"`
	AsOfDates    []string              `json:"as_of_dates"`
	Items        []backtest.ScanResult `json:"items"`
}

func SummarizeScan(results []backtest.ScanResult) ScanSummary {
	sum := ScanSummary{
		TotalSymbols: len(results),
		Items:        results,
	}

	dateSet := map[string]struct{}{}
	signals := 0
	for _, r := range results {
		if r.NextAction != "" {
			signals++
		}
		if r.LastDate != "" && r.LastDate != "-" {
			dateSet[r.LastDate] = struct{}{}
		}
	}
	sum.Signals = signals

	var dates []string
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	sum.AsOfDates = dates

	return sum
}

func (s ScanSummary) MarshalIndented() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
