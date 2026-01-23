package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// KLine K线数据
type KLine struct {
	Date   string  `json:"date"`   // 日期
	Open   float64 `json:"open"`   // 开盘价
	Close  float64 `json:"close"`  // 收盘价
	High   float64 `json:"high"`   // 最高价
	Low    float64 `json:"low"`    // 最低价
	Volume int64   `json:"volume"` // 成交量
}

// KLineFetcher K线数据拉取器
type KLineFetcher struct {
	client *http.Client
}

// NewKLineFetcher 创建K线数据拉取器
func NewKLineFetcher() *KLineFetcher {
	return &KLineFetcher{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// FetchStockKLine 获取股票日K线数据
// code: 股票代码（如 sh600000, sz000001）
// days: 获取天数
func (f *KLineFetcher) FetchStockKLine(code string, days int) ([]KLine, error) {
	// 使用东方财富接口获取日K数据
	// 转换代码格式: sh600000 -> 1.600000, sz000001 -> 0.000001
	var secid string
	if len(code) > 2 {
		market := code[:2]
		num := code[2:]
		if market == "sh" {
			secid = "1." + num
		} else if market == "sz" {
			secid = "0." + num
		} else {
			return nil, fmt.Errorf("未知的股票代码格式: %s", code)
		}
	} else {
		return nil, fmt.Errorf("股票代码格式错误: %s", code)
	}

	url := fmt.Sprintf(
		"https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57&klt=101&fqt=1&end=20500101&lmt=%d",
		secid, days,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return f.parseStockKLine(body)
}

// parseStockKLine 解析股票K线数据
func (f *KLineFetcher) parseStockKLine(data []byte) ([]KLine, error) {
	var result struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	var klines []KLine
	for _, line := range result.Data.Klines {
		// 格式: 日期,开盘,收盘,最高,最低,成交量,成交额
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}

		open, _ := strconv.ParseFloat(parts[1], 64)
		close, _ := strconv.ParseFloat(parts[2], 64)
		high, _ := strconv.ParseFloat(parts[3], 64)
		low, _ := strconv.ParseFloat(parts[4], 64)
		volume, _ := strconv.ParseInt(parts[5], 10, 64)

		k := KLine{
			Date:   parts[0],
			Open:   open,
			Close:  close,
			High:   high,
			Low:    low,
			Volume: volume,
		}
		klines = append(klines, k)
	}

	return klines, nil
}

// FetchFuturesKLine 获取期货日K线数据
// code: 期货代码（如 nf_AU0）
// days: 获取天数
func (f *KLineFetcher) FetchFuturesKLine(code string, days int) ([]KLine, error) {
	// 期货使用新浪接口
	// nf_AU0 -> AU0
	symbol := code
	if len(code) > 3 && code[:3] == "nf_" {
		symbol = code[3:]
	}

	url := fmt.Sprintf(
		"https://stock2.finance.sina.com.cn/futures/api/jsonp.php/var=/InnerFuturesNewService.getDailyKLine?symbol=%s&_=%d",
		symbol, time.Now().UnixMilli(),
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return f.parseFuturesKLine(body, days)
}

// FuturesKLineData 期货K线数据结构
type FuturesKLineData struct {
	D string `json:"d"` // 日期
	O string `json:"o"` // 开盘
	H string `json:"h"` // 最高
	L string `json:"l"` // 最低
	C string `json:"c"` // 收盘
	V string `json:"v"` // 成交量
}

// parseFuturesKLine 解析期货K线数据
func (f *KLineFetcher) parseFuturesKLine(data []byte, days int) ([]KLine, error) {
	// 响应格式: var=([{...},{...},...])
	str := string(data)
	// 找到 JSON 数组部分
	start := -1
	end := -1
	for i, c := range str {
		if c == '[' {
			start = i
			break
		}
	}
	for i := len(str) - 1; i >= 0; i-- {
		if str[i] == ']' {
			end = i + 1
			break
		}
	}

	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("无法解析期货K线数据")
	}

	jsonStr := str[start:end]
	var rawData []FuturesKLineData
	if err := json.Unmarshal([]byte(jsonStr), &rawData); err != nil {
		return nil, err
	}

	var klines []KLine
	// 只取最后 days 条
	startIdx := 0
	if len(rawData) > days {
		startIdx = len(rawData) - days
	}

	for i := startIdx; i < len(rawData); i++ {
		row := rawData[i]
		open, _ := strconv.ParseFloat(row.O, 64)
		high, _ := strconv.ParseFloat(row.H, 64)
		low, _ := strconv.ParseFloat(row.L, 64)
		close, _ := strconv.ParseFloat(row.C, 64)
		volume, _ := strconv.ParseInt(row.V, 10, 64)

		k := KLine{
			Date:   row.D,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: volume,
		}
		klines = append(klines, k)
	}

	return klines, nil
}

