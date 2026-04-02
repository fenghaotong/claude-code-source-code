# Claude Code — Go Implementation

A Go port of the core Claude Code agent, implementing the same architecture as the TypeScript reference implementation.

## Features

- **Interactive REPL** — Multi-turn conversation loop with history management
- **Streaming** — Server-Sent Events (SSE) streaming for real-time output
- **Tool execution** — Seven built-in tools identical in behaviour to the TypeScript versions
- **Git context** — Automatically injects `git status` into the system prompt
- **CLAUDE.md support** — Reads memory files from the working directory hierarchy
- **Config file** — Persists API key and model to a platform-appropriate config file

## Tools

| Tool | TypeScript name | Description |
|------|----------------|-------------|
| `Bash` | `BashTool` | Execute shell commands |
| `Read` | `FileReadTool` | Read file contents with line numbers |
| `Write` | `FileWriteTool` | Create or overwrite files |
| `Edit` | `FileEditTool` | Replace exact text in a file |
| `Glob` | `GlobTool` | Find files by glob pattern |
| `Grep` | `GrepTool` | Search file contents with regex |
| `WebFetch` | `WebFetchTool` | Fetch a URL |

## Directory Structure

```
golang/
├── cmd/
│   └── claude/
│       └── main.go          # CLI entrypoint & REPL
├── internal/
│   ├── api/
│   │   └── client.go        # Anthropic API client (SSE streaming)
│   ├── config/
│   │   └── config.go        # Config file + env var loading
│   ├── context/
│   │   └── context.go       # Git status & CLAUDE.md injection
│   ├── query/
│   │   └── query.go         # Core agent loop (send → tools → repeat)
│   └── tools/
│       ├── tools.go          # Tool interface & registry
│       ├── bash.go
│       ├── fileread.go
│       ├── filewrite.go
│       ├── fileedit.go
│       ├── glob.go
│       ├── grep.go
│       └── webfetch.go
├── go.mod
└── README.md
```

## Usage

### Prerequisites

- Go 1.21+
- An Anthropic API key

### Build

```bash
cd golang
go build -o claude ./cmd/claude
```

### Run

```bash
# Set your API key
export ANTHROPIC_API_KEY=sk-ant-...

# Interactive REPL
./claude

# Single-shot mode (non-interactive)
./claude -p "What files are in the current directory?"

# Inline prompt with interactive follow-ups
./claude "Help me refactor this project"

# Custom model
./claude -m claude-haiku-4-5

# Custom system prompt
./claude -s "You are a strict code reviewer."
```

### REPL Commands

| Command | Description |
|---------|-------------|
| `/clear` | Clear conversation history |
| `/history` | Show conversation history |
| `/model` | Show the active model |
| `/help` | Show available commands |
| `/exit` | Exit the program |

### Configuration

The config file is stored at:
- **Linux/other**: `$XDG_CONFIG_HOME/claude/config.json` (default `~/.config/claude/config.json`)
- **macOS**: `~/Library/Application Support/claude/config.json`
- **Windows**: `%APPDATA%\claude\config.json`

```json
{
  "api_key": "sk-ant-...",
  "model": "claude-opus-4-5"
}
```

Environment variables override file settings:
- `ANTHROPIC_API_KEY` — API key
- `CLAUDE_MODEL` — Model name

## Architecture

### Query Loop (`internal/query`)

The core loop mirrors `src/query.ts`:

1. Build a `MessagesRequest` with the current history and tools
2. Stream the response via SSE, printing text deltas in real time
3. If the model returns `tool_use` blocks, execute each tool
4. Append tool results as a `user` message and repeat
5. Stop when no tool calls remain or `MaxTurns` is reached

### API Client (`internal/api`)

Implements the Anthropic Messages API directly over HTTP without an SDK dependency:
- Sends streaming requests (`stream: true`)
- Parses SSE events: `content_block_start`, `content_block_delta`, `content_block_stop`, etc.
- Assembles the final `StreamResult` from incremental deltas

### Tool Interface (`internal/tools`)

```go
type Tool interface {
    Name()        string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, input json.RawMessage) (*Result, error)
}
```

Register custom tools with `registry.Register(myTool)`.

## Testing

```bash
go test ./...
```
