package main

import (
	"os"

	"stock/internal/stockd"
)

func main() {
	os.Exit(stockd.Run(os.Args[1:]))
}
