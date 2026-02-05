package main

import (
	"fmt"
	"os"

	"stock/backtest"
)

func runBacktest(configPath, outPath string) error {
	cfg, err := backtest.LoadRunConfig(configPath)
	if err != nil {
		return err
	}

	runner := backtest.NewRunner()
	results, err := runner.Run(cfg)
	if err != nil {
		return err
	}

	if outPath == "" {
		return backtest.WriteResultsJSON(os.Stdout, results)
	}

	if err := ensureParentDir(outPath); err != nil {
		return fmt.Errorf("prepare output dir: %w", err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer f.Close()
	return backtest.WriteResultsJSON(f, results)
}
