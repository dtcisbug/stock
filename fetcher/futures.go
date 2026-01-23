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
	// 新浪期货行情接口
	sinaFuturesURL = "http://hq.sinajs.cn/list=%s"
)

// FuturesFetcher 期货数据拉取器
type FuturesFetcher struct {
	client *http.Client
}

// NewFuturesFetcher 创建期货数据拉取器
func NewFuturesFetcher() *FuturesFetcher {
	return &FuturesFetcher{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Fetch 拉取多个期货合约的实时行情
func (f *FuturesFetcher) Fetch(codes []string) ([]*model.FuturesQuote, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	// 构建请求URL
	url := fmt.Sprintf(sinaFuturesURL, strings.Join(codes, ","))

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

	// 读取并转换编码
	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析数据
	return f.parse(string(body))
}

// FetchOne 拉取单个期货合约的实时行情
func (f *FuturesFetcher) FetchOne(code string) (*model.FuturesQuote, error) {
	quotes, err := f.Fetch([]string{code})
	if err != nil {
		return nil, err
	}
	if len(quotes) == 0 {
		return nil, fmt.Errorf("未获取到期货 %s 的数据", code)
	}
	return quotes[0], nil
}

// parse 解析新浪期货数据
func (f *FuturesFetcher) parse(data string) ([]*model.FuturesQuote, error) {
	var quotes []*model.FuturesQuote

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
			continue
		}

		quote, err := f.parseOneLine(code, content)
		if err != nil {
			continue
		}
		quotes = append(quotes, quote)
	}

	return quotes, nil
}

// parseOneLine 解析单行期货数据
// 新浪期货有两种格式：
// 1. 商品期货(AU/AG/CU等): 名称,?,开盘,最高,最低,?,买价,卖价,最新价,?,昨结算,买量,卖量,成交量,持仓量,...
// 2. 股指期货(IF/IC/IH/IM): 今开,最高,最低,最新价,成交量,成交额,持仓量,...,昨结算,...,日期,时间,...,名称
func (f *FuturesFetcher) parseOneLine(code, content string) (*model.FuturesQuote, error) {
	fields := strings.Split(content, ",")
	if len(fields) < 15 {
		return nil, fmt.Errorf("字段数量不足: %d", len(fields))
	}

	// 判断是哪种格式：第一个字段是数字则为股指期货
	firstField := strings.TrimSpace(fields[0])
	_, err := strconv.ParseFloat(firstField, 64)
	isIndexFutures := err == nil

	var quote *model.FuturesQuote

	if isIndexFutures {
		// 股指期货格式 (IF/IC/IH/IM)
		quote = &model.FuturesQuote{
			Code:         code,
			Name:         fields[len(fields)-1], // 名称在最后
			Open:         parseFloat(fields[0]),
			High:         parseFloat(fields[1]),
			Low:          parseFloat(fields[2]),
			Price:        parseFloat(fields[3]),
			Volume:       parseInt(fields[4]),
			OpenInterest: parseInt(fields[6]),
			PreSettle:    parseFloat(fields[9]),
			UpdatedAt:    time.Now(),
		}
		// 日期时间
		if len(fields) > 38 {
			quote.Date = fields[37]
			quote.Time = fields[38]
		}
	} else {
		// 商品期货格式 (AU/AG/CU/SC等)
		quote = &model.FuturesQuote{
			Code:         code,
			Name:         fields[0],
			Open:         parseFloat(fields[2]),
			High:         parseFloat(fields[3]),
			Low:          parseFloat(fields[4]),
			Bid:          parseFloat(fields[6]),
			Ask:          parseFloat(fields[7]),
			Price:        parseFloat(fields[8]),
			PreSettle:    parseFloat(fields[10]),
			BidVol:       parseInt(fields[11]),
			AskVol:       parseInt(fields[12]),
			Volume:       parseInt(fields[13]),
			OpenInterest: parseInt(fields[14]),
			UpdatedAt:    time.Now(),
		}
		// 日期
		if len(fields) > 17 {
			quote.Date = fields[17]
		}
	}

	return quote, nil
}
