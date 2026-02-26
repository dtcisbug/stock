package stockctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"stock"
	"stock/analyzer"
	"stock/cache"
	"stock/config"
	"stock/fetcher"
	"stock/internal/realtime"
	"stock/internal/terminalui"
	"stock/model"
	"stock/trading"
)

func runCLI(serverURL, configPath string, enableAI bool, standalone bool) error {
	if standalone {
		return runStandaloneCLI(configPath, enableAI)
	}
	return runAPICLI(serverURL)
}

func runStandaloneCLI(configPath string, enableAI bool) error {
	if configPath == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		}
	}

	cfg := config.GetConfig(configPath)
	c := cache.NewCache()
	sf := fetcher.NewStockFetcher()
	ff := fetcher.NewFuturesFetcher()

	aiEnabled := enableAI || cfg.EnableAI
	var a *analyzer.ClaudeAnalyzer
	if aiEnabled && cfg.ClaudeAPIKey != "" {
		a = analyzer.NewClaudeAnalyzer(cfg.ClaudeAPIKey, cfg.ClaudeAPIBase, cfg.ClaudeModel)
		_ = a.LoadFromFile(stock.DefaultAIStorePath())
	}

	stop := make(chan struct{})
	go realtime.RunDataSync(cfg, c, sf, ff, stop, realtime.SyncOptions{Quiet: true})
	if a != nil && a.IsEnabled() {
		// Best-effort: run once in background; stockd has a scheduler, standalone CLI keeps it simple.
		go func() {
			time.Sleep(3 * time.Second)
			for _, code := range cfg.Stocks {
				name := code
				if q := c.GetStock(code); q != nil && q.Name != "" {
					name = q.Name
				}
				_, _ = a.AnalyzeStock(code, name)
			}
			for _, code := range cfg.Futures {
				name := code
				if q := c.GetFutures(code); q != nil && q.Name != "" {
					name = q.Name
				}
				_, _ = a.AnalyzeFutures(code, name)
			}
			_ = a.SaveToFile(stock.DefaultAIStorePath())
		}()
	}

	// Wait first load.
	time.Sleep(500 * time.Millisecond)

	hasGlobal := realtime.HasGlobalFutures(cfg.Futures)
	isTrading := trading.IsStockTradingTime() || trading.IsFuturesTradingTime() || hasGlobal

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	if !isTrading {
		// Off market: wait up to 60s for analysis then print once.
		if a != nil && a.IsEnabled() {
			for i := 0; i < 60; i++ {
				if len(a.GetAllAnalysis()) > 0 {
					break
				}
				time.Sleep(1 * time.Second)
			}
		}
		terminalui.Render(terminalui.Snapshot{
			Now:              time.Now(),
			StockTrading:     trading.IsStockTradingTime(),
			FuturesTrading:   trading.IsFuturesTradingTime(),
			Stocks:           c.GetAllStocks(),
			Futures:          c.GetAllFutures(),
			Analyses:         safeAnalyses(a),
			ShowFullAnalysis: true,
		})
		<-sig
		close(stop)
		return nil
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			close(stop)
			return nil
		case <-ticker.C:
			terminalui.Render(terminalui.Snapshot{
				Now:              time.Now(),
				StockTrading:     trading.IsStockTradingTime(),
				FuturesTrading:   trading.IsFuturesTradingTime(),
				Stocks:           c.GetAllStocks(),
				Futures:          c.GetAllFutures(),
				Analyses:         safeAnalyses(a),
				ShowFullAnalysis: false,
			})
		}
	}
}

func safeAnalyses(a *analyzer.ClaudeAnalyzer) []*analyzer.Analysis {
	if a == nil || !a.IsEnabled() {
		return nil
	}
	return a.GetAllAnalysis()
}

func runAPICLI(serverURL string) error {
	base := strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if base == "" {
		base = "http://localhost:19527"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	// Initial fetch
	stocks, futures, analyses, stockTrading, futuresTrading, err := fetchSnapshot(ctx, client, base)
	if err != nil {
		return err
	}

	hasGlobal := false
	for _, q := range futures {
		if q != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(q.Code)), "hf_") {
			hasGlobal = true
			break
		}
	}

	isTrading := stockTrading || futuresTrading || hasGlobal
	if !isTrading {
		terminalui.Render(terminalui.Snapshot{
			Now:              time.Now(),
			StockTrading:     stockTrading,
			FuturesTrading:   futuresTrading,
			Stocks:           stocks,
			Futures:          futures,
			Analyses:         analyses,
			ShowFullAnalysis: true,
		})
		<-sig
		return nil
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			return nil
		case <-ticker.C:
			stocks, futures, analyses, stockTrading, futuresTrading, err := fetchSnapshot(ctx, client, base)
			if err != nil {
				// Keep last screen; next tick may recover.
				continue
			}
			terminalui.Render(terminalui.Snapshot{
				Now:              time.Now(),
				StockTrading:     stockTrading,
				FuturesTrading:   futuresTrading,
				Stocks:           stocks,
				Futures:          futures,
				Analyses:         analyses,
				ShowFullAnalysis: false,
			})
		}
	}
}

type statusResp struct {
	Code int `json:"code"`
	Data struct {
		StockTrading   bool `json:"stock_trading"`
		FuturesTrading bool `json:"futures_trading"`
		AIEnabled      bool `json:"ai_enabled"`
	} `json:"data"`
}

type stocksResp struct {
	Code int `json:"code"`
	Data []struct {
		Quote model.StockQuote `json:"quote"`
	} `json:"data"`
}

type futuresResp struct {
	Code int `json:"code"`
	Data []struct {
		Quote model.FuturesQuote `json:"quote"`
	} `json:"data"`
}

type analysisResp struct {
	Code  int                  `json:"code"`
	Count int                  `json:"count"`
	Data  []*analyzer.Analysis `json:"data"`
}

func fetchSnapshot(ctx context.Context, client *http.Client, base string) (stocks []*model.StockQuote, futures []*model.FuturesQuote, analyses []*analyzer.Analysis, stockTrading bool, futuresTrading bool, err error) {
	// status
	{
		var sr statusResp
		if err := getJSON(ctx, client, base+"/api/status", &sr); err == nil && sr.Code == 0 {
			stockTrading = sr.Data.StockTrading
			futuresTrading = sr.Data.FuturesTrading
		}
	}

	{
		var r stocksResp
		if err := getJSON(ctx, client, base+"/api/stocks", &r); err == nil && r.Code == 0 {
			stocks = make([]*model.StockQuote, 0, len(r.Data))
			for _, item := range r.Data {
				q := item.Quote
				stocks = append(stocks, &q)
			}
		}
	}
	{
		var r futuresResp
		if err := getJSON(ctx, client, base+"/api/futures", &r); err == nil && r.Code == 0 {
			futures = make([]*model.FuturesQuote, 0, len(r.Data))
			for _, item := range r.Data {
				q := item.Quote
				futures = append(futures, &q)
			}
		}
	}

	// AI analysis is optional; ignore errors (including 503).
	{
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/analysis", nil)
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var ar analysisResp
				if derr := json.NewDecoder(resp.Body).Decode(&ar); derr == nil && ar.Code == 0 {
					analyses = ar.Data
				}
			}
		}
	}

	if stocks == nil && futures == nil {
		return nil, nil, nil, false, false, fmt.Errorf("failed to fetch snapshot from %s", base)
	}
	return stocks, futures, analyses, stockTrading, futuresTrading, nil
}

func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: http %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
