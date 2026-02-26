package backtest

// TsaiSenLevels returns support/resistance levels at index i using TsaiSenParams.
func TsaiSenLevels(bars []Bar, i int, p TsaiSenParams) (support, resist float64) {
	return levels(bars, i, p)
}

// VolumeMA returns the simple moving average of volume at index i over window n.
func VolumeMA(bars []Bar, i int, n int) float64 {
	return volMA(bars, i, n)
}
