package backtest

import (
	"bytes"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
)

type ChartLine struct {
	Price float64
	Label string
	Color string
	Dash  bool
}

type ChartPoint struct {
	Date  string
	Price float64
	Label string
	Color string
}

type SVGChartOptions struct {
	Width  int
	Height int
}

func (o SVGChartOptions) withDefaults() SVGChartOptions {
	if o.Width <= 0 {
		o.Width = 980
	}
	if o.Height <= 0 {
		o.Height = 520
	}
	return o
}

func RenderCandlesSVG(symbol string, bars []Bar, lines []ChartLine, points []ChartPoint, opt SVGChartOptions) ([]byte, error) {
	opt = opt.withDefaults()
	if len(bars) < 2 {
		return nil, fmt.Errorf("not enough bars: %d", len(bars))
	}

	minP := math.Inf(1)
	maxP := math.Inf(-1)
	for _, b := range bars {
		if b.Low > 0 && b.Low < minP {
			minP = b.Low
		}
		if b.High > 0 && b.High > maxP {
			maxP = b.High
		}
	}
	for _, ln := range lines {
		if ln.Price > 0 && ln.Price < minP {
			minP = ln.Price
		}
		if ln.Price > 0 && ln.Price > maxP {
			maxP = ln.Price
		}
	}
	for _, pt := range points {
		if pt.Price > 0 && pt.Price < minP {
			minP = pt.Price
		}
		if pt.Price > 0 && pt.Price > maxP {
			maxP = pt.Price
		}
	}
	if math.IsInf(minP, 0) || math.IsInf(maxP, 0) || maxP <= minP {
		return nil, fmt.Errorf("invalid price range")
	}
	pad := (maxP - minP) * 0.05
	if pad <= 0 {
		pad = minP * 0.02
	}
	minP -= pad
	maxP += pad

	// Layout
	w := float64(opt.Width)
	h := float64(opt.Height)
	mLeft := 70.0
	mRight := 20.0
	mTop := 24.0
	mBottom := 40.0
	plotW := w - mLeft - mRight
	plotH := h - mTop - mBottom
	if plotW <= 10 || plotH <= 10 {
		return nil, fmt.Errorf("invalid chart size")
	}

	priceToY := func(p float64) float64 {
		if p <= 0 {
			return mTop + plotH/2
		}
		r := (p - minP) / (maxP - minP)
		r = math.Max(0, math.Min(1, r))
		return mTop + (1.0-r)*plotH
	}

	n := float64(len(bars))
	step := plotW / n
	cw := math.Max(1.0, step*0.65)

	xAt := func(i int) float64 {
		return mLeft + (float64(i)+0.5)*step
	}

	bg := "#0b1220"
	grid := "rgba(255,255,255,0.08)"
	up := "#22c55e"
	down := "#ef4444"
	txt := "rgba(255,255,255,0.85)"

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="` + strconv.Itoa(opt.Width) + `" height="` + strconv.Itoa(opt.Height) + `" viewBox="0 0 ` + strconv.Itoa(opt.Width) + ` ` + strconv.Itoa(opt.Height) + `">` + "\n")
	buf.WriteString(`<rect x="0" y="0" width="100%" height="100%" fill="` + bg + `"/>` + "\n")

	// Header
	firstD := bars[0].Time.Format("2006-01-02")
	lastD := bars[len(bars)-1].Time.Format("2006-01-02")
	title := strings.TrimSpace(symbol)
	if title == "" {
		title = "UNKNOWN"
	}
	buf.WriteString(`<text x="` + fmtFloat(mLeft) + `" y="16" fill="` + txt + `" font-size="14" font-family="ui-monospace, Menlo, Monaco, Consolas, monospace">` +
		html.EscapeString(title) + `  ` + html.EscapeString(firstD) + ` ~ ` + html.EscapeString(lastD) + `</text>` + "\n")

	// Grid: price lines (5)
	for k := 0; k <= 5; k++ {
		y := mTop + (float64(k)/5.0)*plotH
		buf.WriteString(`<line x1="` + fmtFloat(mLeft) + `" y1="` + fmtFloat(y) + `" x2="` + fmtFloat(mLeft+plotW) + `" y2="` + fmtFloat(y) + `" stroke="` + grid + `" stroke-width="1"/>` + "\n")
		p := maxP - (float64(k)/5.0)*(maxP-minP)
		buf.WriteString(`<text x="` + fmtFloat(6) + `" y="` + fmtFloat(y+4) + `" fill="` + txt + `" font-size="12" font-family="ui-monospace, Menlo, Monaco, Consolas, monospace">` +
			html.EscapeString(fmtPrice(p)) + `</text>` + "\n")
	}

	// Candles
	for i, b := range bars {
		x := xAt(i)
		o := b.Open
		c := b.Close
		hi := b.High
		lo := b.Low
		col := up
		if c < o {
			col = down
		}

		yHi := priceToY(hi)
		yLo := priceToY(lo)
		yO := priceToY(o)
		yC := priceToY(c)
		yTop := math.Min(yO, yC)
		yBot := math.Max(yO, yC)
		if yBot-yTop < 1 {
			yBot = yTop + 1
		}

		// wick
		buf.WriteString(`<line x1="` + fmtFloat(x) + `" y1="` + fmtFloat(yHi) + `" x2="` + fmtFloat(x) + `" y2="` + fmtFloat(yLo) + `" stroke="` + col + `" stroke-width="1"/>` + "\n")
		// body
		buf.WriteString(`<rect x="` + fmtFloat(x-cw/2) + `" y="` + fmtFloat(yTop) + `" width="` + fmtFloat(cw) + `" height="` + fmtFloat(yBot-yTop) + `" fill="` + col + `" opacity="0.9"/>` + "\n")
	}

	// Overlay lines
	for _, ln := range lines {
		if ln.Price <= 0 {
			continue
		}
		col := strings.TrimSpace(ln.Color)
		if col == "" {
			col = "rgba(255,255,255,0.65)"
		}
		y := priceToY(ln.Price)
		style := ""
		if ln.Dash {
			style = ` stroke-dasharray="6 6"`
		}
		buf.WriteString(`<line x1="` + fmtFloat(mLeft) + `" y1="` + fmtFloat(y) + `" x2="` + fmtFloat(mLeft+plotW) + `" y2="` + fmtFloat(y) + `" stroke="` + col + `" stroke-width="1.2"` + style + `/>` + "\n")
		label := strings.TrimSpace(ln.Label)
		if label != "" {
			buf.WriteString(`<text x="` + fmtFloat(mLeft+6) + `" y="` + fmtFloat(y-4) + `" fill="` + col + `" font-size="12" font-family="ui-monospace, Menlo, Monaco, Consolas, monospace">` +
				html.EscapeString(label) + ` ` + html.EscapeString(fmtPrice(ln.Price)) + `</text>` + "\n")
		}
	}

	// Overlay points
	for _, pt := range points {
		if pt.Price <= 0 {
			continue
		}
		col := strings.TrimSpace(pt.Color)
		if col == "" {
			col = "#38bdf8"
		}
		// locate x by date
		x := -1.0
		for i := range bars {
			if bars[i].Time.Format("2006-01-02") == pt.Date {
				x = xAt(i)
				break
			}
		}
		if x < 0 {
			continue
		}
		y := priceToY(pt.Price)
		buf.WriteString(`<circle cx="` + fmtFloat(x) + `" cy="` + fmtFloat(y) + `" r="3.5" fill="` + col + `" />` + "\n")
		label := strings.TrimSpace(pt.Label)
		if label != "" {
			buf.WriteString(`<text x="` + fmtFloat(x+6) + `" y="` + fmtFloat(y-6) + `" fill="` + col + `" font-size="12" font-family="ui-monospace, Menlo, Monaco, Consolas, monospace">` +
				html.EscapeString(label) + `</text>` + "\n")
		}
	}

	// Footer dates
	buf.WriteString(`<text x="` + fmtFloat(mLeft) + `" y="` + fmtFloat(mTop+plotH+mBottom-12) + `" fill="` + txt + `" font-size="12" font-family="ui-monospace, Menlo, Monaco, Consolas, monospace">` +
		html.EscapeString(firstD) + `</text>` + "\n")
	buf.WriteString(`<text x="` + fmtFloat(mLeft+plotW-70) + `" y="` + fmtFloat(mTop+plotH+mBottom-12) + `" fill="` + txt + `" font-size="12" font-family="ui-monospace, Menlo, Monaco, Consolas, monospace">` +
		html.EscapeString(lastD) + `</text>` + "\n")

	buf.WriteString(`</svg>` + "\n")
	return buf.Bytes(), nil
}

func fmtFloat(x float64) string {
	// stable compact formatting for SVG attributes
	return strconv.FormatFloat(x, 'f', 2, 64)
}

func fmtPrice(p float64) string {
	// keep price labels readable
	if p >= 1000 {
		return strconv.FormatFloat(p, 'f', 0, 64)
	}
	if p >= 100 {
		return strconv.FormatFloat(p, 'f', 1, 64)
	}
	return strconv.FormatFloat(p, 'f', 2, 64)
}

