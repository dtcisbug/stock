package backtest

import "testing"

func TestParseLLMBacktestConfigJSON_OK(t *testing.T) {
	raw := []byte(`{
  "backtest": {
    "days": 5000,
    "start": "2018-01-01",
    "end": "2025-12-31",
    "initial_cash": 1000000,
    "position_pct": 1.0,
    "slippage_bps": 5,
    "commission_bps": 1,
    "stock_lot_size": 100,
    "futures_multiplier": 1,
    "futures_margin_rate": 1,
    "instruments": { "stocks": ["sh600000"], "futures": ["nf_I0", "pp2605"] }
  },
  "strategy": {
    "type": "tsai_sen",
    "params": {
      "box_lookback": 60,
      "break_pct": 0.005,
      "reclaim_pct": 0,
      "flip_max_bars": 20,
      "entry_mode": "reclaim_support",
      "vol_ma_n": 20,
      "vol_ratio_min": 0,
      "enable_fake_breakout": true,
      "fake_max_bars": 10
    }
  }
}`)
	cfg, err := ParseLLMBacktestConfigJSON(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	y, err := cfg.ToYAML()
	if err != nil {
		t.Fatalf("to yaml: %v", err)
	}
	if len(y) == 0 {
		t.Fatalf("expected yaml output")
	}
}

func TestParseLLMBacktestConfigJSON_UnknownFieldRejected(t *testing.T) {
	raw := []byte(`{
  "backtest": {"days": 100, "initial_cash": 1, "position_pct": 1, "slippage_bps": 0, "commission_bps": 0, "stock_lot_size": 100, "futures_multiplier": 1, "futures_margin_rate": 1, "instruments": {"stocks": ["sh600000"], "futures": []}, "extra": 1},
  "strategy": {"type":"tsai_sen", "params":{"box_lookback":60,"break_pct":0.01,"reclaim_pct":0,"flip_max_bars":10,"entry_mode":"reclaim_support","vol_ma_n":20,"vol_ratio_min":0,"enable_fake_breakout":false,"fake_max_bars":10}}
}`)
	if _, err := ParseLLMBacktestConfigJSON(raw); err == nil {
		t.Fatalf("expected error")
	}
}

