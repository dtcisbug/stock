package stock

import "path/filepath"

func DefaultAIStorePath() string {
	return filepath.Join("runtime", "ai", "analysis.json")
}
