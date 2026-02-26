package main

import (
	"os"
	"strings"

	"stock/internal/stockctl"
	"stock/internal/stockd"
)

// Version is injected by build scripts via -ldflags "-X main.Version=..."
var Version = "dev"

func main() {
	args := os.Args[1:]
	if shouldRouteToCtl(args) {
		os.Exit(stockctl.Run(args))
	}
	os.Exit(stockd.Run(args))
}

func shouldRouteToCtl(args []string) bool {
	for _, a := range args {
		if a == "" {
			continue
		}
		if a == "-cli" || a == "--cli" {
			return true
		}
		if a == "-standalone" || a == "--standalone" {
			return true
		}
		if a == "-scan" || a == "--scan" || a == "-backtest" || a == "--backtest" || a == "-analyze" || a == "--analyze" {
			return true
		}
		if strings.HasPrefix(a, "-llm-") || strings.HasPrefix(a, "--llm-") {
			return true
		}
	}
	return false
}
