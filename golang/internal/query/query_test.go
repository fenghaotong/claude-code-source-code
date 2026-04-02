package query_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fenghaotong/claude-code/golang/internal/api"
	"github.com/fenghaotong/claude-code/golang/internal/query"
)

func TestMessageFromText(t *testing.T) {
	msg := query.MessageFromText(api.RoleUser, "hello")
	if msg.Role != api.RoleUser {
		t.Errorf("expected role %q, got %q", api.RoleUser, msg.Role)
	}
	if len(msg.Content) != 1 || msg.Content[0].Text != "hello" {
		t.Errorf("unexpected content: %v", msg.Content)
	}
}

func TestLastAssistantText(t *testing.T) {
	msgs := []api.Message{
		query.MessageFromText(api.RoleUser, "hi"),
		{
			Role: api.RoleAssistant,
			Content: []api.ContentBlock{
				{Type: "text", Text: "Hello! How can I help?"},
			},
		},
	}

	text := query.LastAssistantText(msgs)
	if text != "Hello! How can I help?" {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestLastAssistantText_Empty(t *testing.T) {
	text := query.LastAssistantText(nil)
	if text != "" {
		t.Errorf("expected empty string, got %q", text)
	}
}

func TestMarshalUnmarshalHistory(t *testing.T) {
	history := []api.Message{
		query.MessageFromText(api.RoleUser, "test prompt"),
		{
			Role: api.RoleAssistant,
			Content: []api.ContentBlock{
				{Type: "text", Text: "test response"},
			},
		},
	}

	data, err := query.MarshalHistory(history)
	if err != nil {
		t.Fatalf("MarshalHistory error: %v", err)
	}

	restored, err := query.UnmarshalHistory(data)
	if err != nil {
		t.Fatalf("UnmarshalHistory error: %v", err)
	}

	if len(restored) != len(history) {
		t.Fatalf("expected %d messages, got %d", len(history), len(restored))
	}
	if restored[0].Role != api.RoleUser {
		t.Errorf("expected role %q, got %q", api.RoleUser, restored[0].Role)
	}
}

// mockTool is a simple tool for testing.
type mockTool struct {
	name   string
	result string
}

func (m *mockTool) Name() string              { return m.name }
func (m *mockTool) Description() string       { return "mock tool" }
func (m *mockTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object","properties":{}}`) }
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (*struct {
	Content string
	IsError bool
}, error) {
	return &struct {
		Content string
		IsError bool
	}{Content: m.result}, nil
}

func TestUnmarshalHistory_InvalidJSON(t *testing.T) {
	_, err := query.UnmarshalHistory([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMarshalHistory_ValidJSON(t *testing.T) {
	history := []api.Message{query.MessageFromText(api.RoleUser, "hello")}
	data, err := query.MarshalHistory(history)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Error("serialised history should contain 'hello'")
	}
	if !json.Valid(data) {
		t.Error("serialised history is not valid JSON")
	}
}
