package realtime

import (
	"log"
	"strings"
	"time"

	"stock/cache"
	"stock/config"
	"stock/fetcher"
	"stock/trading"
)

type Logger interface {
	Printf(format string, v ...any)
}

type SyncOptions struct {
	Logger Logger
	Quiet  bool
}

func RunDataSync(cfg *config.Config, c *cache.Cache, sf *fetcher.StockFetcher, ff *fetcher.FuturesFetcher, stop <-chan struct{}, opt SyncOptions) {
	logger := opt.Logger
	if logger == nil {
		logger = log.Default()
	}

	chinaFutures, globalFutures := splitFuturesCodes(cfg.Futures)

	// First fetch immediately.
	if !opt.Quiet {
		logger.Printf("[sync] initial fetch...")
	}
	fetchChinaData(cfg.Stocks, chinaFutures, c, sf, ff, logger, opt.Quiet)
	fetchGlobalFutures(globalFutures, c, ff, logger, opt.Quiet)

	refreshTicker := time.NewTicker(cfg.RefreshInterval) // China markets, only during trading time
	globalTicker := time.NewTicker(cfg.RefreshInterval)  // hf_ futures, always refresh
	checkTicker := time.NewTicker(cfg.CheckInterval)
	defer refreshTicker.Stop()
	defer globalTicker.Stop()
	defer checkTicker.Stop()

	for {
		select {
		case <-stop:
			if !opt.Quiet {
				logger.Printf("[sync] stop")
			}
			return

		case <-refreshTicker.C:
			if trading.IsTradingTime() {
				fetchChinaData(cfg.Stocks, chinaFutures, c, sf, ff, logger, opt.Quiet)
			}

		case <-globalTicker.C:
			if len(globalFutures) > 0 {
				fetchGlobalFutures(globalFutures, c, ff, logger, opt.Quiet)
			}

		case <-checkTicker.C:
			if opt.Quiet {
				continue
			}
			isStock := trading.IsStockTradingTime()
			isFutures := trading.IsFuturesTradingTime()
			if !isStock && !isFutures {
				logger.Printf("[sync] non-trading time (stock=%v futures=%v)", isStock, isFutures)
			}
		}
	}
}

func fetchChinaData(stocks []string, futures []string, c *cache.Cache, sf *fetcher.StockFetcher, ff *fetcher.FuturesFetcher, logger Logger, quiet bool) {
	if len(stocks) > 0 {
		quotes, err := sf.Fetch(stocks)
		if err != nil {
			if !quiet {
				logger.Printf("[sync] fetch stocks failed: %v", err)
			}
		} else {
			c.SetStocks(quotes)
			if !quiet {
				logger.Printf("[sync] stocks updated: %d", len(quotes))
			}
		}
	}

	if len(futures) > 0 {
		quotes, err := ff.Fetch(futures)
		if err != nil {
			if !quiet {
				logger.Printf("[sync] fetch futures failed: %v", err)
			}
		} else {
			c.SetFuturesList(quotes)
			if !quiet {
				logger.Printf("[sync] futures updated: %d", len(quotes))
			}
		}
	}
}

func fetchGlobalFutures(futures []string, c *cache.Cache, ff *fetcher.FuturesFetcher, logger Logger, quiet bool) {
	if len(futures) == 0 {
		return
	}
	quotes, err := ff.Fetch(futures)
	if err != nil {
		if !quiet {
			logger.Printf("[sync] fetch global futures failed: %v", err)
		}
		return
	}
	c.SetFuturesList(quotes)
	if !quiet {
		logger.Printf("[sync] global futures updated: %d", len(quotes))
	}
}

func HasGlobalFutures(codes []string) bool {
	for _, c := range codes {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(c)), "hf_") {
			return true
		}
	}
	return false
}

func splitFuturesCodes(codes []string) (china []string, global []string) {
	for _, c := range codes {
		s := strings.TrimSpace(c)
		if s == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(s), "hf_") {
			global = append(global, s)
			continue
		}
		china = append(china, s)
	}
	return china, global
}
