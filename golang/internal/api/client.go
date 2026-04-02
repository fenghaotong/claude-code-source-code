// Package api provides a client for the Anthropic Claude API with streaming support.
package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	messagesEndpoint = "/v1/messages"
	betaHeader       = "interleaved-thinking-2025-05-14"

	// APIBaseURL is the default Anthropic API base URL.
	APIBaseURL = "https://api.anthropic.com"
	// APIVersion is the Anthropic API version header value.
	APIVersion = "2023-06-01"
)

// Client is the Anthropic API client.
type Client struct {
	apiKey     string
	baseURL    string
	apiVersion string
	httpClient *http.Client
}

// NewClient creates a new Anthropic API client.
func NewClient(apiKey, baseURL, apiVersion string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		apiVersion: apiVersion,
		httpClient: &http.Client{Timeout: 10 * time.Minute},
	}
}

// --- Message types ---

// Role represents the sender of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ContentBlock is a single block inside a message.
type ContentBlock struct {
	Type string `json:"type"`

	// text block
	Text string `json:"text,omitempty"`

	// tool_use block
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result block
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Message is a single conversation turn.
type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// Tool describes a callable function exposed to the model.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// MessagesRequest is the body of a POST /v1/messages request.
type MessagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
	Stream    bool      `json:"stream"`
}

// UsageInfo contains token usage statistics.
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// MessagesResponse is the full (non-streaming) response body.
type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         Role           `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Usage        UsageInfo      `json:"usage"`
}

// --- Streaming event types ---

// StreamEventType is the SSE event name.
type StreamEventType string

const (
	EventMessageStart      StreamEventType = "message_start"
	EventContentBlockStart StreamEventType = "content_block_start"
	EventContentBlockDelta StreamEventType = "content_block_delta"
	EventContentBlockStop  StreamEventType = "content_block_stop"
	EventMessageDelta      StreamEventType = "message_delta"
	EventMessageStop       StreamEventType = "message_stop"
	EventError             StreamEventType = "error"
	EventPing              StreamEventType = "ping"
)

// RawStreamEvent is a single Server-Sent Event line decoded from the stream.
type RawStreamEvent struct {
	Type  StreamEventType `json:"type"`
	Index int             `json:"index"`

	// message_start
	Message *MessagesResponse `json:"message,omitempty"`

	// content_block_start
	ContentBlock *ContentBlock `json:"content_block,omitempty"`

	// content_block_delta
	Delta *ContentBlockDelta `json:"delta,omitempty"`

	// message_delta
	Usage *UsageInfo `json:"usage,omitempty"`

	// error
	Error *APIError `json:"error,omitempty"`
}

// ContentBlockDelta contains the incremental update for a content block.
type ContentBlockDelta struct {
	Type        string `json:"type"` // "text_delta" | "input_json_delta"
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

// APIError represents an error returned by the Anthropic API.
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("anthropic API error (%s): %s", e.Type, e.Message)
}

// StreamResult is the fully assembled response after consuming a streaming response.
type StreamResult struct {
	Content    []ContentBlock
	StopReason string
	Usage      UsageInfo
}

// SendStreaming sends a streaming messages request and calls handler for each
// raw SSE event. It returns the fully assembled StreamResult.
func (c *Client) SendStreaming(ctx context.Context, req MessagesRequest, handler func(RawStreamEvent)) (*StreamResult, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+messagesEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)
	httpReq.Header.Set("anthropic-version", c.apiVersion)
	httpReq.Header.Set("anthropic-beta", betaHeader)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(errBody))
	}

	return c.consumeStream(resp.Body, handler)
}

// consumeStream reads the SSE stream and assembles the final result.
func (c *Client) consumeStream(r io.Reader, handler func(RawStreamEvent)) (*StreamResult, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	result := &StreamResult{}

	// Per-block accumulators (indexed by block index).
	blockTexts := map[int]*strings.Builder{}
	blockJSONs := map[int]*strings.Builder{}
	blockTypes := map[int]string{}
	blockNames := map[int]string{}
	blockIDs := map[int]string{}

	var eventType string

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "event: "):
			eventType = strings.TrimPrefix(line, "event: ")

		case strings.HasPrefix(line, "data: "):
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}

			var evt RawStreamEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}

			// Backfill type from SSE event line when absent in JSON.
			if evt.Type == "" {
				evt.Type = StreamEventType(eventType)
			}

			handler(evt)

			switch evt.Type {
			case EventContentBlockStart:
				if evt.ContentBlock != nil {
					blockTypes[evt.Index] = evt.ContentBlock.Type
					blockNames[evt.Index] = evt.ContentBlock.Name
					blockIDs[evt.Index] = evt.ContentBlock.ID
					blockTexts[evt.Index] = &strings.Builder{}
					blockJSONs[evt.Index] = &strings.Builder{}
				}

			case EventContentBlockDelta:
				if evt.Delta != nil {
					if b, ok := blockTexts[evt.Index]; ok {
						b.WriteString(evt.Delta.Text)
					}
					if b, ok := blockJSONs[evt.Index]; ok {
						b.WriteString(evt.Delta.PartialJSON)
					}
				}

			case EventContentBlockStop:
				bt := blockTypes[evt.Index]
				switch bt {
				case "text":
					text := blockTexts[evt.Index].String()
					result.Content = append(result.Content, ContentBlock{Type: "text", Text: text})
				case "tool_use":
					rawInput := json.RawMessage(blockJSONs[evt.Index].String())
					if len(rawInput) == 0 {
						rawInput = json.RawMessage("{}")
					}
					result.Content = append(result.Content, ContentBlock{
						Type:  "tool_use",
						ID:    blockIDs[evt.Index],
						Name:  blockNames[evt.Index],
						Input: rawInput,
					})
				}

			case EventMessageDelta:
				if evt.Delta != nil {
					result.StopReason = evt.Delta.Type
				}
				if evt.Usage != nil {
					result.Usage.OutputTokens = evt.Usage.OutputTokens
				}

			case EventMessageStart:
				if evt.Message != nil {
					result.Usage.InputTokens = evt.Message.Usage.InputTokens
				}

			case EventError:
				if evt.Error != nil {
					return nil, evt.Error
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stream: %w", err)
	}

	return result, nil
}
