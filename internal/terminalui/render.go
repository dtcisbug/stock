package terminalui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"stock/analyzer"
	"stock/model"
)

type Snapshot struct {
	Now            time.Time
	StockTrading   bool
	FuturesTrading bool
	Stocks         []*model.StockQuote
	Futures        []*model.FuturesQuote
	Analyses       []*analyzer.Analysis

	// If true, print full analysis blocks; otherwise print one-line summaries.
	ShowFullAnalysis bool
}

func Render(s Snapshot) {
	// Clear screen
	fmt.Print("\033[2J\033[H")

	now := s.Now
	if now.IsZero() {
		now = time.Now()
	}
	nowStr := now.Format("2006-01-02 15:04:05")

	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║                    A股/期货实时行情  %s                   ║\n", nowStr)
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	stockStatus := "休市"
	futuresStatus := "休市"
	if s.StockTrading {
		stockStatus = "\033[32m交易中\033[0m"
	}
	if s.FuturesTrading {
		futuresStatus = "\033[32m交易中\033[0m"
	}
	fmt.Printf("║  股票: %-12s  期货: %-12s                                  ║\n", stockStatus, futuresStatus)
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	// Stocks
	fmt.Println("║  【股票】                                                                ║")
	fmt.Println("║  代码     名称         最新价    涨跌幅    涨跌额      成交量           ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────────────────╢")

	stocks := append([]*model.StockQuote(nil), s.Stocks...)
	sort.Slice(stocks, func(i, j int) bool { return stocks[i].Code < stocks[j].Code })
	for _, q := range stocks {
		if q == nil {
			continue
		}
		change := q.Change()
		changePercent := q.ChangePercent()
		color := colorByChange(change)
		name := truncateName(q.Name, 8)
		vol := formatVolume(q.Volume)
		code := strings.TrimPrefix(q.Code, "sz")
		code = strings.TrimPrefix(code, "sh")
		fmt.Printf("║  %-8s %-12s %s%8.2f  %+7.2f%%  %+8.2f\033[0m  %12s  ║\n",
			code, name, color, q.Price, changePercent, change, vol)
	}

	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")

	// Futures
	fmt.Println("║  【期货】                                                                ║")
	fmt.Println("║  代码     名称         最新价    涨跌幅    涨跌额      持仓量           ║")
	fmt.Println("╟──────────────────────────────────────────────────────────────────────────╢")

	futures := append([]*model.FuturesQuote(nil), s.Futures...)
	sort.Slice(futures, func(i, j int) bool { return futures[i].Code < futures[j].Code })
	for _, q := range futures {
		if q == nil {
			continue
		}
		change := q.Change()
		changePercent := q.ChangePercent()
		color := colorByChange(change)
		name := truncateName(q.Name, 8)
		oi := formatVolume(q.OpenInterest)
		code := strings.TrimPrefix(q.Code, "nf_")
		fmt.Printf("║  %-8s %-12s %s%8.2f  %+7.2f%%  %+8.2f\033[0m  %12s  ║\n",
			code, name, color, q.Price, changePercent, change, oi)
	}

	// AI analysis
	if len(s.Analyses) > 0 {
		fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")
		fmt.Println("║  【AI分析】                                                              ║")
		fmt.Println("╟──────────────────────────────────────────────────────────────────────────╢")

		if s.ShowFullAnalysis {
			for _, a := range s.Analyses {
				if a == nil {
					continue
				}
				typeName := "股票"
				if a.Type == "futures" {
					typeName = "期货"
				}
				fmt.Printf("\n  \033[33m[%s] %s\033[0m\n", typeName, a.Name)
				fmt.Println("  " + strings.Repeat("-", 70))
				lines := strings.Split(a.Analysis, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
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
			for _, a := range s.Analyses {
				if a == nil {
					continue
				}
				summary := analysisSummary(a.Analysis, 50)
				typeName := "股票"
				if a.Type == "futures" {
					typeName = "期货"
				}
				fmt.Printf("║  [%s] %-8s: %-47s ║\n", typeName, truncateName(a.Name, 6), summary)
			}
		}
	}

	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	if s.StockTrading || s.FuturesTrading {
		fmt.Println("  交易中 | 按 Ctrl+C 退出 | 5秒刷新")
	} else {
		fmt.Println("  休市中 | 按 Ctrl+C 退出")
	}
}

func colorByChange(change float64) string {
	if change > 0 {
		return "\033[31m"
	}
	if change < 0 {
		return "\033[32m"
	}
	return "\033[37m"
}

func truncateName(name string, maxLen int) string {
	runes := []rune(name)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return name
}

func formatVolume(vol int64) string {
	if vol >= 100000000 {
		return fmt.Sprintf("%.2f亿", float64(vol)/100000000)
	}
	if vol >= 10000 {
		return fmt.Sprintf("%.2f万", float64(vol)/10000)
	}
	return fmt.Sprintf("%d", vol)
}

func analysisSummary(analysis string, maxLen int) string {
	lines := strings.Split(analysis, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
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
