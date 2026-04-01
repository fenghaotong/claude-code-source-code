# Code Structure Analysis

> Based on Claude Code v2.1.88 decompiled source code analysis.

## Overview

Claude Code is a large TypeScript/React application (~1,884 files, ~512,664 lines) built on top of the Anthropic SDK. It is compiled via **Bun** into a single 12 MB `cli.js` bundle. The source is organized into a layered architecture with clear separation between the entry layer, the query engine, the tool system, and supporting services.

---

## Top-Level Directory Map

```
src/
├── main.tsx              # Interactive REPL bootstrap (4,683 lines)
├── QueryEngine.ts        # SDK / headless query lifecycle
├── query.ts              # Core agent loop (~785 KB — largest file)
├── Tool.ts               # Tool interface + buildTool factory
├── Task.ts               # Task type, ID, and status base
├── tools.ts              # Tool registration, presets, filtering
├── commands.ts           # Slash-command definitions
├── context.ts            # User-input context type
├── cost-tracker.ts       # API cost accumulation
├── costHook.ts           # React hook for cost display
├── setup.ts              # First-run setup flow
│
├── bridge/               # Claude Desktop / remote-bridge
├── cli/                  # CLI infrastructure
├── commands/             # ~80 slash-command implementations
├── components/           # React/Ink terminal UI components
├── constants/            # Shared constants and prompts
├── context/              # React context providers
├── coordinator/          # Multi-agent coordinator
├── entrypoints/          # Application entry points
├── hooks/                # React hooks
├── keybindings/          # Terminal keybinding definitions
├── memdir/               # Memory directory (CLAUDE.md)
├── migrations/           # Schema/state migration scripts
├── native-ts/            # Native TypeScript bindings
├── outputStyles/         # Output formatting styles
├── plugins/              # Plugin loader infrastructure
├── query/                # Query utilities and helpers
├── remote/               # Remote session support
├── schemas/              # Zod schema definitions
├── screens/              # Full-screen terminal views
├── services/             # Business-logic services
├── skills/               # Skill loading utilities
├── state/                # Global application state (Zustand)
├── tasks/                # Task implementations
├── tools/                # 40+ tool implementations
├── types/                # Shared TypeScript type definitions
├── upstreamproxy/        # Upstream proxy support
├── utils/                # Utility functions (largest directory)
├── vendor/               # Native module source stubs
├── vim/                  # Vim keybinding mode
└── voice/                # Voice input support
```

---

## Layer 1 — Entry Points

### `src/entrypoints/`

```
entrypoints/
├── cli.tsx          # Main CLI entry — parses argv, selects mode
├── init.ts          # First-run initialisation
├── mcp.ts           # MCP server entry point
└── sdk/             # SDK / headless API exports
```

`cli.tsx` is the first file executed. It:
1. Parses command-line arguments
2. Detects the mode (`interactive`, `headless`, `bridge`, `mcp-server`, `sdk`)
3. Renders the appropriate screen or launches the query engine

Source: `src/entrypoints/cli.tsx`

### `src/screens/`

| Screen | Purpose |
|--------|---------|
| `REPL.tsx` | Main interactive session |
| `ResumeConversation.tsx` | Session resume UI |
| `Doctor.tsx` | `/doctor` diagnostics screen |

---

## Layer 2 — Query Engine

The query engine is the heart of Claude Code.

### `src/QueryEngine.ts`

Headless (SDK / non-interactive) query lifecycle manager. Exposes `submitMessage(prompt)` returning `AsyncGenerator<SDKMessage>`.

### `src/query.ts` (~785 KB)

The main agent loop. Key functions:

| Function | Role |
|----------|------|
| `query()` | Outer control loop — sends messages to Anthropic, handles streaming |
| `runTools()` | Orchestrates parallel tool execution after `stop_reason === "tool_use"` |
| `autoCompact()` | Triggers context-window compression when tokens are high |
| `fetchSystemPromptParts()` | Assembles the system prompt from all registered parts |
| `processUserInput()` | Detects and handles `/slash` commands |

### Agent Loop (simplified)

```
submitMessage(prompt)
  │
  ├─ fetchSystemPromptParts()    → assemble system prompt
  ├─ processUserInput()          → handle /commands
  └─ query()                     → CORE LOOP
        │
        ├─ Call Anthropic API (streaming)
        │
        ├─ stop_reason == "tool_use"?
        │     YES → StreamingToolExecutor.execute()
        │               └─ runTools() (parallel)
        │               └─ append tool_result messages
        │               └─ loop back to API call
        │
        └─ stop_reason == "end_turn"?
              YES → yield SDKMessage to consumer
```

Source: `src/query.ts`, `src/QueryEngine.ts`

---

## Layer 3 — Tool System

### `src/Tool.ts`

Defines the `Tool` interface that every tool must implement:

```typescript
interface Tool {
  name: string
  description: string
  inputSchema: ToolInputJSONSchema       // Zod-validated JSON schema
  call(input, context): Promise<ToolResult>
  renderResultForAssistant(result): string
  // Optional:
  userFacingName?(): string
  checkPermissions?(input, context): PermissionResult
  prompt?(): string                      // system-prompt contribution
}
```

Tools are registered in `src/tools.ts` and filtered per context (agent mode, user type, feature flags).

### Tool Categories

#### File System Tools

| Tool | File | Purpose |
|------|------|---------|
| `FileReadTool` | `tools/FileReadTool/` | Read files, images, notebooks |
| `FileWriteTool` | `tools/FileWriteTool/` | Write / overwrite files |
| `FileEditTool` | `tools/FileEditTool/` | Surgical in-place edits (diff-based) |
| `GlobTool` | `tools/GlobTool/` | Find files by glob pattern |
| `GrepTool` | `tools/GrepTool/` | Regex search across files |

#### Execution Tools

| Tool | File | Purpose |
|------|------|---------|
| `BashTool` | `tools/BashTool/` | Execute shell commands |
| `PowerShellTool` | `tools/PowerShellTool/` | Windows PowerShell execution |
| `NotebookEditTool` | `tools/NotebookEditTool/` | Edit Jupyter notebooks |

#### Agent / Orchestration Tools

| Tool | File | Purpose |
|------|------|---------|
| `AgentTool` | `tools/AgentTool/` | Spawn sub-agents (Task tool) |
| `SkillTool` | `tools/SkillTool/` | Load and run skill scripts |
| `TaskCreateTool` | `tools/TaskCreateTool/` | Create background tasks |
| `TaskGetTool` | `tools/TaskGetTool/` | Get task status |
| `TaskUpdateTool` | `tools/TaskUpdateTool/` | Update task state |
| `TaskListTool` | `tools/TaskListTool/` | List active tasks |
| `TaskStopTool` | `tools/TaskStopTool/` | Stop a running task |
| `TaskOutputTool` | `tools/TaskOutputTool/` | Read task output |
| `SendMessageTool` | `tools/SendMessageTool/` | Send message to peer agent |

#### Web / Search Tools

| Tool | File | Purpose |
|------|------|---------|
| `WebFetchTool` | `tools/WebFetchTool/` | Fetch web pages |
| `WebSearchTool` | `tools/WebSearchTool/` | Internet search |

#### MCP Tools

| Tool | File | Purpose |
|------|------|---------|
| `MCPTool` | `tools/MCPTool/` | Proxy to MCP server tools |
| `ListMcpResourcesTool` | `tools/ListMcpResourcesTool/` | List MCP resources |
| `ReadMcpResourceTool` | `tools/ReadMcpResourceTool/` | Read MCP resource |
| `McpAuthTool` | `tools/McpAuthTool/` | MCP OAuth flow |

#### UI / Planning Tools

| Tool | File | Purpose |
|------|------|---------|
| `TodoWriteTool` | `tools/TodoWriteTool/` | Write structured TODO lists |
| `EnterPlanModeTool` | `tools/EnterPlanModeTool/` | Switch to plan mode |
| `ExitPlanModeTool` | `tools/ExitPlanModeTool/` | Exit plan mode |
| `AskUserQuestionTool` | `tools/AskUserQuestionTool/` | Interactive user question |
| `ConfigTool` | `tools/ConfigTool/` | Read/write configuration |
| `LSPTool` | `tools/LSPTool/` | Language server protocol queries |

#### Worktree Tools

| Tool | File | Purpose |
|------|------|---------|
| `EnterWorktreeTool` | `tools/EnterWorktreeTool/` | Switch to a git worktree |
| `ExitWorktreeTool` | `tools/ExitWorktreeTool/` | Exit a git worktree |

#### Internal / Feature-Gated Tools (not in public bundle)

| Tool | Feature Gate |
|------|-------------|
| `REPLTool` | `USER_TYPE === 'ant'` |
| `SleepTool` | `PROACTIVE` or `KAIROS` |
| `MonitorTool` | `MONITOR_TOOL` |
| `VerifyPlanExecutionTool` | `CLAUDE_CODE_VERIFY_PLAN` |
| `SendUserFileTool` | `KAIROS` |
| `PushNotificationTool` | `KAIROS` or `KAIROS_PUSH_NOTIFICATION` |
| `SubscribePRTool` | `KAIROS_GITHUB_WEBHOOKS` |
| `SuggestBackgroundPRTool` | `USER_TYPE === 'ant'` |
| `TungstenTool` | Internal (compiled in but unlisted) |

Source: `src/tools.ts`

---

## Layer 4 — Services

### `src/services/`

```
services/
├── analytics/            # Telemetry pipeline
│   ├── index.ts
│   ├── metadata.ts       # Event metadata assembly (~500 lines)
│   ├── firstPartyEventLogger.ts
│   ├── firstPartyEventLoggingExporter.ts
│   ├── datadog.ts        # Datadog sink (64 event types)
│   └── growthbook.ts     # Feature flag / A-B test client
│
├── api/                  # Anthropic API client wrappers
│   ├── client.ts         # SDK client factory
│   ├── claude.ts         # Primary completions wrapper
│   ├── withRetry.ts      # Retry + backoff logic
│   ├── errors.ts         # API error types
│   └── ...
│
├── compact/              # Context-window compression
│   ├── autoCompact.ts    # Trigger logic
│   ├── compact.ts        # Compaction algorithms
│   ├── microCompact.ts   # Micro-compaction strategy
│   └── ...
│
├── mcp/                  # Model Context Protocol
│   ├── MCPConnectionManager.tsx
│   ├── client.ts         # MCP client
│   ├── config.ts         # MCP server configuration
│   ├── auth.ts           # MCP OAuth
│   └── types.ts
│
├── remoteManagedSettings/ # Remote killswitch polling
├── settingsSync/          # Settings synchronisation
├── oauth/                 # OAuth flows (Claude.ai, AWS)
├── lsp/                   # Language Server Protocol support
├── tools/                 # Tool execution infrastructure
│   ├── StreamingToolExecutor.ts  # Parallel streaming execution
│   └── toolOrchestration.ts     # runTools() implementation
│
├── SessionMemory/         # Per-session memory
├── extractMemories/       # Memory extraction pipeline
├── AgentSummary/          # Agent conversation summariser
├── MagicDocs/             # Magic documentation generation
├── PromptSuggestion/      # Prompt suggestion engine
├── compact/               # Context compact service
├── toolUseSummary/        # Tool-use summary generation
└── voice.ts               # Voice input/output
```

### Key Service: `StreamingToolExecutor`

Executes multiple tools in parallel while streaming their intermediate output back to the UI.

Source: `src/services/tools/StreamingToolExecutor.ts`

### Key Service: Compact Pipeline

When the context window approaches its limit, the compact pipeline:
1. Generates a summary of the conversation so far (via `AgentSummary`)
2. Replaces old messages with the summary
3. Optionally stores a micro-compact boundary marker

Source: `src/services/compact/`

---

## Layer 5 — State Management

### `src/state/`

```
state/
├── AppState.tsx          # Global application state shape
├── AppStateStore.ts      # Zustand store factory
├── store.ts              # Store singleton
├── selectors.ts          # Memoised selectors
├── onChangeAppState.ts   # Side-effect handlers on state change
└── teammateViewHelpers.ts
```

State is managed with **Zustand**. The `AppState` includes:
- Current conversation messages
- Active tools and their progress
- Permission mode and denials tracking
- Cost / token usage
- UI state (plan mode, worktree, etc.)

Source: `src/state/AppState.tsx`

---

## Layer 6 — UI (React/Ink)

Claude Code's terminal UI is built with **Ink** (React for the terminal).

### `src/components/`

Selected key components:

| Component | Purpose |
|-----------|---------|
| `App.tsx` | Root application component |
| `REPL.tsx` (screen) | Main conversation UI |
| `Spinner.tsx` | Streaming / loading indicator |
| `ToolProgressLine.tsx` | Per-tool progress display |
| `CompactSummary.tsx` | Shows compaction events |
| `AutoUpdater.tsx` | Auto-update notification overlay |
| `BridgeDialog.tsx` | Claude Desktop bridge dialog |

### `src/context/`

React context providers:

| Context | Purpose |
|---------|---------|
| `mailbox.tsx` | Inter-component message bus |
| `modalContext.tsx` | Modal overlay management |
| `notifications.tsx` | In-app notification queue |
| `voice.tsx` | Voice session state |

---

## Layer 7 — Commands (~80 Slash Commands)

### `src/commands/`

Each subdirectory is a slash command. Selected examples:

```
commands/
├── add-dir/          # /add-dir — add working directory
├── compact/          # /compact — manual compaction
├── config/           # /config — configuration management
├── context/          # /context — show context usage
├── cost/             # /cost — show API cost
├── debug-tool-call/  # /debug-tool-call
├── diff/             # /diff — show diff
├── doctor/           # /doctor — diagnostics
├── export/           # /export — export conversation
├── feedback/         # /feedback — send feedback
├── files/            # /files — list project files
├── help/             # /help
├── memory/           # /memory — CLAUDE.md management
├── model/            # /model — switch model
├── permissions/      # /permissions — permission management
├── pr/               # /pr — pull request commands
├── review/           # /review — code review
├── terminal-setup/   # /terminal-setup
├── vim/              # /vim — vim mode
└── ...               # 60+ more
```

---

## Layer 8 — Utilities (`src/utils/`)

The `utils/` directory is the largest in the codebase, containing ~200+ files. Major categories:

### Authentication & API

| File | Purpose |
|------|---------|
| `auth.ts` | API key management |
| `authPortable.ts` | Portable auth (keyring / env) |
| `aws.ts` | AWS Bedrock credential helpers |
| `api.ts` | API request helpers |
| `billing.ts` | Billing status helpers |

### Message Processing

| File | Purpose |
|------|---------|
| `messages.ts` | Message creation, normalisation, truncation |
| `tokens.ts` | Token counting and estimation |
| `toolResultStorage.ts` | Large tool result caching (>4 KB stored to disk) |
| `attachments.ts` | Memory attachment management |
| `messageQueueManager.ts` | Slash-command queue |

### Permissions

| File | Purpose |
|------|---------|
| `permissions/` | Permission evaluation engine |
| `permissions/denialTracking.ts` | Track and learn from denials |
| `autoModeDenials.ts` | Auto-approve rule engine |

### Shell / Process

| File | Purpose |
|------|---------|
| `bash/` | Shell command helpers |
| `Shell.ts` | Shell abstraction |
| `ShellCommand.ts` | Shell command runner |

### Model Utilities

| File | Purpose |
|------|---------|
| `model/model.ts` | Model selection and name rendering |
| `thinking.ts` | Extended thinking configuration |
| `context.ts` | Context window size lookup |

### Git / VCS

| File | Purpose |
|------|---------|
| `attribution.ts` | Commit attribution |
| `commitAttribution.ts` | Advanced commit attribution |
| `fileHistory.ts` | File change tracking |

---

## Bridge Mode (`src/bridge/`)

Bridge mode connects Claude Code to **Claude Desktop** or a remote Claude.ai session.

```
bridge/
├── bridgeMain.ts       # Session lifecycle manager
├── bridgeApi.ts        # HTTP client to Claude.ai
├── bridgeConfig.ts     # Connection configuration
├── bridgeMessaging.ts  # Message relay layer
├── sessionRunner.ts    # Process spawning
├── jwtUtils.ts         # JWT refresh
├── workSecret.ts       # Auth token management
└── capacityWake.ts     # Capacity-based wakeup trigger
```

When bridge mode is active, Claude Code acts as a local execution environment while the UI lives in Claude Desktop or the web browser.

Source: `src/bridge/`

---

## Memory System (`src/memdir/`)

Claude Code uses a multi-layer memory system:

| Layer | Storage | Purpose |
|-------|---------|---------|
| **CLAUDE.md files** | On-disk per project | Long-term project memory |
| **Session memory** | In-process | Short-term conversation context |
| **Memory extraction** | Async post-conversation | Distilled facts saved to CLAUDE.md |
| **Remote team memory** | Cloud-synced | Shared across team members |

The `memdir` module scans the project directory tree and parent directories for `CLAUDE.md` files, composing them into the system prompt.

Source: `src/memdir/`, `src/services/SessionMemory/`, `src/services/extractMemories/`

---

## MCP Integration (`src/services/mcp/`)

Claude Code acts as an MCP **client**, connecting to external MCP servers to extend its tool set.

```
Connection flow:
  User config (CLAUDE.md / settings.json)
    → MCPConnectionManager
    → Transport (stdio / HTTP / SSE / in-process)
    → Tool discovery
    → MCPTool registration
    → Available to agent loop
```

Authentication: OAuth 2.0 PKCE flow via `src/services/mcp/auth.ts`.

Source: `src/services/mcp/`

---

## Permission System

Permissions gate every destructive tool action. Architecture:

```
ToolCall
  │
  ├─ checkPermissions() on Tool
  │     ├─ is it read-only? → allow
  │     ├─ is it in auto-approve list? → allow
  │     ├─ is bypass mode active? → allow
  │     └─ else → prompt user
  │
  └─ DenialTracker records outcomes
        └─ feeds back into auto-approval rules
```

Permission modes: `default`, `acceptEdits`, `bypassPermissions` (YOLO mode).

Source: `src/types/permissions.ts`, `src/utils/permissions/`

---

## Build System

| File | Purpose |
|------|---------|
| `scripts/build.mjs` | Main build script |
| `scripts/prepare-src.mjs` | Source preparation |
| `scripts/excluded-strings.txt` | Canary string scanner (codename leak detection) |
| `tsconfig.json` | TypeScript compiler options |
| `stubs/` | Module stubs for missing internal modules |
| `vendor/` | Native module source stubs |

The build requires **Bun** as both the bundler and `feature()` macro resolver. The npm-published artifact is a single `cli.js` (~12 MB).

---

## Dependency Overview

Key runtime dependencies (from the decompiled bundle):

| Package | Role |
|---------|------|
| `@anthropic-ai/sdk` | Official Anthropic API client |
| `@modelcontextprotocol/sdk` | MCP client SDK |
| `react` + `ink` | Terminal UI framework |
| `zustand` | State management |
| `zod` | Runtime schema validation |
| `lodash-es` | Utility functions |
| `glob` | File pattern matching |
| `ripgrep` (bundled) | Fast file search |
| `opentelemetry` | Telemetry export pipeline |

---

## Summary

Claude Code's architecture follows a clean separation of concerns:

1. **Entry Layer** (`entrypoints/`, `main.tsx`) — boots the appropriate mode
2. **Query Engine** (`query.ts`, `QueryEngine.ts`) — runs the agent loop
3. **Tool System** (`Tool.ts`, `tools/`, `tools.ts`) — 40+ modular tools
4. **Services** (`services/`) — analytics, compaction, MCP, API, OAuth
5. **State** (`state/`) — Zustand global store
6. **UI** (`components/`, `screens/`, `hooks/`) — React/Ink terminal UI
7. **Commands** (`commands/`) — ~80 slash-command handlers
8. **Utilities** (`utils/`) — shared helpers for auth, messages, permissions, shell

The largest bottleneck for understanding is `query.ts` (~785 KB), which contains the complete agent loop and accounts for nearly all of the core agent behaviour.
