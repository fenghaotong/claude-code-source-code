package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// GlobTool finds files matching a glob pattern.
type GlobTool struct{}

// NewGlobTool creates a new GlobTool.
func NewGlobTool() *GlobTool { return &GlobTool{} }

func (g *GlobTool) Name() string { return "Glob" }

func (g *GlobTool) Description() string {
	return `Finds files matching a glob pattern. Supports standard glob wildcards:
- * matches any characters within a path segment
- ** matches any characters across multiple path segments
- ? matches a single character
- {a,b} matches either a or b

Usage notes:
- Prefer absolute paths in the path parameter.
- Results are sorted alphabetically.`
}

func (g *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "pattern": {
      "type": "string",
      "description": "The glob pattern to match files against."
    },
    "path": {
      "type": "string",
      "description": "The directory to search in. Defaults to the current working directory."
    }
  },
  "required": ["pattern"]
}`)
}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func (g *GlobTool) Execute(_ context.Context, input json.RawMessage) (*Result, error) {
	var inp globInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return nil, fmt.Errorf("invalid glob input: %w", err)
	}

	pattern := inp.Pattern
	if inp.Path != "" && !filepath.IsAbs(pattern) {
		pattern = filepath.Join(inp.Path, pattern)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Invalid glob pattern: %v", err), IsError: true}, nil
	}

	if len(matches) == 0 {
		return &Result{Content: "No files found matching the pattern."}, nil
	}

	return &Result{Content: strings.Join(matches, "\n")}, nil
}
