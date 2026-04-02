// Package tools defines the Tool interface and a registry of available tools.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Result is what a tool returns after execution.
type Result struct {
	// Content is the human-readable output shown to the model.
	Content string
	// IsError indicates whether execution failed.
	IsError bool
}

// Tool is the interface that every tool must implement.
type Tool interface {
	// Name returns the tool's identifier used in API calls.
	Name() string
	// Description returns a concise description of what the tool does.
	Description() string
	// InputSchema returns the JSON Schema (as a raw byte slice) for the tool's input.
	InputSchema() json.RawMessage
	// Execute runs the tool with the provided JSON-encoded input.
	Execute(ctx context.Context, input json.RawMessage) (*Result, error)
}

// Registry holds the set of available tools and provides lookup by name.
type Registry struct {
	tools map[string]Tool
	order []string
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.  Panics on duplicate names.
func (r *Registry) Register(t Tool) {
	if _, exists := r.tools[t.Name()]; exists {
		panic(fmt.Sprintf("tool already registered: %s", t.Name()))
	}
	r.tools[t.Name()] = t
	r.order = append(r.order, t.Name())
}

// Get returns the tool with the given name, or false if not found.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All returns all registered tools in registration order.
func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// DefaultRegistry builds a registry with all built-in tools.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(NewBashTool())
	r.Register(NewFileReadTool())
	r.Register(NewFileWriteTool())
	r.Register(NewFileEditTool())
	r.Register(NewGlobTool())
	r.Register(NewGrepTool())
	r.Register(NewWebFetchTool())
	return r
}
