package stockd

import (
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"stock"
	"stock/analyzer"
	"stock/api"
	"stock/cache"
	"stock/config"
	"stock/fetcher"
	"stock/internal/realtime"
	"stock/trading"
)

func Run(args []string) int {
	flags := flag.NewFlagSet("stockd", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	var (
		configPath string
		enableAI   bool
	)

	flags.StringVar(&configPath, "config", "", "配置文件路径(YAML格式)，默认优先使用 ./config.yaml")
	flags.BoolVar(&enableAI, "ai", false, "启用AI分析功能（需要配置 api.token 或环境变量）")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	if configPath == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		}
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	cfg := config.GetConfig(configPath)
	dataCache := cache.Global

	stockFetcher := fetcher.NewStockFetcher()
	futuresFetcher := fetcher.NewFuturesFetcher()

	aiEnabled := enableAI || cfg.EnableAI
	var globalAnalyzer *analyzer.ClaudeAnalyzer
	aiStorePath := stock.DefaultAIStorePath()
	if aiEnabled && cfg.ClaudeAPIKey != "" {
		globalAnalyzer = analyzer.NewClaudeAnalyzer(cfg.ClaudeAPIKey, cfg.ClaudeAPIBase, cfg.ClaudeModel)
		if err := globalAnalyzer.LoadFromFile(aiStorePath); err != nil {
			log.Printf("[WARN] load persisted AI analysis failed: %v\n", err)
		}
	}

	stop := make(chan struct{})
	go realtime.RunDataSync(cfg, dataCache, stockFetcher, futuresFetcher, stop, realtime.SyncOptions{Logger: log.Default(), Quiet: false})

	if globalAnalyzer != nil && globalAnalyzer.IsEnabled() {
		go runAIAnalysisLoop(cfg, dataCache, globalAnalyzer, stop, aiStorePath)
	}

	log.Println("=== A股/期货实时行情服务 (stockd) ===")
	if globalAnalyzer != nil && globalAnalyzer.IsEnabled() {
		log.Println("[AI] Claude AI 分析已启用")
	} else if aiEnabled && cfg.ClaudeAPIKey == "" {
		log.Println("[AI] 未设置 API Token，AI分析功能未能启用")
	} else {
		log.Println("[AI] AI分析功能已关闭（使用 -ai 参数或配置文件启用）")
	}

	staticFS, err := stock.GetStaticFS()
	if err != nil {
		log.Printf("[WARN] 无法加载前端资源: %v (仅API模式)\n", err)
	}

	var sfs fs.FS
	if err == nil {
		sfs = staticFS
	}

	server := api.NewServer(dataCache, cfg.Port, globalAnalyzer, sfs)
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("[ERROR] HTTP服务启动失败: %v\n", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("正在关闭服务...")
	close(stop)
	_ = server.Shutdown()
	log.Println("服务已关闭")
	return 0
}

func runAIAnalysisLoop(cfg *config.Config, c *cache.Cache, a *analyzer.ClaudeAnalyzer, stop <-chan struct{}, storePath string) {
	// Wait initial data load.
	time.Sleep(3 * time.Second)

	runAnalysisOnce(cfg, c, a)
	if err := a.SaveToFile(storePath); err != nil {
		log.Printf("[WARN] persist AI analysis failed: %v\n", err)
	}

	ticker := time.NewTicker(cfg.AnalysisInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// Only run during China trading time (matches README intent).
			if !trading.IsTradingTime() {
				continue
			}
			runAnalysisOnce(cfg, c, a)
			if err := a.SaveToFile(storePath); err != nil {
				log.Printf("[WARN] persist AI analysis failed: %v\n", err)
			}
		}
	}
}

func runAnalysisOnce(cfg *config.Config, c *cache.Cache, a *analyzer.ClaudeAnalyzer) {
	type job struct {
		code string
		name string
		typ  string
	}
	jobs := make([]job, 0, len(cfg.Stocks)+len(cfg.Futures))

	for _, code := range cfg.Stocks {
		name := code
		if q := c.GetStock(code); q != nil && q.Name != "" {
			name = q.Name
		}
		jobs = append(jobs, job{code: code, name: name, typ: "stock"})
	}
	for _, code := range cfg.Futures {
		name := code
		if q := c.GetFutures(code); q != nil && q.Name != "" {
			name = q.Name
		}
		jobs = append(jobs, job{code: code, name: name, typ: "futures"})
	}

	if len(jobs) == 0 {
		return
	}

	log.Printf("[AI] analysis start (%d instruments)...\n", len(jobs))

	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	for _, j := range jobs {
		j := j
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			var err error
			if j.typ == "stock" {
				_, err = a.AnalyzeStock(j.code, j.name)
			} else {
				_, err = a.AnalyzeFutures(j.code, j.name)
			}
			if err != nil {
				log.Printf("[AI] analyze failed %s %s: %v\n", j.typ, j.code, err)
			}
		}()
	}

	wg.Wait()
	log.Printf("[AI] analysis done (%d/%d)\n", len(jobs), len(jobs))
}
