package backtest

import "testing"

func TestDetectMTop(t *testing.T) {
	// Price path: peak -> trough (neck) -> peak -> break below neck.
	prices := []float64{
		10, 20, 40, 60, 80, 100, 90, 80, 90, 101, 95, 79,
	}
	bars := make([]Bar, 0, len(prices))
	for _, p := range prices {
		bars = append(bars, Bar{Open: p, High: p, Low: p, Close: p, Volume: 100})
	}

	p := PatternsParams{
		Lookback:      100,
		PivotN:        1,
		EqualTolPct:   0.05,
		BreakPct:      0.0,
		StopBufferPct: 0.01,
		EnableMTop:    true,
	}
	s := NewPatternsStrategy(p)

	var got *Signal
	for i := range bars {
		got = s.OnBar(i, bars, Position{Side: SideFlat})
	}
	if got == nil || got.Action != SignalShort {
		t.Fatalf("expected short signal, got %#v", got)
	}
}

func TestDetectWBottom(t *testing.T) {
	// Price path: trough -> peak (neck) -> trough -> break above neck.
	prices := []float64{
		100, 80, 60, 40, 20, 30, 40, 60, 80, 60, 40, 21, 30, 50, 81,
	}
	bars := make([]Bar, 0, len(prices))
	for _, p := range prices {
		bars = append(bars, Bar{Open: p, High: p, Low: p, Close: p, Volume: 100})
	}

	p := PatternsParams{
		Lookback:       200,
		PivotN:         1,
		EqualTolPct:    0.10,
		BreakPct:       0.0,
		StopBufferPct:  0.01,
		EnableWBottom:  true,
		TargetMultiple: 1,
	}
	s := NewPatternsStrategy(p)

	var got *Signal
	for i := range bars {
		got = s.OnBar(i, bars, Position{Side: SideFlat})
	}
	if got == nil || got.Action != SignalBuy {
		t.Fatalf("expected buy signal, got %#v", got)
	}
}
