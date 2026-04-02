// Package query implements the core agent loop: send messages → get response → execute tools → repeat.
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/fenghaotong/claude-code/golang/internal/api"
	"github.com/fenghaotong/claude-code/golang/internal/tools"
)

const maxAgentTurns = 50

// Options configures a single query invocation.
type Options struct {
	// SystemPrompt is prepended to every request as the system message.
	SystemPrompt string
	// MaxTurns limits the number of tool-use iterations (default: 50).
	MaxTurns int
	// Output is where streaming text is written (typically os.Stdout).
	Output io.Writer
}

// Run executes the agent loop.  It appends the user's messages to history,
// sends requests to the Claude API, executes any requested tools, and returns
// the updated conversation history when the model stops requesting tool use or
// MaxTurns is exhausted.
func Run(
	ctx context.Context,
	client *api.Client,
	model string,
	maxTokens int,
	history []api.Message,
	registry *tools.Registry,
	opts Options,
) ([]api.Message, error) {
	maxTurns := opts.MaxTurns
	if maxTurns <= 0 {
		maxTurns = maxAgentTurns
	}

	// Build the API tool descriptors from the registry.
	apiTools := buildAPITools(registry)

	messages := history

	for turn := 0; turn < maxTurns; turn++ {
		req := api.MessagesRequest{
			Model:     model,
			MaxTokens: maxTokens,
			System:    opts.SystemPrompt,
			Messages:  messages,
			Tools:     apiTools,
		}

		result, err := client.SendStreaming(ctx, req, func(evt api.RawStreamEvent) {
			if evt.Type == api.EventContentBlockDelta && evt.Delta != nil && evt.Delta.Text != "" {
				if opts.Output != nil {
					fmt.Fprint(opts.Output, evt.Delta.Text)
				}
			}
		})
		if err != nil {
			return messages, fmt.Errorf("API call failed: %w", err)
		}

		// Add a newline after streaming text if we printed anything.
		if opts.Output != nil {
			hasText := false
			for _, block := range result.Content {
				if block.Type == "text" && block.Text != "" {
					hasText = true
					break
				}
			}
			if hasText {
				fmt.Fprintln(opts.Output)
			}
		}

		// Append the assistant message to history.
		assistantMsg := api.Message{
			Role:    api.RoleAssistant,
			Content: result.Content,
		}
		messages = append(messages, assistantMsg)

		// If the model didn't request any tools, we're done.
		toolUses := filterToolUse(result.Content)
		if len(toolUses) == 0 {
			break
		}

		// Execute all requested tools and collect results.
		toolResults, err := executeTools(ctx, registry, toolUses, opts.Output)
		if err != nil {
			return messages, fmt.Errorf("tool execution failed: %w", err)
		}

		// Append a user message containing all tool results.
		messages = append(messages, api.Message{
			Role:    api.RoleUser,
			Content: toolResults,
		})
	}

	return messages, nil
}

// --- helpers ---

func buildAPITools(registry *tools.Registry) []api.Tool {
	var out []api.Tool
	for _, t := range registry.All() {
		out = append(out, api.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return out
}

func filterToolUse(blocks []api.ContentBlock) []api.ContentBlock {
	var out []api.ContentBlock
	for _, b := range blocks {
		if b.Type == "tool_use" {
			out = append(out, b)
		}
	}
	return out
}

func executeTools(
	ctx context.Context,
	registry *tools.Registry,
	toolUses []api.ContentBlock,
	output io.Writer,
) ([]api.ContentBlock, error) {
	var results []api.ContentBlock

	for _, tu := range toolUses {
		tool, ok := registry.Get(tu.Name)
		if !ok {
			// Return an error result for unknown tools rather than aborting.
			results = append(results, api.ContentBlock{
				Type:      "tool_result",
				ToolUseID: tu.ID,
				Content:   fmt.Sprintf("Unknown tool: %s", tu.Name),
				IsError:   true,
			})
			continue
		}

		if output != nil {
			fmt.Fprintf(output, "\n[tool: %s]\n", tu.Name)
		}

		toolResult, err := tool.Execute(ctx, tu.Input)
		if err != nil {
			results = append(results, api.ContentBlock{
				Type:      "tool_result",
				ToolUseID: tu.ID,
				Content:   fmt.Sprintf("Tool execution error: %v", err),
				IsError:   true,
			})
			continue
		}

		resultContent := toolResult.Content
		if output != nil && toolResult.IsError {
			fmt.Fprintf(output, "[error] %s\n", resultContent)
		}

		results = append(results, api.ContentBlock{
			Type:      "tool_result",
			ToolUseID: tu.ID,
			Content:   resultContent,
			IsError:   toolResult.IsError,
		})
	}

	return results, nil
}

// MessageFromText creates a simple text user message.
func MessageFromText(role api.Role, text string) api.Message {
	return api.Message{
		Role:    role,
		Content: []api.ContentBlock{{Type: "text", Text: text}},
	}
}

// LastAssistantText extracts the concatenated text from the last assistant message.
func LastAssistantText(messages []api.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == api.RoleAssistant {
			var sb string
			for _, block := range messages[i].Content {
				if block.Type == "text" {
					sb += block.Text
				}
			}
			return sb
		}
	}
	return ""
}

// MarshalHistory serialises the conversation history to JSON.
func MarshalHistory(messages []api.Message) ([]byte, error) {
	return json.MarshalIndent(messages, "", "  ")
}

// UnmarshalHistory deserialises conversation history from JSON.
func UnmarshalHistory(data []byte) ([]api.Message, error) {
	var messages []api.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}
