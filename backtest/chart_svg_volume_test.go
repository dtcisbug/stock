package backtest

import (
	"strings"
	"testing"
	"time"
)

func TestRenderCandlesWithVolumeSVG_IncludesVolumePanel(t *testing.T) {
	bars := []Bar{
		{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local), Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 100},
		{Time: time.Date(2025, 1, 2, 0, 0, 0, 0, time.Local), Open: 10.5, High: 12, Low: 10, Close: 11.5, Volume: 200},
		{Time: time.Date(2025, 1, 3, 0, 0, 0, 0, time.Local), Open: 11.5, High: 12, Low: 11, Close: 11.0, Volume: 150},
	}

	svg, err := RenderCandlesWithVolumeSVG("sh600000", bars, nil, nil, 2, SVGChartOptions{})
	if err != nil {
		t.Fatalf("RenderCandlesWithVolumeSVG: %v", err)
	}
	s := string(svg)
	if !strings.Contains(s, "VOLUME") {
		t.Fatalf("expected volume label in svg")
	}
	if !strings.Contains(s, "MA2") {
		t.Fatalf("expected MA label in svg")
	}
	if !strings.Contains(s, "<polyline") {
		t.Fatalf("expected MA polyline in svg")
	}
}
