package backtest

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"stock/config"
)

type YAMLConfig struct {
	Backtest struct {
		Days          int     `yaml:"days"`
		Start         string  `yaml:"start"`
		End           string  `yaml:"end"`
		InitialCash   float64 `yaml:"initial_cash"`
		PositionPct   float64 `yaml:"position_pct"`
		SlippageBps   float64 `yaml:"slippage_bps"`
		CommissionBps float64 `yaml:"commission_bps"`
		StockLotSize  int64   `yaml:"stock_lot_size"`
		FuturesMult   float64 `yaml:"futures_multiplier"`
		FuturesMargin float64 `yaml:"futures_margin_rate"`

		Instruments struct {
			Stocks  []string `yaml:"stocks"`
			Futures []string `yaml:"futures"`
		} `yaml:"instruments"`
	} `yaml:"backtest"`

	Strategy struct {
		Type   string         `yaml:"type"`
		Params map[string]any `yaml:"params"`
	} `yaml:"strategy"`
}

type RunConfig struct {
	Days          int
	Start         time.Time
	End           time.Time
	InitialCash   float64
	PositionPct   float64
	SlippageBps   float64
	CommissionBps float64
	FuturesMargin float64

	Instruments []Instrument
	Strategy    Strategy

	// Scan-only options (not loaded from YAML)
	ScanChart     bool
	ScanChartDir  string
	ScanChartBars int
}

func DefaultRunConfig() RunConfig {
	return RunConfig{
		Days:          5000,
		InitialCash:   1_000_000,
		PositionPct:   1.0,
		SlippageBps:   5,
		CommissionBps: 1,
		FuturesMargin: 1.0,
		Instruments:   nil,
		Strategy:      NewTsaiSenStrategy(TsaiSenParams{}),
	}
}

func LoadRunConfig(path string) (RunConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RunConfig{}, fmt.Errorf("read config: %w", err)
	}

	var yc YAMLConfig
	if err := yaml.Unmarshal(raw, &yc); err != nil {
		return RunConfig{}, fmt.Errorf("parse yaml: %w", err)
	}

	cfg := DefaultRunConfig()

	if yc.Backtest.Days > 0 {
		cfg.Days = yc.Backtest.Days
	}
	if yc.Backtest.InitialCash > 0 {
		cfg.InitialCash = yc.Backtest.InitialCash
	}
	if yc.Backtest.PositionPct > 0 && yc.Backtest.PositionPct <= 1 {
		cfg.PositionPct = yc.Backtest.PositionPct
	}
	if yc.Backtest.SlippageBps >= 0 {
		cfg.SlippageBps = yc.Backtest.SlippageBps
	}
	if yc.Backtest.CommissionBps >= 0 {
		cfg.CommissionBps = yc.Backtest.CommissionBps
	}
	if yc.Backtest.FuturesMargin > 0 && yc.Backtest.FuturesMargin <= 1 {
		cfg.FuturesMargin = yc.Backtest.FuturesMargin
	}

	stockLotSize := yc.Backtest.StockLotSize
	if stockLotSize <= 0 {
		stockLotSize = 100
	}
	futuresMult := yc.Backtest.FuturesMult
	if futuresMult <= 0 {
		futuresMult = 1
	}

	var instruments []Instrument
	for _, s := range yc.Backtest.Instruments.Stocks {
		sym := s
		if sym == "" {
			continue
		}
		instruments = append(instruments, Instrument{
			Symbol:  sym,
			Type:    InstrumentTypeStock,
			LotSize: stockLotSize,
		})
	}
	for _, f := range yc.Backtest.Instruments.Futures {
		sym := config.NormalizeFuturesCode(f)
		if sym == "" {
			continue
		}
		instruments = append(instruments, Instrument{
			Symbol:     sym,
			Type:       InstrumentTypeFutures,
			Multiplier: futuresMult,
			AllowShort: true,
		})
	}
	cfg.Instruments = instruments

	if yc.Backtest.Start != "" {
		t, err := time.ParseInLocation("2006-01-02", yc.Backtest.Start, time.Local)
		if err != nil {
			return RunConfig{}, fmt.Errorf("invalid backtest.start: %w", err)
		}
		cfg.Start = t
	}
	if yc.Backtest.End != "" {
		t, err := time.ParseInLocation("2006-01-02", yc.Backtest.End, time.Local)
		if err != nil {
			return RunConfig{}, fmt.Errorf("invalid backtest.end: %w", err)
		}
		cfg.End = t
	}

	switch yc.Strategy.Type {
	case "", "tsai_sen":
		var p TsaiSenParams
		if yc.Strategy.Params != nil {
			b, _ := yaml.Marshal(yc.Strategy.Params)
			_ = yaml.Unmarshal(b, &p)
		}
		p = p.withDefaults()
		cfg.Strategy = NewTsaiSenStrategy(p)
	case "patterns":
		var p PatternsParams
		if yc.Strategy.Params != nil {
			b, _ := yaml.Marshal(yc.Strategy.Params)
			_ = yaml.Unmarshal(b, &p)
		}
		p = p.withDefaults()
		cfg.Strategy = NewPatternsStrategy(p)
	default:
		return RunConfig{}, fmt.Errorf("unknown strategy.type: %s", yc.Strategy.Type)
	}

	return cfg, nil
}
