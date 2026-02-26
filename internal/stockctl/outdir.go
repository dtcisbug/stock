package stockctl

import (
	"os"
	"path/filepath"
	"strings"
)

func ensureParentDir(path string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil
	}
	dir := filepath.Dir(p)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
