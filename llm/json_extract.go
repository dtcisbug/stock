package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func ExtractFirstJSONValue(text string) (json.RawMessage, error) {
	b := []byte(text)
	start := bytes.IndexAny(b, "{[")
	if start < 0 {
		return nil, fmt.Errorf("no json start found")
	}

	dec := json.NewDecoder(bytes.NewReader(b[start:]))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("re-marshal json: %w", err)
	}
	return out, nil
}

