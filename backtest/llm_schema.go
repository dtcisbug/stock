package backtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"stock/config"
)

type LLMBacktestConfig struct {
	Backtest LLMBacktestSection `json:"backtest"`
	Strategy LLMStrategySection `json:"strategy"`
}

type LLMBacktestSection struct {
	Days          int     `json:"days"`
	Start         string  `json:"start"`
	End           string  `json:"end"`
	InitialCash   float64 `json:"initial_cash"`
	PositionPct   float64 `json:"position_pct"`
	SlippageBps   float64 `json:"slippage_bps"`
	CommissionBps float64 `json:"commission_bps"`
	StockLotSize  int64   `json:"stock_lot_size"`

	FuturesMultiplier float64 `json:"futures_multiplier"`
	FuturesMarginRate float64 `json:"futures_margin_rate"`

	Instruments struct {
		Stocks  []string `json:"stocks"`
		Futures []string `json:"futures"`
	} `json:"instruments"`
}

type LLMStrategySection struct {
	Type   string        `json:"type"`
	Params TsaiSenParams `json:"params"`
}

func ParseLLMBacktestConfigJSON(raw []byte) (LLMBacktestConfig, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber()

	var cfg LLMBacktestConfig
	if err := dec.Decode(&cfg); err != nil {
		return LLMBacktestConfig{}, fmt.Errorf("invalid config json: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return LLMBacktestConfig{}, err
	}
	return cfg, nil
}

var stockCodeRe = regexp.MustCompile(`^(sh|sz)[0-9]{6}$`)

func (c LLMBacktestConfig) Validate() error {
	bt := c.Backtest

	if bt.Days <= 0 || bt.Days > 50000 {
		return fmt.Errorf("backtest.days out of range")
	}
	if bt.InitialCash <= 0 {
		return fmt.Errorf("backtest.initial_cash must be > 0")
	}
	if bt.PositionPct <= 0 || bt.PositionPct > 1 {
		return fmt.Errorf("backtest.position_pct must be in (0,1]")
	}
	if bt.SlippageBps < 0 || bt.SlippageBps > 500 {
		return fmt.Errorf("backtest.slippage_bps out of range")
	}
	if bt.CommissionBps < 0 || bt.CommissionBps > 500 {
		return fmt.Errorf("backtest.commission_bps out of range")
	}
	if bt.StockLotSize <= 0 {
		return fmt.Errorf("backtest.stock_lot_size must be > 0")
	}
	if bt.FuturesMultiplier <= 0 || bt.FuturesMultiplier > 100000 {
		return fmt.Errorf("backtest.futures_multiplier out of range")
	}
	if bt.FuturesMarginRate <= 0 || bt.FuturesMarginRate > 1 {
		return fmt.Errorf("backtest.futures_margin_rate must be in (0,1]")
	}

	var start, end time.Time
	if bt.Start != "" {
		t, err := time.ParseInLocation("2006-01-02", bt.Start, time.Local)
		if err != nil {
			return fmt.Errorf("backtest.start invalid: %w", err)
		}
		start = t
	}
	if bt.End != "" {
		t, err := time.ParseInLocation("2006-01-02", bt.End, time.Local)
		if err != nil {
			return fmt.Errorf("backtest.end invalid: %w", err)
		}
		end = t
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		return fmt.Errorf("backtest.end must be >= backtest.start")
	}

	if len(bt.Instruments.Stocks) == 0 && len(bt.Instruments.Futures) == 0 {
		return fmt.Errorf("backtest.instruments must not be empty")
	}

	for _, s := range bt.Instruments.Stocks {
		sym := strings.TrimSpace(s)
		if sym == "" || !stockCodeRe.MatchString(sym) {
			return fmt.Errorf("invalid stock code: %q", s)
		}
	}
	for _, f := range bt.Instruments.Futures {
		sym := config.NormalizeFuturesCode(f)
		if sym == "" || !strings.HasPrefix(strings.ToLower(sym), "nf_") {
			return fmt.Errorf("invalid futures code: %q", f)
		}
	}

	st := strings.TrimSpace(c.Strategy.Type)
	if st == "" {
		return fmt.Errorf("strategy.type required")
	}
	if st != "tsai_sen" {
		return fmt.Errorf("unsupported strategy.type: %s", st)
	}

	p := c.Strategy.Params.withDefaults()
	if p.LevelMode != "pivots" && p.LevelMode != "extremes" {
		return fmt.Errorf("strategy.params.level_mode invalid")
	}
	if p.BoxLookback < 10 || p.BoxLookback > 500 {
		return fmt.Errorf("strategy.params.box_lookback out of range")
	}
	if p.PivotN < 1 || p.PivotN > 20 {
		return fmt.Errorf("strategy.params.pivot_n out of range")
	}
	if p.TouchTolPct <= 0 || p.TouchTolPct > 0.05 {
		return fmt.Errorf("strategy.params.touch_tol_pct out of range")
	}
	if p.MinTouches < 2 || p.MinTouches > 50 {
		return fmt.Errorf("strategy.params.min_touches out of range")
	}
	if p.MinRangePct <= 0 || p.MinRangePct > 0.5 {
		return fmt.Errorf("strategy.params.min_range_pct out of range")
	}
	if p.BreakPct <= 0 || p.BreakPct > 0.2 {
		return fmt.Errorf("strategy.params.break_pct out of range")
	}
	if p.ReclaimPct < 0 || p.ReclaimPct > 0.2 {
		return fmt.Errorf("strategy.params.reclaim_pct out of range")
	}
	if p.FlipMaxBars <= 0 || p.FlipMaxBars > 300 {
		return fmt.Errorf("strategy.params.flip_max_bars out of range")
	}
	if p.EntryMode != "reclaim_support" && p.EntryMode != "stabilize_support" && p.EntryMode != "break_resistance" {
		return fmt.Errorf("strategy.params.entry_mode invalid")
	}
	if p.StabilizeBars < 1 || p.StabilizeBars > 60 {
		return fmt.Errorf("strategy.params.stabilize_bars out of range")
	}
	if p.StopBufferPct <= 0 || p.StopBufferPct > 0.2 {
		return fmt.Errorf("strategy.params.stop_buffer_pct out of range")
	}
	if p.TargetMultiple <= 0 || p.TargetMultiple > 10 {
		return fmt.Errorf("strategy.params.target_multiple out of range")
	}
	if p.VolMAN < 1 || p.VolMAN > 300 {
		return fmt.Errorf("strategy.params.vol_ma_n out of range")
	}
	if p.VolRatioMin < 0 || p.VolRatioMin > 50 {
		return fmt.Errorf("strategy.params.vol_ratio_min out of range")
	}
	if p.FakeMaxBars < 1 || p.FakeMaxBars > 300 {
		return fmt.Errorf("strategy.params.fake_max_bars out of range")
	}

	return nil
}

type yamlOut struct {
	Backtest struct {
		Days              int     `yaml:"days"`
		Start             string  `yaml:"start,omitempty"`
		End               string  `yaml:"end,omitempty"`
		InitialCash       float64 `yaml:"initial_cash"`
		PositionPct       float64 `yaml:"position_pct"`
		SlippageBps       float64 `yaml:"slippage_bps"`
		CommissionBps     float64 `yaml:"commission_bps"`
		StockLotSize      int64   `yaml:"stock_lot_size"`
		FuturesMultiplier float64 `yaml:"futures_multiplier"`
		FuturesMarginRate float64 `yaml:"futures_margin_rate"`
		Instruments       any     `yaml:"instruments"`
	} `yaml:"backtest"`

	Strategy struct {
		Type   string        `yaml:"type"`
		Params TsaiSenParams `yaml:"params"`
	} `yaml:"strategy"`
}

func (c LLMBacktestConfig) ToYAML() ([]byte, error) {
	var out yamlOut
	out.Backtest.Days = c.Backtest.Days
	out.Backtest.Start = c.Backtest.Start
	out.Backtest.End = c.Backtest.End
	out.Backtest.InitialCash = c.Backtest.InitialCash
	out.Backtest.PositionPct = c.Backtest.PositionPct
	out.Backtest.SlippageBps = c.Backtest.SlippageBps
	out.Backtest.CommissionBps = c.Backtest.CommissionBps
	out.Backtest.StockLotSize = c.Backtest.StockLotSize
	out.Backtest.FuturesMultiplier = c.Backtest.FuturesMultiplier
	out.Backtest.FuturesMarginRate = c.Backtest.FuturesMarginRate

	instruments := struct {
		Stocks  []string `yaml:"stocks,omitempty"`
		Futures []string `yaml:"futures,omitempty"`
	}{}

	for _, s := range c.Backtest.Instruments.Stocks {
		instruments.Stocks = append(instruments.Stocks, strings.TrimSpace(s))
	}
	for _, f := range c.Backtest.Instruments.Futures {
		instruments.Futures = append(instruments.Futures, config.NormalizeFuturesCode(f))
	}
	out.Backtest.Instruments = instruments

	out.Strategy.Type = "tsai_sen"
	out.Strategy.Params = c.Strategy.Params.withDefaults()

	b, err := yaml.Marshal(out)
	if err != nil {
		return nil, err
	}
	return b, nil
}
