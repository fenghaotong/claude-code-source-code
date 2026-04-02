package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	fileReadMaxBytes    = 500_000
	fileReadMaxLines    = 2000
	lineNumberSeparator = "\t"
)

// FileReadTool reads the contents of a file.
type FileReadTool struct{}

// NewFileReadTool creates a new FileReadTool.
func NewFileReadTool() *FileReadTool { return &FileReadTool{} }

func (f *FileReadTool) Name() string { return "Read" }

func (f *FileReadTool) Description() string {
	return `Reads a file from the local filesystem. You can optionally specify a line range to read only part of the file. The output will include line numbers for each line.

Usage notes:
- Prefer absolute paths over relative paths.
- For large files, use start_line and end_line to read specific sections.
- Line numbers in the output are 1-indexed.`
}

func (f *FileReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "file_path": {
      "type": "string",
      "description": "The absolute path to the file to read."
    },
    "start_line": {
      "type": "integer",
      "description": "The line number to start reading from (1-indexed, inclusive). If omitted, starts from line 1."
    },
    "end_line": {
      "type": "integer",
      "description": "The line number to stop reading at (1-indexed, inclusive). If omitted, reads to end of file."
    }
  },
  "required": ["file_path"]
}`)
}

type fileReadInput struct {
	FilePath  string `json:"file_path"`
	StartLine *int   `json:"start_line,omitempty"`
	EndLine   *int   `json:"end_line,omitempty"`
}

func (f *FileReadTool) Execute(_ context.Context, input json.RawMessage) (*Result, error) {
	var inp fileReadInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return nil, fmt.Errorf("invalid read input: %w", err)
	}

	data, err := os.ReadFile(inp.FilePath)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error reading file: %v", err), IsError: true}, nil
	}

	content := string(data)
	if len(data) > fileReadMaxBytes {
		content = string(data[:fileReadMaxBytes]) + "\n... (file truncated, use start_line/end_line to read more)"
	}

	lines := strings.Split(content, "\n")

	startLine := 1
	endLine := len(lines)

	if inp.StartLine != nil && *inp.StartLine > 0 {
		startLine = *inp.StartLine
	}
	if inp.EndLine != nil && *inp.EndLine > 0 && *inp.EndLine < endLine {
		endLine = *inp.EndLine
	}
	if startLine > len(lines) {
		startLine = len(lines)
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	selectedLines := lines[startLine-1 : endLine]
	if len(selectedLines) > fileReadMaxLines {
		selectedLines = selectedLines[:fileReadMaxLines]
	}

	var sb strings.Builder
	for i, line := range selectedLines {
		lineNum := startLine + i
		sb.WriteString(fmt.Sprintf("%d%s%s\n", lineNum, lineNumberSeparator, line))
	}

	result := sb.String()
	if result == "" {
		result = "(empty file)"
	}

	return &Result{Content: result}, nil
}
