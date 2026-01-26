package main

import (
	"sort"
	"strings"

	"stock/backtest"
	"stock/fetcher"
)

func enrichScanNames(results []backtest.ScanResult) []backtest.ScanResult {
	var stockCodes []string
	var futuresCodes []string
	seenStock := map[string]struct{}{}
	seenFut := map[string]struct{}{}

	for _, r := range results {
		if r.Symbol == "" {
			continue
		}
		switch r.Instrument {
		case string(backtest.InstrumentTypeStock):
			if _, ok := seenStock[r.Symbol]; !ok {
				seenStock[r.Symbol] = struct{}{}
				stockCodes = append(stockCodes, r.Symbol)
			}
		case string(backtest.InstrumentTypeFutures):
			if _, ok := seenFut[r.Symbol]; !ok {
				seenFut[r.Symbol] = struct{}{}
				futuresCodes = append(futuresCodes, r.Symbol)
			}
		}
	}

	sort.Strings(stockCodes)
	sort.Strings(futuresCodes)

	names := map[string]string{}

	if len(stockCodes) > 0 {
		sf := fetcher.NewStockFetcher()
		if quotes, err := sf.Fetch(stockCodes); err == nil {
			for _, q := range quotes {
				if q == nil || q.Code == "" {
					continue
				}
				n := strings.TrimSpace(q.Name)
				if n != "" {
					names[q.Code] = n
				}
			}
		}
	}
	if len(futuresCodes) > 0 {
		ff := fetcher.NewFuturesFetcher()
		if quotes, err := ff.Fetch(futuresCodes); err == nil {
			for _, q := range quotes {
				if q == nil || q.Code == "" {
					continue
				}
				n := strings.TrimSpace(q.Name)
				if n != "" {
					names[q.Code] = n
				}
			}
		}
	}

	out := make([]backtest.ScanResult, 0, len(results))
	for _, r := range results {
		if r.Name == "" {
			if n, ok := names[r.Symbol]; ok {
				r.Name = n
			}
		}
		out = append(out, r)
	}
	return out
}
