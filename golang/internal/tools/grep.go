package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const grepMaxResults = 1000

// GrepTool searches file contents using regular expressions.
type GrepTool struct{}

// NewGrepTool creates a new GrepTool.
func NewGrepTool() *GrepTool { return &GrepTool{} }

func (g *GrepTool) Name() string { return "Grep" }

func (g *GrepTool) Description() string {
	return `Searches file contents using a regular expression pattern.

Usage notes:
- pattern is a Go regular expression.
- Provide path to limit the search to a directory or file.
- Use include to filter by file extension (e.g. "*.go").
- Results include file path and line number.`
}

func (g *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "pattern": {
      "type": "string",
      "description": "The regular expression pattern to search for."
    },
    "path": {
      "type": "string",
      "description": "File or directory to search. Defaults to current directory."
    },
    "include": {
      "type": "string",
      "description": "Glob pattern to filter files (e.g. \"*.go\")."
    },
    "case_insensitive": {
      "type": "boolean",
      "description": "Whether to perform a case-insensitive search. Defaults to false."
    }
  },
  "required": ["pattern"]
}`)
}

type grepInput struct {
	Pattern         string `json:"pattern"`
	Path            string `json:"path,omitempty"`
	Include         string `json:"include,omitempty"`
	CaseInsensitive bool   `json:"case_insensitive,omitempty"`
}

func (g *GrepTool) Execute(_ context.Context, input json.RawMessage) (*Result, error) {
	var inp grepInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return nil, fmt.Errorf("invalid grep input: %w", err)
	}

	pattern := inp.Pattern
	if inp.CaseInsensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Invalid pattern: %v", err), IsError: true}, nil
	}

	searchPath := inp.Path
	if searchPath == "" {
		searchPath = "."
	}

	var results []string
	err = filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if d.IsDir() {
			// Skip hidden directories (e.g. .git).
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Filter by include pattern if specified.
		if inp.Include != "" {
			matched, err := filepath.Match(inp.Include, d.Name())
			if err != nil || !matched {
				return nil
			}
		}

		// Use an anonymous function so the file is closed as soon as we
		// finish scanning it, rather than deferring until the walk ends.
		if err := func() error {
			file, err := os.Open(path)
			if err != nil {
				return nil //nolint:nilerr // skip inaccessible files
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := scanner.Text()
				if re.MatchString(line) {
					results = append(results, fmt.Sprintf("%s:%d: %s", path, lineNum, line))
					if len(results) >= grepMaxResults {
						return fmt.Errorf("limit reached")
					}
				}
			}
			return nil
		}(); err != nil {
			return err
		}
		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		return &Result{Content: fmt.Sprintf("Search error: %v", err), IsError: true}, nil
	}

	if len(results) == 0 {
		return &Result{Content: "No matches found."}, nil
	}

	output := strings.Join(results, "\n")
	if len(results) >= grepMaxResults {
		output += fmt.Sprintf("\n... (results truncated at %d matches)", grepMaxResults)
	}

	return &Result{Content: output}, nil
}
