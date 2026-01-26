package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"stock/backtest"
	"stock/llm"
)

func runLLMGenerateBacktest(baseURL, model, prompt, promptFile, outPath string) error {
	userPrompt, err := readLLMInput(prompt, promptFile)
	if err != nil {
		return err
	}
	userPrompt = strings.TrimSpace(userPrompt)
	if userPrompt == "" {
		return fmt.Errorf("empty prompt")
	}

	client := llm.NewOllamaClientWithTimeout(baseURL, model, llmTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), llmTimeout+30*time.Second)
	defer cancel()

	schemaHint := `Schema (JSON object only):
{
  "backtest": {
    "days": 5000,
    "start": "2015-01-01",
    "end": "2025-12-31",
    "initial_cash": 1000000,
    "position_pct": 1.0,
    "slippage_bps": 5,
    "commission_bps": 1,
    "stock_lot_size": 100,
    "futures_multiplier": 1,
    "futures_margin_rate": 0.12,
    "instruments": {
      "stocks": ["sh600000"],
      "futures": ["nf_I0", "pp2605"]
    }
  },
  "strategy": {
    "type": "tsai_sen",
    "params": {
      "level_mode": "pivots",
      "box_lookback": 60,
      "pivot_n": 3,
      "touch_tol_pct": 0.003,
      "min_touches": 3,
      "min_range_pct": 0.03,
      "break_pct": 0.005,
      "reclaim_pct": 0.0,
      "flip_max_bars": 20,
      "entry_mode": "reclaim_support",
      "stabilize_bars": 2,
      "stop_buffer_pct": 0.005,
      "target_multiple": 1.0,
      "vol_ma_n": 20,
      "vol_ratio_min": 0,
      "enable_fake_breakout": true,
      "fake_max_bars": 10
    }
  }
}`

	fullPrompt := strings.TrimSpace(`
You will generate a backtest configuration for A-share / China futures daily bars.
Rules:
- Daily bars, close-confirm signal, execute at next-day open.
- Futures use main continuous symbol format like nf_I0, or short like pp2605 (will be normalized).
` + "\n\n" + schemaHint + "\n\nUser requirement:\n" + userPrompt)

	req := llm.GenerateRequest{
		Prompt: fullPrompt,
		System: llm.SystemBacktestConfigJSON(),
		Options: map[string]any{
			"temperature": 0.1,
			"top_p":       0.9,
			"num_predict": 2048,
		},
		Format: "json",
	}

	resp, err := client.Generate(ctx, req)
	if err != nil && strings.Contains(err.Error(), "format") {
		// Fallback for older Ollama
		req.Format = ""
		resp, err = client.Generate(ctx, req)
	}
	if err != nil {
		return err
	}

	rawJSON, err := llm.ExtractFirstJSONValue(resp.Response)
	if err != nil {
		return fmt.Errorf("extract json: %w", err)
	}

	cfg, err := backtest.ParseLLMBacktestConfigJSON(rawJSON)
	if err != nil {
		return err
	}
	yamlBytes, err := cfg.ToYAML()
	if err != nil {
		return err
	}

	// Validate by loading via existing backtest loader.
	tmp, err := os.CreateTemp("", "backtest-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpName)
	if err := os.WriteFile(tmpName, yamlBytes, 0o644); err != nil {
		return err
	}
	if _, err := backtest.LoadRunConfig(tmpName); err != nil {
		return fmt.Errorf("generated yaml failed to load: %w", err)
	}

	return os.WriteFile(outPath, yamlBytes, 0o644)
}

func runLLMAnalyzeReport(baseURL, model, reportPath, btConfigPath, outPath string) error {
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		return err
	}
	var results []backtest.Result
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("parse report json: %w", err)
	}

	sum := llm.SummarizeReport(results)
	sumJSON, err := sum.MarshalIndented()
	if err != nil {
		return err
	}

	btCfgText := ""
	if strings.TrimSpace(btConfigPath) != "" {
		b, err := os.ReadFile(btConfigPath)
		if err != nil {
			return fmt.Errorf("read backtest config: %w", err)
		}
		btCfgText = string(b)
	}

	client := llm.NewOllamaClientWithTimeout(baseURL, model, llmTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), llmTimeout+30*time.Second)
	defer cancel()

	defs := strings.TrimSpace(`
字段定义（请按此定义解释）:
- avg_win_rate_pct: 各品种 win_rate_pct 的简单平均（不按交易次数加权）
- overall_win_rate_pct: 总盈利笔数 / 总交易笔数 * 100（按交易数加权的整体胜率）
`)

	prompt := defs + "\n\n回测摘要(JSON):\n" + string(sumJSON)
	if btCfgText != "" {
		prompt += "\n\n回测配置(backtest.yaml):\n" + btCfgText
	}

	req := llm.GenerateRequest{
		Prompt: prompt,
		System: llm.SystemReportAnalysis(),
		Options: map[string]any{
			"temperature": 0.3,
			"top_p":       0.9,
			"num_predict": 2048,
		},
	}
	resp, err := client.Generate(ctx, req)
	if err != nil {
		return err
	}

	out := strings.TrimSpace(resp.Response)
	if outPath == "" {
		_, err := io.WriteString(os.Stdout, out+"\n")
		return err
	}
	return os.WriteFile(outPath, []byte(out+"\n"), 0o644)
}

func runLLMScanAdvice(baseURL, model, btConfigPath, serviceConfigPath, outPath string, onlySignal bool, scanDays int, scanChart bool, scanChartDir string, scanChartBars int) error {
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

	sum := llm.SummarizeScan(results)
	sumJSON, err := sum.MarshalIndented()
	if err != nil {
		return err
	}

	btCfgBytes, err := os.ReadFile(btConfigPath)
	if err != nil {
		return fmt.Errorf("read backtest config: %w", err)
	}

	client := llm.NewOllamaClientWithTimeout(baseURL, model, llmTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), llmTimeout+30*time.Second)
	defer cancel()

	prompt := ""
	if window != "" {
		prompt += window + "\n\n"
	}
	prompt += "最新日线扫描结果(JSON):\n" + string(sumJSON) + "\n\n回测/扫描配置(backtest.yaml):\n" + string(btCfgBytes)
	req := llm.GenerateRequest{
		Prompt: prompt,
		System: llm.SystemScanAdvice(),
		Options: map[string]any{
			"temperature": 0.2,
			"top_p":       0.9,
			"num_predict": 2048,
		},
	}

	resp, err := client.Generate(ctx, req)
	if err != nil {
		return err
	}

	out := strings.TrimSpace(resp.Response)
	if outPath == "" {
		_, err := io.WriteString(os.Stdout, out+"\n")
		return err
	}
	return os.WriteFile(outPath, []byte(out+"\n"), 0o644)
}

func readLLMInput(prompt, promptFile string) (string, error) {
	if strings.TrimSpace(prompt) != "" {
		return prompt, nil
	}
	if strings.TrimSpace(promptFile) != "" {
		b, err := os.ReadFile(promptFile)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	// stdin
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
