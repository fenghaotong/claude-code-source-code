// Command claude is a Go implementation of the Claude Code CLI.
//
// It provides an interactive REPL for conversing with Claude using the
// Anthropic Messages API.  Available tools mirror those in the TypeScript
// reference implementation: Bash, Read, Write, Edit, Glob, Grep, WebFetch.
//
// Usage:
//
//	claude [flags] [prompt]
//
// Flags:
//
//	-m, --model string       Model to use (default from config or claude-opus-4-5)
//	-s, --system string      Custom system prompt
//	-p, --print              Print output and exit (non-interactive)
//	    --version            Print version and exit
//	    --no-tools           Disable all tools
//
// Environment:
//
//	ANTHROPIC_API_KEY  API key (required if not stored in config file)
//	CLAUDE_MODEL       Override the model
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fenghaotong/claude-code/golang/internal/api"
	appcontext "github.com/fenghaotong/claude-code/golang/internal/context"
	"github.com/fenghaotong/claude-code/golang/internal/config"
	"github.com/fenghaotong/claude-code/golang/internal/query"
	"github.com/fenghaotong/claude-code/golang/internal/tools"
)

const (
	version = "1.0.0"

	baseSystemPrompt = `You are Claude Code, an AI coding assistant. You help users with software development tasks including writing, reading, editing, and debugging code.

You have access to a set of tools that let you interact with the local filesystem and execute shell commands. Use them proactively to accomplish tasks.

Important guidelines:
- Always prefer absolute paths when working with files.
- When editing files, read the current content first so your edits are accurate.
- For tasks that require multiple steps, break them down and execute them one at a time.
- Explain what you are doing as you go.`
)

func main() {
	// --- flags ---
	var (
		modelFlag   string
		systemFlag  string
		printMode   bool
		showVersion bool
		noTools     bool
	)

	flag.StringVar(&modelFlag, "m", "", "Model to use")
	flag.StringVar(&modelFlag, "model", "", "Model to use")
	flag.StringVar(&systemFlag, "s", "", "Custom system prompt")
	flag.StringVar(&systemFlag, "system", "", "Custom system prompt")
	flag.BoolVar(&printMode, "p", false, "Print output and exit (non-interactive)")
	flag.BoolVar(&printMode, "print", false, "Print output and exit (non-interactive)")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.BoolVar(&noTools, "no-tools", false, "Disable all tools")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: claude [flags] [prompt]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVersion {
		fmt.Printf("claude-code-go %s\n", version)
		os.Exit(0)
	}

	// --- config ---
	cfg, err := config.Load()
	if err != nil {
		fatalf("Loading config: %v", err)
	}

	if modelFlag != "" {
		cfg.Model = modelFlag
	}
	if cfg.APIKey == "" {
		fatalf("API key not set. Set ANTHROPIC_API_KEY or store it in the config file.")
	}

	// --- tools registry ---
	var registry *tools.Registry
	if noTools {
		registry = tools.NewRegistry()
	} else {
		registry = tools.DefaultRegistry()
	}

	// --- API client ---
	client := api.NewClient(cfg.APIKey, api.APIBaseURL, api.APIVersion)

	// --- working directory & context ---
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	sc := appcontext.GetSystemContext(cwd)
	uc := appcontext.GetUserContext(cwd)

	systemPrompt := baseSystemPrompt
	if systemFlag != "" {
		systemPrompt = systemFlag
	}
	systemPrompt = appcontext.BuildSystemPrompt(sc, uc, systemPrompt)

	// --- run ---
	promptArgs := flag.Args()

	if printMode && len(promptArgs) > 0 {
		// Single-shot mode: run one prompt and exit.
		prompt := strings.Join(promptArgs, " ")
		messages := []api.Message{query.MessageFromText(api.RoleUser, prompt)}
		opts := query.Options{
			SystemPrompt: systemPrompt,
			Output:       os.Stdout,
		}
		if _, err := query.Run(context.Background(), client, cfg.Model, cfg.MaxTokens, messages, registry, opts); err != nil {
			fatalf("Query failed: %v", err)
		}
		return
	}

	if len(promptArgs) > 0 {
		// Inline prompt with interactive follow-ups.
		initialPrompt := strings.Join(promptArgs, " ")
		runREPL(client, cfg, systemPrompt, registry, initialPrompt)
	} else {
		// Pure interactive REPL.
		runREPL(client, cfg, systemPrompt, registry, "")
	}
}

// runREPL runs the interactive read-eval-print loop.
func runREPL(
	client *api.Client,
	cfg *config.Config,
	systemPrompt string,
	registry *tools.Registry,
	initialPrompt string,
) {
	ctx := context.Background()

	var history []api.Message
	scanner := bufio.NewScanner(os.Stdin)

	printWelcome(cfg.Model)

	// Handle an inline initial prompt before entering the loop.
	if initialPrompt != "" {
		history = processPrompt(ctx, client, cfg, systemPrompt, registry, history, initialPrompt)
	}

	for {
		fmt.Print("\n\033[1;36m> \033[0m")

		if !scanner.Scan() {
			// EOF or error — exit gracefully.
			fmt.Println()
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Built-in REPL commands.
		switch line {
		case "/exit", "/quit", "exit", "quit":
			fmt.Println("Goodbye!")
			return
		case "/clear":
			history = nil
			fmt.Println("Conversation cleared.")
			continue
		case "/history":
			printHistory(history)
			continue
		case "/help":
			printHelp()
			continue
		case "/model":
			fmt.Printf("Current model: %s\n", cfg.Model)
			continue
		}

		history = processPrompt(ctx, client, cfg, systemPrompt, registry, history, line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Input error: %v\n", err)
	}
}

// processPrompt sends a single user prompt through the query loop and returns the updated history.
func processPrompt(
	ctx context.Context,
	client *api.Client,
	cfg *config.Config,
	systemPrompt string,
	registry *tools.Registry,
	history []api.Message,
	prompt string,
) []api.Message {
	// Append user message to history.
	history = append(history, query.MessageFromText(api.RoleUser, prompt))

	opts := query.Options{
		SystemPrompt: systemPrompt,
		Output:       os.Stdout,
	}

	updated, err := query.Run(ctx, client, cfg.Model, cfg.MaxTokens, history, registry, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n\033[1;31mError: %v\033[0m\n", err)
		return history
	}

	return updated
}

// --- helpers ---

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\033[1;31merror: \033[0m"+format+"\n", args...)
	os.Exit(1)
}

func printWelcome(model string) {
	fmt.Printf("\033[1;32mClaude Code Go\033[0m (v%s) — model: \033[1m%s\033[0m\n", version, model)
	fmt.Println("Type your message and press Enter. Use /help for commands, /exit to quit.")
	fmt.Println(strings.Repeat("─", 60))
}

func printHelp() {
	fmt.Println(`Available commands:
  /clear    Clear conversation history
  /history  Show conversation history
  /model    Show current model
  /help     Show this help
  /exit     Exit the program`)
}

func printHistory(history []api.Message) {
	if len(history) == 0 {
		fmt.Println("(no history)")
		return
	}
	for i, msg := range history {
		role := string(msg.Role)
		var text string
		for _, block := range msg.Content {
			if block.Type == "text" {
				text = block.Text
				break
			}
		}
		if len(text) > 200 {
			text = text[:200] + "..."
		}
		fmt.Printf("[%d] %s: %s\n", i+1, role, text)
	}
}
