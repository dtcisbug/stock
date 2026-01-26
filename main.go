package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"stock/analyzer"
	"stock/api"
	"stock/cache"
	"stock/config"
	"stock/fetcher"
	"stock/trading"
)

var (
	cliMode        bool
	enableAI       bool
	configPath     string
	backtestMode   bool
	backtestConfig string
	backtestOut    string
	llmGenBT       bool
	llmAnalyzePath string
	llmPrompt      string
	llmPromptFile  string
	llmURL         string
	llmModel       string
	llmOut         string
	llmBTConfig    string
	llmTimeout     time.Duration
	llmScan        bool
	llmScanOnly    bool
	scanMode       bool
	scanOut        string
	scanJSON       bool
	scanOnlySignal bool
	scanDays       int
	scanChart      bool
	scanChartDir   string
	scanChartBars  int
	globalAnalyzer *analyzer.ClaudeAnalyzer
)

func main() {
	flag.BoolVar(&cliMode, "cli", false, "终端实时行情模式")
	flag.BoolVar(&enableAI, "ai", false, "启用AI分析功能")
	flag.StringVar(&configPath, "config", "", "配置文件路径(YAML格式)")
	flag.BoolVar(&backtestMode, "backtest", false, "运行日线回测并退出")
	flag.StringVar(&backtestConfig, "bt-config", "backtest.yaml", "回测配置文件路径(YAML格式)")
	flag.StringVar(&backtestOut, "bt-out", "", "回测输出JSON文件路径(默认stdout)")
	flag.BoolVar(&llmGenBT, "llm-gen-bt", false, "使用本地大模型(Ollama)生成 backtest.yaml 并退出（从 stdin/--llm-prompt/--llm-prompt-file 读取策略描述）")
	flag.StringVar(&llmAnalyzePath, "llm-analyze", "", "使用本地大模型(Ollama)复盘回测报告JSON并退出（传入 report.json 路径）")
	flag.StringVar(&llmPrompt, "llm-prompt", "", "LLM 输入（用于生成 backtest.yaml，优先级高于 --llm-prompt-file/stdin）")
	flag.StringVar(&llmPromptFile, "llm-prompt-file", "", "LLM 输入文件路径（用于生成 backtest.yaml）")
	flag.StringVar(&llmURL, "llm-url", "http://localhost:11434", "Ollama Base URL（默认 http://localhost:11434）")
	flag.StringVar(&llmModel, "llm-model", "qwen2.5-coder:14b", "Ollama 模型名称（默认 qwen2.5-coder:14b）")
	flag.StringVar(&llmOut, "llm-out", "", "LLM 输出路径（生成 backtest.yaml 默认 backtest.yaml；复盘默认 stdout）")
	flag.StringVar(&llmBTConfig, "llm-bt-config", "", "回测配置文件路径（复盘时可选，帮助模型理解参数）")
	flag.DurationVar(&llmTimeout, "llm-timeout", 10*time.Minute, "Ollama 请求超时时间（如 10m/180s；首次加载模型可设大一点）")
	flag.BoolVar(&llmScan, "llm-scan", false, "使用本地大模型(Ollama)将最新信号扫描结果输出为人类可读的执行建议(Markdown)")
	flag.BoolVar(&llmScanOnly, "llm-scan-only-signal", false, "LLM 扫描建议仅包含有信号的标的（错误仍包含）")
	flag.BoolVar(&scanMode, "scan", false, "扫描最新一根日K是否产生策略信号并退出（信号在收盘确认，下一交易日开盘执行）")
	flag.StringVar(&scanOut, "scan-out", "", "扫描输出路径（默认stdout）")
	flag.BoolVar(&scanJSON, "scan-json", false, "扫描输出使用 JSON 格式（默认表格文本）")
	flag.BoolVar(&scanOnlySignal, "scan-only-signal", false, "仅输出有信号的标的（错误信息仍输出）")
	flag.IntVar(&scanDays, "scan-days", 0, "扫描/LLM扫描时覆盖日期窗口：最近 N 天（如 365 表示最近一年；结束日期默认今天）")
	flag.BoolVar(&scanChart, "scan-chart", false, "扫描/LLM扫描时输出带画线的K线图(SVG)到目录（用于趋势上下文）")
	flag.StringVar(&scanChartDir, "scan-chart-dir", "scan_charts", "扫描图输出目录（配合 -scan-chart）")
	flag.IntVar(&scanChartBars, "scan-chart-bars", 220, "每个标的输出最近 N 根K线到图中（配合 -scan-chart）")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	if llmGenBT || llmAnalyzePath != "" || llmScan {
		if llmScan && (llmGenBT || llmAnalyzePath != "") {
			log.Printf("[ERROR] -llm-scan 不能与 -llm-gen-bt/-llm-analyze 同时使用\n")
			os.Exit(2)
		}
		if llmGenBT && llmAnalyzePath != "" {
			log.Printf("[ERROR] 不能同时使用 -llm-gen-bt 与 -llm-analyze\n")
			os.Exit(2)
		}
		if llmGenBT {
			out := llmOut
			if out == "" {
				out = "backtest.yaml"
			}
			if err := runLLMGenerateBacktest(llmURL, llmModel, llmPrompt, llmPromptFile, out); err != nil {
				log.Printf("[ERROR] LLM 生成回测配置失败: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if llmAnalyzePath != "" {
			if err := runLLMAnalyzeReport(llmURL, llmModel, llmAnalyzePath, llmBTConfig, llmOut); err != nil {
				log.Printf("[ERROR] LLM 复盘失败: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if llmScan {
			btCfg := backtestConfig
			if llmBTConfig != "" {
				btCfg = llmBTConfig
			}
			if err := runLLMScanAdvice(llmURL, llmModel, btCfg, configPath, llmOut, llmScanOnly, scanDays, scanChart, scanChartDir, scanChartBars); err != nil {
				log.Printf("[ERROR] LLM 扫描建议生成失败: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	if scanMode {
		if err := runScan(backtestConfig, configPath, scanOut, scanJSON, scanOnlySignal, scanDays, scanChart, scanChartDir, scanChartBars); err != nil {
			log.Printf("[ERROR] 扫描失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if backtestMode {
		if err := runBacktest(backtestConfig, backtestOut); err != nil {
			log.Printf("[ERROR] 回测失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 加载配置
	cfg := config.GetConfig(configPath)

	// 创建缓存
	dataCache := cache.Global

	// 创建数据拉取器
	stockFetcher := fetcher.NewStockFetcher()
	futuresFetcher := fetcher.NewFuturesFetcher()

	// 创建AI分析器（仅在启用AI功能且有API密钥时）
	// AI启用条件: 命令行 -ai 参数 OR 配置文件 enable_ai 设置
	aiEnabled := enableAI || cfg.EnableAI
	if aiEnabled && cfg.ClaudeAPIKey != "" {
		globalAnalyzer = analyzer.NewClaudeAnalyzer(cfg.ClaudeAPIKey, cfg.ClaudeAPIBase, cfg.ClaudeModel)
	}

	// 启动数据同步
	stopChan := make(chan struct{})
	go runDataSync(cfg, dataCache, stockFetcher, futuresFetcher, stopChan)

	// 启动AI分析任务
	if globalAnalyzer != nil && globalAnalyzer.IsEnabled() {
		go runAIAnalysis(cfg, dataCache, stopChan)
	}

	if cliMode {
		// CLI 模式：终端实时显示
		go runTerminalDisplay(dataCache, cfg, stopChan)
	} else {
		// 服务模式：启动HTTP服务
		log.Println("=== A股/期货实时行情服务 ===")
		if globalAnalyzer != nil && globalAnalyzer.IsEnabled() {
			log.Println("[AI] Claude AI 分析已启用")
		} else if aiEnabled && cfg.ClaudeAPIKey == "" {
			log.Println("[AI] 未设置 API Token，AI分析功能未能启用")
		} else {
			log.Println("[AI] AI分析功能已关闭（使用 -ai 参数或配置文件启用）")
		}

		// 获取嵌入的静态文件系统
		staticFS, err := GetStaticFS()
		if err != nil {
			log.Printf("[WARN] 无法加载前端资源: %v (仅API模式)\n", err)
		}

		server := api.NewServer(dataCache, cfg.Port, globalAnalyzer, staticFS)
		go func() {
			if err := server.Start(); err != nil {
				log.Printf("[ERROR] HTTP服务启动失败: %v\n", err)
			}
		}()
	}

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	if !cliMode {
		log.Println("\n正在关闭服务...")
	}
	close(stopChan)
	if !cliMode {
		log.Println("服务已关闭")
	}
}

// runAIAnalysis 运行AI分析任务（只执行一次）
func runAIAnalysis(cfg *config.Config, c *cache.Cache, stop chan struct{}) {
	// 等待数据加载后执行分析
	time.Sleep(3 * time.Second)
	if !cliMode {
		log.Println("[AI] 开始AI分析（并发模式）...")
	}
	runAnalysisConcurrent(cfg, c)
	if !cliMode {
		log.Println("[AI] 所有分析完成")
	}
}

// runAnalysisConcurrent 并发执行分析
func runAnalysisConcurrent(cfg *config.Config, c *cache.Cache) {
	var wg sync.WaitGroup
	var completed int32
	total := int32(len(cfg.Stocks) + len(cfg.Futures))

	// 并发分析股票
	for _, code := range cfg.Stocks {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			quote := c.GetStock(code)
			name := code
			if quote != nil {
				name = quote.Name
			}

			_, err := globalAnalyzer.AnalyzeStock(code, name)
			done := atomic.AddInt32(&completed, 1)

			if cliMode {
				fmt.Printf("\r[AI] 分析进度: %d/%d (%s)          ", done, total, name)
			} else {
				if err != nil {
					log.Printf("[AI] [%d/%d] 股票分析失败 %s: %v\n", done, total, code, err)
				} else {
					log.Printf("[AI] [%d/%d] 股票分析完成: %s\n", done, total, name)
				}
			}
		}(code)
	}

	// 并发分析期货
	for _, code := range cfg.Futures {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			quote := c.GetFutures(code)
			name := code
			if quote != nil {
				name = quote.Name
			}

			_, err := globalAnalyzer.AnalyzeFutures(code, name)
			done := atomic.AddInt32(&completed, 1)

			if cliMode {
				fmt.Printf("\r[AI] 分析进度: %d/%d (%s)          ", done, total, name)
			} else {
				if err != nil {
					log.Printf("[AI] [%d/%d] 期货分析失败 %s: %v\n", done, total, code, err)
				} else {
					log.Printf("[AI] [%d/%d] 期货分析完成: %s\n", done, total, name)
				}
			}
		}(code)
	}

	wg.Wait()

	if cliMode {
		fmt.Printf("\r[AI] 分析完成: %d/%d                    \n", total, total)
	}
}

// runTerminalDisplay 终端实时显示行情
func runTerminalDisplay(c *cache.Cache, cfg *config.Config, stop chan struct{}) {
	// 等待首次数据加载
	time.Sleep(500 * time.Millisecond)

	// 先检查是否休市
	isTrading := trading.IsStockTradingTime() || trading.IsFuturesTradingTime()

	if !isTrading {
		// 休市：等待AI分析完成后显示一次
		// 等待AI分析完成（最多等60秒）
		for i := 0; i < 60; i++ {
			if globalAnalyzer != nil && len(globalAnalyzer.GetAllAnalysis()) > 0 {
				break
			}
			time.Sleep(1 * time.Second)
		}
		printTerminalQuotes(c, cfg, true)
		// 休市时不再刷新，等待用户退出
		<-stop
		return
	}

	// 交易时间：持续刷新
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			printTerminalQuotes(c, cfg, false)
		}
	}
}

// printTerminalQuotes 打印终端行情
func printTerminalQuotes(c *cache.Cache, cfg *config.Config, showFullAnalysis bool) {
	// 清屏
	fmt.Print("\033[2J\033[H")

	// 标题
	now := time.Now().Format("2006-01-02 15:04:05")
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║                    A股/期货实时行情  %s                   ║\n", now)
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	// 交易状态
	stockTrading := trading.IsStockTradingTime()
	futuresTrading := trading.IsFuturesTradingTime()
	stockStatus := "休市"
	futuresStatus := "休市"
	if stockTrading {
		stockStatus = "\033[32m交易中\033[0m"
	}
	if futuresTrading {
		futuresStatus = "\033[32m交易中\033[0m"
	}
	fmt.Printf("║  股票: %-12s  期货: %-12s                                  ║\n", stockStatus, futuresStatus)
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	// 股票行情
	fmt.Println("║  【股票】                                                                ║")
	fmt.Println("║  代码     名称         最新价    涨跌幅    涨跌额      成交量           ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────────────────╢")

	stocks := c.GetAllStocks()
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].Code < stocks[j].Code
	})

	for _, q := range stocks {
		change := q.Change()
		changePercent := q.ChangePercent()
		color := getColor(change)
		name := truncateName(q.Name, 8)
		vol := formatVolume(q.Volume)
		code := strings.TrimPrefix(q.Code, "sz")
		code = strings.TrimPrefix(code, "sh")
		fmt.Printf("║  %-8s %-12s %s%8.2f  %+7.2f%%  %+8.2f\033[0m  %12s  ║\n",
			code, name, color, q.Price, changePercent, change, vol)
	}

	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	// 期货行情
	fmt.Println("║  【期货】                                                                ║")
	fmt.Println("║  代码     名称         最新价    涨跌幅    涨跌额      持仓量           ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────────────────╢")

	futures := c.GetAllFutures()
	sort.Slice(futures, func(i, j int) bool {
		return futures[i].Code < futures[j].Code
	})

	for _, q := range futures {
		change := q.Change()
		changePercent := q.ChangePercent()
		color := getColor(change)
		name := truncateName(q.Name, 8)
		oi := formatVolume(q.OpenInterest)
		code := strings.TrimPrefix(q.Code, "nf_")
		fmt.Printf("║  %-8s %-12s %s%8.2f  %+7.2f%%  %+8.2f\033[0m  %12s  ║\n",
			code, name, color, q.Price, changePercent, change, oi)
	}

	// AI 分析结果
	if globalAnalyzer != nil && globalAnalyzer.IsEnabled() {
		analyses := globalAnalyzer.GetAllAnalysis()
		if len(analyses) > 0 {
			fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")
			fmt.Println("║  【AI分析】                                                              ║")
			fmt.Println("╟──────────────────────────────────────────────────────────────────────────╢")

			if showFullAnalysis {
				// 休市时显示完整分析
				for _, a := range analyses {
					typeName := "股票"
					if a.Type == "futures" {
						typeName = "期货"
					}
					fmt.Printf("\n  \033[33m[%s] %s\033[0m\n", typeName, a.Name)
					fmt.Println("  " + strings.Repeat("-", 70))
					// 按行打印分析内容
					lines := strings.Split(a.Analysis, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line == "" {
							continue
						}
						// 处理长行换行
						runes := []rune(line)
						for len(runes) > 70 {
							fmt.Printf("  %s\n", string(runes[:70]))
							runes = runes[70:]
						}
						if len(runes) > 0 {
							fmt.Printf("  %s\n", string(runes))
						}
					}
					fmt.Println()
				}
			} else {
				// 交易时间显示摘要
				for _, a := range analyses {
					summary := getAnalysisSummary(a.Analysis, 50)
					typeName := "股票"
					if a.Type == "futures" {
						typeName = "期货"
					}
					fmt.Printf("║  [%s] %-8s: %-47s ║\n", typeName, truncateName(a.Name, 6), summary)
				}
			}
		}
	}

	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	if showFullAnalysis {
		fmt.Println("  休市中 | 按 Ctrl+C 退出")
	} else {
		fmt.Println("  交易中 | 按 Ctrl+C 退出 | 5秒刷新")
	}
	if globalAnalyzer == nil || !globalAnalyzer.IsEnabled() {
		fmt.Println("  提示：使用 ./run-cli-ai.sh 启用AI分析功能")
	}
}

// getColor 获取颜色代码
func getColor(change float64) string {
	if change > 0 {
		return "\033[31m" // 红色-涨
	} else if change < 0 {
		return "\033[32m" // 绿色-跌
	}
	return "\033[37m" // 白色-平
}

// truncateName 截断名称
func truncateName(name string, maxLen int) string {
	runes := []rune(name)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return name
}

// formatVolume 格式化成交量
func formatVolume(vol int64) string {
	if vol >= 100000000 {
		return fmt.Sprintf("%.2f亿", float64(vol)/100000000)
	}
	if vol >= 10000 {
		return fmt.Sprintf("%.2f万", float64(vol)/10000)
	}
	return fmt.Sprintf("%d", vol)
}

// getAnalysisSummary 获取分析摘要
func getAnalysisSummary(analysis string, maxLen int) string {
	// 按行分割，找到第一行有意义的内容
	lines := strings.Split(analysis, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和标题行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 去掉markdown格式
		line = strings.TrimPrefix(line, "**")
		line = strings.TrimSuffix(line, "**")
		line = strings.TrimPrefix(line, "- ")

		runes := []rune(line)
		if len(runes) > maxLen {
			return string(runes[:maxLen]) + "..."
		}
		return line
	}
	return "分析中..."
}

// runDataSync 运行数据同步任务
func runDataSync(cfg *config.Config, c *cache.Cache, sf *fetcher.StockFetcher, ff *fetcher.FuturesFetcher, stop chan struct{}) {
	// 首次立即拉取一次数据
	if !cliMode {
		log.Println("[同步] 首次数据拉取...")
	}
	fetchData(cfg, c, sf, ff)

	// 定时器
	refreshTicker := time.NewTicker(cfg.RefreshInterval)
	checkTicker := time.NewTicker(cfg.CheckInterval)
	defer refreshTicker.Stop()
	defer checkTicker.Stop()

	for {
		select {
		case <-stop:
			if !cliMode {
				log.Println("[同步] 停止数据同步")
			}
			return

		case <-refreshTicker.C:
			// 交易时间内刷新数据
			if trading.IsTradingTime() {
				fetchData(cfg, c, sf, ff)
			}

		case <-checkTicker.C:
			// 非交易时间，检查并打印状态
			if !cliMode {
				isStock := trading.IsStockTradingTime()
				isFutures := trading.IsFuturesTradingTime()
				if !isStock && !isFutures {
					log.Printf("[同步] 当前非交易时间 (股票交易: %v, 期货交易: %v)\n", isStock, isFutures)
				}
			}
		}
	}
}

// fetchData 拉取数据
func fetchData(cfg *config.Config, c *cache.Cache, sf *fetcher.StockFetcher, ff *fetcher.FuturesFetcher) {
	// 拉取股票数据
	if len(cfg.Stocks) > 0 {
		quotes, err := sf.Fetch(cfg.Stocks)
		if err != nil {
			if !cliMode {
				log.Printf("[同步] 拉取股票数据失败: %v\n", err)
			}
		} else {
			c.SetStocks(quotes)
			if !cliMode {
				log.Printf("[同步] 股票数据已更新: %d 条\n", len(quotes))
			}
		}
	}

	// 拉取期货数据
	if len(cfg.Futures) > 0 {
		quotes, err := ff.Fetch(cfg.Futures)
		if err != nil {
			if !cliMode {
				log.Printf("[同步] 拉取期货数据失败: %v\n", err)
			}
		} else {
			c.SetFuturesList(quotes)
			if !cliMode {
				log.Printf("[同步] 期货数据已更新: %d 条\n", len(quotes))
			}
		}
	}
}
