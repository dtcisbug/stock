package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OllamaClient struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOllamaClient(baseURL, model string) *OllamaClient {
	return NewOllamaClientWithTimeout(baseURL, model, 10*time.Minute)
}

func NewOllamaClientWithTimeout(baseURL, model string, timeout time.Duration) *OllamaClient {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if u == "" {
		u = "http://localhost:11434"
	}
	m := strings.TrimSpace(model)
	if m == "" {
		m = "qwen2.5-coder:14b"
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	return &OllamaClient{
		BaseURL: u,
		Model:   m,
		Client:  &http.Client{Timeout: timeout},
	}
}

type GenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	System  string                 `json:"system,omitempty"`
	Stream  bool                   `json:"stream"`
	Options map[string]any         `json:"options,omitempty"`
	Format  string                 `json:"format,omitempty"`
	Context []int                  `json:"context,omitempty"`
	Raw     bool                   `json:"raw,omitempty"`
	Keep    string                 `json:"keep_alive,omitempty"`
	Extra   map[string]interface{} `json:"-"`
}

type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

func (c *OllamaClient) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	req.Model = c.Model
	req.Stream = false

	b, err := json.Marshal(req)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/generate", bytes.NewReader(b))
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GenerateResponse{}, fmt.Errorf("ollama http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out GenerateResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return GenerateResponse{}, fmt.Errorf("parse response: %w", err)
	}
	if out.Error != "" {
		return GenerateResponse{}, fmt.Errorf("ollama error: %s", out.Error)
	}
	return out, nil
}
