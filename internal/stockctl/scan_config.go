package stockctl

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"stock/backtest"
	appcfg "stock/config"
)

func loadScanRunConfig(btConfigPath, serviceConfigPath string) (backtest.RunConfig, error) {
	btCfg, err := backtest.LoadRunConfig(btConfigPath)
	if err != nil {
		return backtest.RunConfig{}, err
	}

	path := strings.TrimSpace(serviceConfigPath)
	if path == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			path = "config.yaml"
		}
	}
	if path == "" {
		return btCfg, nil
	}

	cfg, err := appcfg.LoadFromFile(path)
	if err != nil {
		return backtest.RunConfig{}, fmt.Errorf("load config.yaml: %w", err)
	}

	// 扫描/回测目前仅支持 A股 与国内期货日K；外盘(hf_)仅用于实时行情监控，避免自动合并进扫描。
	btCfg.Instruments = mergeInstruments(btCfg.Instruments, cfg.Stocks, filterChinaFutures(cfg.Futures))
	return btCfg, nil
}

func filterChinaFutures(codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		s := strings.TrimSpace(c)
		if s == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(s), "hf_") {
			continue
		}
		out = append(out, s)
	}
	return out
}

func mergeInstruments(existing []backtest.Instrument, stocks []string, futures []string) []backtest.Instrument {
	stockLot := int64(100)
	futMult := 1.0
	for _, inst := range existing {
		if inst.Type == backtest.InstrumentTypeStock && inst.LotSize > 0 {
			stockLot = inst.LotSize
			break
		}
	}
	for _, inst := range existing {
		if inst.Type == backtest.InstrumentTypeFutures && inst.Multiplier > 0 {
			futMult = inst.Multiplier
			break
		}
	}

	type key struct {
		t backtest.InstrumentType
		s string
	}
	m := map[key]backtest.Instrument{}
	for _, inst := range existing {
		m[key{t: inst.Type, s: inst.Symbol}] = inst
	}

	for _, s := range stocks {
		sym := strings.TrimSpace(s)
		if sym == "" {
			continue
		}
		k := key{t: backtest.InstrumentTypeStock, s: sym}
		if _, ok := m[k]; ok {
			continue
		}
		m[k] = backtest.Instrument{Symbol: sym, Type: backtest.InstrumentTypeStock, LotSize: stockLot}
	}
	for _, f := range futures {
		sym := strings.TrimSpace(f)
		if sym == "" {
			continue
		}
		k := key{t: backtest.InstrumentTypeFutures, s: sym}
		if _, ok := m[k]; ok {
			continue
		}
		m[k] = backtest.Instrument{Symbol: sym, Type: backtest.InstrumentTypeFutures, Multiplier: futMult, AllowShort: true}
	}

	out := make([]backtest.Instrument, 0, len(m))
	for _, inst := range m {
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}
