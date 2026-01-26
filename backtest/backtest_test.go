package backtest

import "testing"

func TestTsaiSenBreakBottomFlipReclaimSupport(t *testing.T) {
	// Synthetic series:
	// - build a range support=10, resist=12
	// - break to 9 then reclaim close>10 => buy
	bars := make([]Bar, 0, 100)
	for i := 0; i < 60; i++ {
		bars = append(bars, Bar{Open: 11, High: 12, Low: 10, Close: 11, Volume: 100})
	}
	bars = append(bars, Bar{Open: 11, High: 11.5, Low: 9.0, Close: 9.5, Volume: 200})  // breakdown
	bars = append(bars, Bar{Open: 9.6, High: 10.6, Low: 9.4, Close: 10.2, Volume: 200}) // reclaim

	s := NewTsaiSenStrategy(TsaiSenParams{
		BoxLookback: 60,
		BreakPct:    0.01,
		EntryMode:   "reclaim_support",
	})

	var got *Signal
	for i := range bars {
		got = s.OnBar(i, bars, Position{Side: SideFlat})
		if got != nil {
			break
		}
	}
	if got == nil || got.Action != SignalBuy {
		t.Fatalf("expected buy signal, got %#v", got)
	}
}

