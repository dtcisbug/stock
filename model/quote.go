package model

import "time"

// StockQuote A股实时报价
type StockQuote struct {
	Code      string    `json:"code"`       // 股票代码 (sh600000, sz000001)
	Name      string    `json:"name"`       // 股票名称
	Open      float64   `json:"open"`       // 今开
	PreClose  float64   `json:"pre_close"`  // 昨收
	Price     float64   `json:"price"`      // 当前价
	High      float64   `json:"high"`       // 最高
	Low       float64   `json:"low"`        // 最低
	Volume    int64     `json:"volume"`     // 成交量（股）
	Amount    float64   `json:"amount"`     // 成交额（元）
	Bid1Price float64   `json:"bid1_price"` // 买一价
	Bid1Vol   int64     `json:"bid1_vol"`   // 买一量
	Bid2Price float64   `json:"bid2_price"` // 买二价
	Bid2Vol   int64     `json:"bid2_vol"`   // 买二量
	Bid3Price float64   `json:"bid3_price"` // 买三价
	Bid3Vol   int64     `json:"bid3_vol"`   // 买三量
	Bid4Price float64   `json:"bid4_price"` // 买四价
	Bid4Vol   int64     `json:"bid4_vol"`   // 买四量
	Bid5Price float64   `json:"bid5_price"` // 买五价
	Bid5Vol   int64     `json:"bid5_vol"`   // 买五量
	Ask1Price float64   `json:"ask1_price"` // 卖一价
	Ask1Vol   int64     `json:"ask1_vol"`   // 卖一量
	Ask2Price float64   `json:"ask2_price"` // 卖二价
	Ask2Vol   int64     `json:"ask2_vol"`   // 卖二量
	Ask3Price float64   `json:"ask3_price"` // 卖三价
	Ask3Vol   int64     `json:"ask3_vol"`   // 卖三量
	Ask4Price float64   `json:"ask4_price"` // 卖四价
	Ask4Vol   int64     `json:"ask4_vol"`   // 卖四量
	Ask5Price float64   `json:"ask5_price"` // 卖五价
	Ask5Vol   int64     `json:"ask5_vol"`   // 卖五量
	Date      string    `json:"date"`       // 日期
	Time      string    `json:"time"`       // 时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// Change 计算涨跌额
func (q *StockQuote) Change() float64 {
	return q.Price - q.PreClose
}

// ChangePercent 计算涨跌幅
func (q *StockQuote) ChangePercent() float64 {
	if q.PreClose == 0 {
		return 0
	}
	return (q.Price - q.PreClose) / q.PreClose * 100
}

// FuturesQuote 期货实时报价
type FuturesQuote struct {
	Code         string    `json:"code"`          // 合约代码
	Name         string    `json:"name"`          // 合约名称
	Open         float64   `json:"open"`          // 今开
	High         float64   `json:"high"`          // 最高
	Low          float64   `json:"low"`           // 最低
	PreClose     float64   `json:"pre_close"`     // 昨收
	PreSettle    float64   `json:"pre_settle"`    // 昨结算
	Price        float64   `json:"price"`         // 最新价
	Settle       float64   `json:"settle"`        // 结算价
	Bid          float64   `json:"bid"`           // 买价
	BidVol       int64     `json:"bid_vol"`       // 买量
	Ask          float64   `json:"ask"`           // 卖价
	AskVol       int64     `json:"ask_vol"`       // 卖量
	Volume       int64     `json:"volume"`        // 成交量
	OpenInterest int64     `json:"open_interest"` // 持仓量
	Date         string    `json:"date"`          // 日期
	Time         string    `json:"time"`          // 时间
	UpdatedAt    time.Time `json:"updated_at"`    // 更新时间
}

// Change 计算涨跌额
func (q *FuturesQuote) Change() float64 {
	return q.Price - q.PreSettle
}

// ChangePercent 计算涨跌幅
func (q *FuturesQuote) ChangePercent() float64 {
	if q.PreSettle == 0 {
		return 0
	}
	return (q.Price - q.PreSettle) / q.PreSettle * 100
}
