package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FileEditTool replaces exact occurrences of text within a file.
type FileEditTool struct{}

// NewFileEditTool creates a new FileEditTool.
func NewFileEditTool() *FileEditTool { return &FileEditTool{} }

func (f *FileEditTool) Name() string { return "Edit" }

func (f *FileEditTool) Description() string {
	return `Makes a surgical edit to a file by replacing one exact occurrence of old_string with new_string.

Usage notes:
- Provide enough context in old_string to uniquely identify the section to replace.
- The old_string must match exactly (including whitespace and newlines).
- Fails if old_string appears zero times or more than once.`
}

func (f *FileEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "file_path": {
      "type": "string",
      "description": "The absolute path of the file to edit."
    },
    "old_string": {
      "type": "string",
      "description": "The text to search for. Must appear exactly once."
    },
    "new_string": {
      "type": "string",
      "description": "The replacement text."
    }
  },
  "required": ["file_path", "old_string", "new_string"]
}`)
}

type fileEditInput struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (f *FileEditTool) Execute(_ context.Context, input json.RawMessage) (*Result, error) {
	var inp fileEditInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return nil, fmt.Errorf("invalid edit input: %w", err)
	}

	data, err := os.ReadFile(inp.FilePath)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error reading file: %v", err), IsError: true}, nil
	}

	content := string(data)
	count := strings.Count(content, inp.OldString)

	switch count {
	case 0:
		return &Result{
			Content: fmt.Sprintf("Error: old_string not found in %s. Make sure you are using the exact text from the file.", inp.FilePath),
			IsError: true,
		}, nil
	case 1:
		// Exactly one match — safe to replace.
	default:
		return &Result{
			Content: fmt.Sprintf("Error: old_string appears %d times in %s. Provide more context to uniquely identify the section.", count, inp.FilePath),
			IsError: true,
		}, nil
	}

	newContent := strings.Replace(content, inp.OldString, inp.NewString, 1)

	if err := os.WriteFile(inp.FilePath, []byte(newContent), 0o644); err != nil {
		return &Result{Content: fmt.Sprintf("Error writing file: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Successfully edited %s", inp.FilePath)}, nil
}
