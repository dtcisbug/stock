package backtest

import "time"

type InstrumentType string

const (
	InstrumentTypeStock   InstrumentType = "stock"
	InstrumentTypeFutures InstrumentType = "futures"
)

type Side string

const (
	SideFlat  Side = "flat"
	SideLong  Side = "long"
	SideShort Side = "short"
)

type Bar struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

type Instrument struct {
	Symbol     string
	Type       InstrumentType
	LotSize    int64   // stock only (default 100)
	Multiplier float64 // futures only (default 1)
	AllowShort bool    // futures only
}

type SignalAction string

const (
	SignalBuy   SignalAction = "buy"
	SignalSell  SignalAction = "sell"
	SignalShort SignalAction = "short"
	SignalCover SignalAction = "cover"
)

type Signal struct {
	Time   time.Time
	Action SignalAction
	Reason string
}

type Position struct {
	Side       Side
	Qty        float64
	EntryTime  time.Time
	EntryPrice float64
	EntryFee   float64
	Margin     float64
}

type Trade struct {
	Symbol      string  `json:"symbol"`
	Side        Side    `json:"side"`
	EntryTime   string  `json:"entry_time"`
	EntryPrice  float64 `json:"entry_price"`
	ExitTime    string  `json:"exit_time"`
	ExitPrice   float64 `json:"exit_price"`
	Qty         float64 `json:"qty"`
	GrossPnL    float64 `json:"gross_pnl"`
	NetPnL      float64 `json:"net_pnl"`
	ReturnPct   float64 `json:"return_pct"`
	ReasonEntry string  `json:"reason_entry"`
	ReasonExit  string  `json:"reason_exit"`
}
