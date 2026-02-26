package main

import (
	"os"

	"stock/internal/stockctl"
)

func main() {
	os.Exit(stockctl.Run(os.Args[1:]))
}
