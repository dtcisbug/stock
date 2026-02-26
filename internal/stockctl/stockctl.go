package stockctl

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func Run(args []string) int {
	fs := flag.NewFlagSet("stockctl", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		cliMode    bool
		standalone bool
		serverURL  string

		enableAI   bool
		configPath string

		backtestMode   bool
		backtestConfig string
		backtestOut    string

		scanMode       bool
		scanOut        string
		scanJSON       bool
		scanOnlySignal bool
		scanDays       int
		scanChart      bool
		scanChartDir   string
		scanChartBars  int

		analyzeMode       bool
		analyzeOutDir     string
		analyzeWindowDays int
		analyzeBars       int

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
	)

	fs.BoolVar(&cliMode, "cli", false, "终端实时行情模式（默认通过 -server 调用 stockd；可加 -standalone 直连数据源）")
	fs.BoolVar(&standalone, "standalone", false, "CLI 直连数据源（不依赖 stockd API）")
	fs.StringVar(&serverURL, "server", "http://localhost:19527", "stockd HTTP Base URL（如 http://localhost:19527）")

	fs.BoolVar(&enableAI, "ai", false, "启用AI分析功能（standalone 时生效；API 模式下仅尝试展示已有分析）")
	fs.StringVar(&configPath, "config", "", "配置文件路径(YAML格式)，默认优先使用 ./config.yaml")

	fs.BoolVar(&backtestMode, "backtest", false, "运行日线回测并退出")
	fs.StringVar(&backtestConfig, "bt-config", "backtest.yaml", "回测/扫描配置文件路径(YAML格式)")
	fs.StringVar(&backtestOut, "bt-out", "", "回测输出JSON文件路径(默认stdout)")

	fs.BoolVar(&scanMode, "scan", false, "扫描最新一根日K是否产生策略信号并退出（信号在收盘确认，下一交易日开盘执行）")
	fs.StringVar(&scanOut, "scan-out", "", "扫描输出路径（默认stdout）")
	fs.BoolVar(&scanJSON, "scan-json", false, "扫描输出使用 JSON 格式（默认表格文本）")
	fs.BoolVar(&scanOnlySignal, "scan-only-signal", false, "仅输出有信号的标的（错误信息仍输出）")
	fs.IntVar(&scanDays, "scan-days", 0, "扫描/LLM扫描时覆盖日期窗口：最近 N 天（自然日窗口；结束日期默认今天）")
	fs.BoolVar(&scanChart, "scan-chart", false, "扫描/LLM扫描时输出带画线的K线图(SVG)到目录（用于趋势上下文）")
	fs.StringVar(&scanChartDir, "scan-chart-dir", "runtime/scan_charts", "扫描图输出目录（配合 -scan-chart）")
	fs.IntVar(&scanChartBars, "scan-chart-bars", 220, "每个标的输出最近 N 根K线到图中（配合 -scan-chart）")

	fs.BoolVar(&analyzeMode, "analyze", false, "对 config.yaml 中标的做一年量价分析（蔡森破底翻）并输出 JSON/CSV + K线图")
	fs.StringVar(&analyzeOutDir, "analyze-out-dir", "runtime/analysis", "分析输出目录（默认 runtime/analysis）")
	fs.IntVar(&analyzeWindowDays, "analyze-window-days", 365, "分析窗口：最近 N 个自然日（与 -analyze-bars 互斥）")
	fs.IntVar(&analyzeBars, "analyze-bars", 0, "分析窗口：最近 N 根日K（优先级高于 -analyze-window-days）")

	fs.BoolVar(&llmGenBT, "llm-gen-bt", false, "使用本地大模型(Ollama)生成 backtest.yaml 并退出（从 stdin/--llm-prompt/--llm-prompt-file 读取策略描述）")
	fs.StringVar(&llmAnalyzePath, "llm-analyze", "", "使用本地大模型(Ollama)复盘回测报告JSON并退出（传入 report.json 路径）")
	fs.StringVar(&llmPrompt, "llm-prompt", "", "LLM 输入（用于生成 backtest.yaml，优先级高于 --llm-prompt-file/stdin）")
	fs.StringVar(&llmPromptFile, "llm-prompt-file", "", "LLM 输入文件路径（用于生成 backtest.yaml）")
	fs.StringVar(&llmURL, "llm-url", "http://localhost:11434", "Ollama Base URL（默认 http://localhost:11434）")
	fs.StringVar(&llmModel, "llm-model", "qwen2.5-coder:14b", "Ollama 模型名称（默认 qwen2.5-coder:14b）")
	fs.StringVar(&llmOut, "llm-out", "", "LLM 输出路径（生成 backtest.yaml 默认 backtest.yaml；复盘默认 stdout）")
	fs.StringVar(&llmBTConfig, "llm-bt-config", "", "回测配置文件路径（复盘/LLM扫描时可选，帮助模型理解参数）")
	fs.DurationVar(&llmTimeout, "llm-timeout", 10*time.Minute, "Ollama 请求超时时间（如 10m/180s；首次加载模型可设大一点）")
	fs.BoolVar(&llmScan, "llm-scan", false, "使用本地大模型(Ollama)将最新信号扫描结果输出为人类可读的执行建议(Markdown)")
	fs.BoolVar(&llmScanOnly, "llm-scan-only-signal", false, "LLM 扫描建议仅包含有信号的标的（错误仍包含）")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Mutually exclusive modes.
	llmAny := llmGenBT || llmAnalyzePath != "" || llmScan
	if llmAny && (scanMode || backtestMode || analyzeMode) {
		log.Printf("[ERROR] LLM 模式不能与 scan/backtest/analyze 同时使用\n")
		return 2
	}
	if analyzeMode && (scanMode || backtestMode) {
		log.Printf("[ERROR] analyze 不能与 scan/backtest 同时使用\n")
		return 2
	}

	if llmAny {
		if llmScan && (llmGenBT || llmAnalyzePath != "") {
			log.Printf("[ERROR] -llm-scan 不能与 -llm-gen-bt/-llm-analyze 同时使用\n")
			return 2
		}
		if llmGenBT && llmAnalyzePath != "" {
			log.Printf("[ERROR] 不能同时使用 -llm-gen-bt 与 -llm-analyze\n")
			return 2
		}

		if llmGenBT {
			out := llmOut
			if out == "" {
				out = "backtest.yaml"
			}
			if err := runLLMGenerateBacktest(llmURL, llmModel, llmPrompt, llmPromptFile, out, llmTimeout); err != nil {
				log.Printf("[ERROR] LLM 生成回测配置失败: %v\n", err)
				return 1
			}
			return 0
		}

		if llmAnalyzePath != "" {
			if err := runLLMAnalyzeReport(llmURL, llmModel, llmAnalyzePath, llmBTConfig, llmOut, llmTimeout); err != nil {
				log.Printf("[ERROR] LLM 复盘失败: %v\n", err)
				return 1
			}
			return 0
		}

		if llmScan {
			btCfg := backtestConfig
			if llmBTConfig != "" {
				btCfg = llmBTConfig
			}
			if err := runLLMScanAdvice(llmURL, llmModel, btCfg, configPath, llmOut, llmScanOnly, scanDays, scanChart, scanChartDir, scanChartBars, llmTimeout); err != nil {
				log.Printf("[ERROR] LLM 扫描建议生成失败: %v\n", err)
				return 1
			}
			return 0
		}
	}

	if analyzeMode {
		if err := runAnalyze(configPath, backtestConfig, analyzeOutDir, analyzeWindowDays, analyzeBars); err != nil {
			log.Printf("[ERROR] 分析失败: %v\n", err)
			return 1
		}
		return 0
	}

	if scanMode {
		if err := runScan(backtestConfig, configPath, scanOut, scanJSON, scanOnlySignal, scanDays, scanChart, scanChartDir, scanChartBars); err != nil {
			log.Printf("[ERROR] 扫描失败: %v\n", err)
			return 1
		}
		return 0
	}

	if backtestMode {
		if err := runBacktest(backtestConfig, backtestOut); err != nil {
			log.Printf("[ERROR] 回测失败: %v\n", err)
			return 1
		}
		return 0
	}

	if cliMode {
		if err := runCLI(serverURL, configPath, enableAI, standalone); err != nil {
			log.Printf("[ERROR] CLI 运行失败: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  stockctl -cli [-server http://localhost:19527] [-standalone] [-config config.yaml]")
	fmt.Fprintln(os.Stderr, "  stockctl -analyze -config config.yaml -bt-config backtest.yaml [-analyze-window-days 365 | -analyze-bars 252]")
	fmt.Fprintln(os.Stderr, "  stockctl -scan -bt-config backtest.yaml [-scan-days 365] [-scan-chart]")
	fmt.Fprintln(os.Stderr, "  stockctl -backtest -bt-config backtest.yaml [-bt-out runtime/report.json]")
	fmt.Fprintln(os.Stderr, "  stockctl -llm-gen-bt / -llm-analyze / -llm-scan ...")
	return 2
}
