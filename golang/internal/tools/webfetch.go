package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	webFetchMaxBytes   = 200_000
	webFetchTimeoutSec = 30
)

// WebFetchTool fetches the content of a URL.
type WebFetchTool struct {
	client *http.Client
}

// NewWebFetchTool creates a new WebFetchTool.
func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		client: &http.Client{Timeout: webFetchTimeoutSec * time.Second},
	}
}

func (w *WebFetchTool) Name() string { return "WebFetch" }

func (w *WebFetchTool) Description() string {
	return `Fetches content from a URL and returns it as text.

Usage notes:
- Only HTTP and HTTPS URLs are supported.
- Response body is truncated at 200,000 characters.`
}

func (w *WebFetchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url": {
      "type": "string",
      "description": "The URL to fetch."
    },
    "prompt": {
      "type": "string",
      "description": "Optional description of what to look for in the response."
    }
  },
  "required": ["url"]
}`)
}

type webFetchInput struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt,omitempty"`
}

func (w *WebFetchTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var inp webFetchInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return nil, fmt.Errorf("invalid webfetch input: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, inp.URL, nil)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Invalid URL: %v", err), IsError: true}, nil
	}
	req.Header.Set("User-Agent", "claude-code-go/1.0")

	resp, err := w.client.Do(req)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Request failed: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(webFetchMaxBytes)+1))
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error reading response: %v", err), IsError: true}, nil
	}

	content := string(body)
	truncated := false
	if len(content) > webFetchMaxBytes {
		content = content[:webFetchMaxBytes]
		truncated = true
	}

	result := fmt.Sprintf("URL: %s\nStatus: %d\n\n%s", inp.URL, resp.StatusCode, content)
	if truncated {
		result += "\n... (response truncated)"
	}

	return &Result{Content: result}, nil
}
