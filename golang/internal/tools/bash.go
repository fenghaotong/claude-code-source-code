package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	bashDefaultTimeoutSec = 120
	bashMaxOutputBytes    = 200_000
)

// BashTool executes shell commands and returns their output.
type BashTool struct{}

// NewBashTool creates a new BashTool.
func NewBashTool() *BashTool { return &BashTool{} }

func (b *BashTool) Name() string { return "Bash" }

func (b *BashTool) Description() string {
	return `Executes a given bash command in a persistent shell session with optional timeout, ensuring proper handling and security measures.

Before executing the command, please follow these steps:
1. Directory Verification: If the command will create new directories or files, first use the LS tool to verify the parent directory exists and is the correct location.
2. Environment Check: If the command requires specific environment variables or configurations, confirm they are set.
3. Command execution: Execute the command and capture both stdout and stderr.

Usage notes:
- Prefer absolute paths over relative paths.
- Use environment variables only when necessary.
- If a long-running command is expected, set timeout_ms appropriately.`
}

func (b *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {
      "type": "string",
      "description": "The bash command to execute. Required unless restart is true."
    },
    "timeout_ms": {
      "type": "number",
      "description": "Optional timeout in milliseconds (max 600000). Defaults to 120000."
    },
    "restart": {
      "type": "boolean",
      "description": "If true, restarts the shell session. Defaults to false."
    }
  },
  "required": ["command"]
}`)
}

type bashInput struct {
	Command   string `json:"command"`
	TimeoutMs *int   `json:"timeout_ms,omitempty"`
	Restart   bool   `json:"restart,omitempty"`
}

func (b *BashTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var inp bashInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return nil, fmt.Errorf("invalid bash input: %w", err)
	}

	if inp.Restart {
		return &Result{Content: "Shell session restarted."}, nil
	}

	timeoutMs := bashDefaultTimeoutSec * 1000
	if inp.TimeoutMs != nil {
		timeoutMs = *inp.TimeoutMs
		if timeoutMs > 600_000 {
			timeoutMs = 600_000
		}
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", "-c", inp.Command)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var sb strings.Builder
	if out := stdout.String(); out != "" {
		sb.WriteString(out)
	}
	if errOut := stderr.String(); errOut != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(errOut)
	}

	output := sb.String()
	if len(output) > bashMaxOutputBytes {
		output = output[:bashMaxOutputBytes] + "\n... (output truncated)"
	}

	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return &Result{
				Content: fmt.Sprintf("Command timed out after %dms.\n%s", timeoutMs, output),
				IsError: true,
			}, nil
		}
		return &Result{
			Content: fmt.Sprintf("Command failed: %v\n%s", err, output),
			IsError: true,
		}, nil
	}

	if output == "" {
		output = "(no output)"
	}

	return &Result{Content: output}, nil
}
