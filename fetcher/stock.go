package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"stock/model"
)

const (
	// 新浪股票行情接口
	sinaStockURL = "http://hq.sinajs.cn/list=%s"
)

// StockFetcher 股票数据拉取器
type StockFetcher struct {
	client *http.Client
}

// NewStockFetcher 创建股票数据拉取器
func NewStockFetcher() *StockFetcher {
	return &StockFetcher{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Fetch 拉取多只股票的实时行情
func (f *StockFetcher) Fetch(codes []string) ([]*model.StockQuote, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	// 构建请求URL
	url := fmt.Sprintf(sinaStockURL, strings.Join(codes, ","))

	// 发送请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取并转换编码（新浪返回GBK编码）
	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析数据
	return f.parse(string(body), codes)
}

// FetchOne 拉取单只股票的实时行情
func (f *StockFetcher) FetchOne(code string) (*model.StockQuote, error) {
	quotes, err := f.Fetch([]string{code})
	if err != nil {
		return nil, err
	}
	if len(quotes) == 0 {
		return nil, fmt.Errorf("未获取到股票 %s 的数据", code)
	}
	return quotes[0], nil
}

// parse 解析新浪股票数据
// 格式: var hq_str_sh600000="浦发银行,11.85,11.83,11.80,11.89,11.77,11.79,11.80,46778853,552469367.00,...";
func (f *StockFetcher) parse(data string, codes []string) ([]*model.StockQuote, error) {
	var quotes []*model.StockQuote

	// 正则匹配每行数据
	re := regexp.MustCompile(`var hq_str_(\w+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		code := match[1]
		content := match[2]

		if content == "" {
			continue // 无数据
		}

		quote, err := f.parseOneLine(code, content)
		if err != nil {
			continue // 跳过解析失败的
		}
		quotes = append(quotes, quote)
	}

	return quotes, nil
}

// parseOneLine 解析单行股票数据
// 字段顺序：名称,今开,昨收,当前价,最高,最低,买一价,卖一价,成交量,成交额,
//           买一量,买一价,买二量,买二价,买三量,买三价,买四量,买四价,买五量,买五价,
//           卖一量,卖一价,卖二量,卖二价,卖三量,卖三价,卖四量,卖四价,卖五量,卖五价,
//           日期,时间,...
func (f *StockFetcher) parseOneLine(code, content string) (*model.StockQuote, error) {
	fields := strings.Split(content, ",")
	if len(fields) < 32 {
		return nil, fmt.Errorf("字段数量不足: %d", len(fields))
	}

	quote := &model.StockQuote{
		Code:      code,
		Name:      fields[0],
		Open:      parseFloat(fields[1]),
		PreClose:  parseFloat(fields[2]),
		Price:     parseFloat(fields[3]),
		High:      parseFloat(fields[4]),
		Low:       parseFloat(fields[5]),
		Volume:    parseInt(fields[8]),
		Amount:    parseFloat(fields[9]),
		Bid1Price: parseFloat(fields[11]),
		Bid1Vol:   parseInt(fields[10]),
		Bid2Price: parseFloat(fields[13]),
		Bid2Vol:   parseInt(fields[12]),
		Bid3Price: parseFloat(fields[15]),
		Bid3Vol:   parseInt(fields[14]),
		Bid4Price: parseFloat(fields[17]),
		Bid4Vol:   parseInt(fields[16]),
		Bid5Price: parseFloat(fields[19]),
		Bid5Vol:   parseInt(fields[18]),
		Ask1Price: parseFloat(fields[21]),
		Ask1Vol:   parseInt(fields[20]),
		Ask2Price: parseFloat(fields[23]),
		Ask2Vol:   parseInt(fields[22]),
		Ask3Price: parseFloat(fields[25]),
		Ask3Vol:   parseInt(fields[24]),
		Ask4Price: parseFloat(fields[27]),
		Ask4Vol:   parseInt(fields[26]),
		Ask5Price: parseFloat(fields[29]),
		Ask5Vol:   parseInt(fields[28]),
		Date:      fields[30],
		Time:      fields[31],
		UpdatedAt: time.Now(),
	}

	return quote, nil
}

// parseFloat 解析浮点数
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseInt 解析整数
func parseInt(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
