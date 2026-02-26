package analyzer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type persistedAnalysis struct {
	Version int         `json:"version"`
	SavedAt time.Time   `json:"saved_at"`
	Items   []*Analysis `json:"items"`
}

func (a *ClaudeAnalyzer) LoadFromFile(path string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil
	}

	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	b = bytesTrimSpace(b)
	if len(b) == 0 {
		return nil
	}

	// New format: {"version":1,"saved_at":"...","items":[...]}
	var v persistedAnalysis
	if err := json.Unmarshal(b, &v); err == nil && len(v.Items) > 0 {
		for _, it := range v.Items {
			if it == nil || strings.TrimSpace(it.Code) == "" {
				continue
			}
			a.results.Store(it.Code, it)
		}
		return nil
	}

	// Backward compatibility: plain array.
	var arr []*Analysis
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	for _, it := range arr {
		if it == nil || strings.TrimSpace(it.Code) == "" {
			continue
		}
		a.results.Store(it.Code, it)
	}
	return nil
}

func (a *ClaudeAnalyzer) SaveToFile(path string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	items := a.GetAllAnalysis()
	sort.Slice(items, func(i, j int) bool {
		if items[i] == nil {
			return false
		}
		if items[j] == nil {
			return true
		}
		return items[i].Code < items[j].Code
	})

	payload := persistedAnalysis{
		Version: 1,
		SavedAt: time.Now(),
		Items:   items,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	// Atomic-ish write.
	dir := filepath.Dir(p)
	tmp, err := os.CreateTemp(dir, ".ai-analysis-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(b); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, p)
}

func bytesTrimSpace(b []byte) []byte {
	// avoid importing bytes just for TrimSpace
	i := 0
	for i < len(b) {
		c := b[i]
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			break
		}
		i++
	}
	j := len(b)
	for j > i {
		c := b[j-1]
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			break
		}
		j--
	}
	return b[i:j]
}
