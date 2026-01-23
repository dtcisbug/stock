package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"stock/fetcher"
)

// Analysis 分析结果
type Analysis struct {
	Code      string    `json:"code"`       // 代码
	Name      string    `json:"name"`       // 名称
	Type      string    `json:"type"`       // 类型: stock/futures
	Analysis  string    `json:"analysis"`   // 分析内容
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// ClaudeAnalyzer Claude AI 分析器
type ClaudeAnalyzer struct {
	apiKey     string
	apiURL     string
	model      string
	klineFetch *fetcher.KLineFetcher
	results    sync.Map // map[string]*Analysis
	mu         sync.RWMutex
}

// NewClaudeAnalyzer 创建分析器
func NewClaudeAnalyzer(apiKey, apiBase, model string) *ClaudeAnalyzer {
	if apiBase == "" {
		apiBase = "https://api.anthropic.com"
	}
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	// 构建完整的 API URL
	apiURL := strings.TrimSuffix(apiBase, "/") + "/v1/messages"

	return &ClaudeAnalyzer{
		apiKey:     apiKey,
		apiURL:     apiURL,
		model:      model,
		klineFetch: fetcher.NewKLineFetcher(),
	}
}

// AnalyzeStock 分析股票
func (a *ClaudeAnalyzer) AnalyzeStock(code, name string) (*Analysis, error) {
	// 获取最近3个月日K线（约60个交易日）
	klines, err := a.klineFetch.FetchStockKLine(code, 60)
	if err != nil {
		return nil, fmt.Errorf("获取K线数据失败: %w", err)
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("未获取到K线数据")
	}

	// 构建分析提示
	prompt := a.buildPrompt(name, "股票", klines)

	// 调用 Claude API
	analysis, err := a.callClaude(prompt)
	if err != nil {
		return nil, err
	}

	result := &Analysis{
		Code:      code,
		Name:      name,
		Type:      "stock",
		Analysis:  analysis,
		UpdatedAt: time.Now(),
	}

	// 缓存结果
	a.results.Store(code, result)

	return result, nil
}

// AnalyzeFutures 分析期货
func (a *ClaudeAnalyzer) AnalyzeFutures(code, name string) (*Analysis, error) {
	// 获取最近3个月日K线（约60个交易日）
	klines, err := a.klineFetch.FetchFuturesKLine(code, 60)
	if err != nil {
		return nil, fmt.Errorf("获取K线数据失败: %w", err)
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("未获取到K线数据")
	}

	// 构建分析提示
	prompt := a.buildPrompt(name, "期货", klines)

	// 调用 Claude API
	analysis, err := a.callClaude(prompt)
	if err != nil {
		return nil, err
	}

	result := &Analysis{
		Code:      code,
		Name:      name,
		Type:      "futures",
		Analysis:  analysis,
		UpdatedAt: time.Now(),
	}

	// 缓存结果
	a.results.Store(code, result)

	return result, nil
}

// buildPrompt 构建分析提示
func (a *ClaudeAnalyzer) buildPrompt(name, typ string, klines []fetcher.KLine) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请分析%s「%s」最近%d个交易日（约3个月）的日K线走势:\n\n", typ, name, len(klines)))
	sb.WriteString("日期 | 开盘 | 收盘 | 最高 | 最低 | 成交量\n")
	sb.WriteString("---|---|---|---|---|---\n")

	for _, k := range klines {
		sb.WriteString(fmt.Sprintf("%s | %.2f | %.2f | %.2f | %.2f | %d\n",
			k.Date, k.Open, k.Close, k.High, k.Low, k.Volume))
	}

	sb.WriteString("\n请从以下角度进行分析（总共不超过300字）:\n")
	sb.WriteString("1. 整体趋势: 3个月内的主要趋势方向\n")
	sb.WriteString("2. 关键点位: 重要的支撑位和压力位\n")
	sb.WriteString("3. 成交量特征: 量价配合情况\n")
	sb.WriteString("4. 后市展望: 中短期走势预判及操作建议\n")

	return sb.String()
}

// callClaude 调用 Claude API
func (a *ClaudeAnalyzer) callClaude(prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":      a.model,
		"max_tokens": 500,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", a.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("API返回内容为空")
	}

	return result.Content[0].Text, nil
}

// GetAnalysis 获取缓存的分析结果
func (a *ClaudeAnalyzer) GetAnalysis(code string) *Analysis {
	if v, ok := a.results.Load(code); ok {
		return v.(*Analysis)
	}
	return nil
}

// GetAllAnalysis 获取所有分析结果
func (a *ClaudeAnalyzer) GetAllAnalysis() []*Analysis {
	var results []*Analysis
	a.results.Range(func(key, value interface{}) bool {
		results = append(results, value.(*Analysis))
		return true
	})
	return results
}

// IsEnabled 检查是否启用
func (a *ClaudeAnalyzer) IsEnabled() bool {
	return a.apiKey != ""
}
