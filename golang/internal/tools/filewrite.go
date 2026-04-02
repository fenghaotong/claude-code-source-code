package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileWriteTool creates or overwrites a file with the provided content.
type FileWriteTool struct{}

// NewFileWriteTool creates a new FileWriteTool.
func NewFileWriteTool() *FileWriteTool { return &FileWriteTool{} }

func (f *FileWriteTool) Name() string { return "Write" }

func (f *FileWriteTool) Description() string {
	return `Writes content to a file, creating it (including any missing parent directories) or overwriting it if it already exists.

Usage notes:
- Prefer absolute paths.
- Parent directories are created automatically.
- Existing file contents are replaced entirely.`
}

func (f *FileWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "file_path": {
      "type": "string",
      "description": "The absolute path of the file to write."
    },
    "content": {
      "type": "string",
      "description": "The content to write to the file."
    }
  },
  "required": ["file_path", "content"]
}`)
}

type fileWriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (f *FileWriteTool) Execute(_ context.Context, input json.RawMessage) (*Result, error) {
	var inp fileWriteInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return nil, fmt.Errorf("invalid write input: %w", err)
	}

	dir := filepath.Dir(inp.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &Result{Content: fmt.Sprintf("Error creating directory: %v", err), IsError: true}, nil
	}

	if err := os.WriteFile(inp.FilePath, []byte(inp.Content), 0o644); err != nil {
		return &Result{Content: fmt.Sprintf("Error writing file: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Successfully wrote to %s", inp.FilePath)}, nil
}
