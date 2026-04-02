package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fenghaotong/claude-code/golang/internal/tools"
)

// --- BashTool ---

func TestBashTool_SimpleCommand(t *testing.T) {
	tool := tools.NewBashTool()
	input, _ := json.Marshal(map[string]any{"command": "echo hello"})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}
	if result.Content != "hello\n" && result.Content != "hello" {
		t.Errorf("expected 'hello', got %q", result.Content)
	}
}

func TestBashTool_FailingCommand(t *testing.T) {
	tool := tools.NewBashTool()
	input, _ := json.Marshal(map[string]any{"command": "exit 1"})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for failing command")
	}
}

func TestBashTool_Restart(t *testing.T) {
	tool := tools.NewBashTool()
	input, _ := json.Marshal(map[string]any{"command": "echo hi", "restart": true})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}
}

// --- FileReadTool ---

func TestFileReadTool_ReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := tools.NewFileReadTool()
	input, _ := json.Marshal(map[string]any{"file_path": path})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}
	// Should contain line numbers.
	if len(result.Content) == 0 {
		t.Error("expected non-empty content")
	}
}

func TestFileReadTool_NotFound(t *testing.T) {
	tool := tools.NewFileReadTool()
	input, _ := json.Marshal(map[string]any{"file_path": "/nonexistent/file.txt"})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing file")
	}
}

// --- FileWriteTool ---

func TestFileWriteTool_Write(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	tool := tools.NewFileWriteTool()
	input, _ := json.Marshal(map[string]any{"file_path": path, "content": "hello world"})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

// --- FileEditTool ---

func TestFileEditTool_Edit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := tools.NewFileEditTool()
	input, _ := json.Marshal(map[string]any{
		"file_path":  path,
		"old_string": "world",
		"new_string": "Go",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello Go\n" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestFileEditTool_NotFound(t *testing.T) {
	tool := tools.NewFileEditTool()
	input, _ := json.Marshal(map[string]any{
		"file_path":  "/tmp/nonexistent_xyz.txt",
		"old_string": "foo",
		"new_string": "bar",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing file")
	}
}

func TestFileEditTool_Duplicate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dup.txt")
	if err := os.WriteFile(path, []byte("foo foo foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := tools.NewFileEditTool()
	input, _ := json.Marshal(map[string]any{
		"file_path":  path,
		"old_string": "foo",
		"new_string": "bar",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for duplicate old_string")
	}
}

// --- GlobTool ---

func TestGlobTool_FindFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := tools.NewGlobTool()
	input, _ := json.Marshal(map[string]any{
		"pattern": filepath.Join(dir, "*.go"),
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	// Should find two .go files.
	if result.Content == "" {
		t.Error("expected non-empty results")
	}
}

// --- GrepTool ---

func TestGrepTool_FindPattern(t *testing.T) {
	dir := t.TempDir()
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := tools.NewGrepTool()
	input, _ := json.Marshal(map[string]any{
		"pattern": "func main",
		"path":    dir,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if result.Content == "" || result.Content == "No matches found." {
		t.Error("expected to find 'func main'")
	}
}

// --- Registry ---

func TestRegistry_DefaultTools(t *testing.T) {
	reg := tools.DefaultRegistry()
	expectedTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebFetch"}

	for _, name := range expectedTools {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("expected tool %q in default registry", name)
		}
	}

	if len(reg.All()) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(reg.All()))
	}
}
