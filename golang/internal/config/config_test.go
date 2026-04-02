package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fenghaotong/claude-code/golang/internal/config"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear env vars that might be set in CI.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_MODEL", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Model != config.DefaultModel {
		t.Errorf("expected model %q, got %q", config.DefaultModel, cfg.Model)
	}
	if cfg.MaxTokens != config.DefaultMaxTokens {
		t.Errorf("expected max_tokens %d, got %d", config.DefaultMaxTokens, cfg.MaxTokens)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("CLAUDE_MODEL", "claude-haiku-3-5")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.APIKey != "test-key" {
		t.Errorf("expected APIKey 'test-key', got %q", cfg.APIKey)
	}
	if cfg.Model != "claude-haiku-3-5" {
		t.Errorf("expected model 'claude-haiku-3-5', got %q", cfg.Model)
	}
}

func TestSave_RoundTrip(t *testing.T) {
	// Override the config dir via XDG_CONFIG_HOME so we write to a temp dir.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_MODEL", "")

	cfg := &config.Config{
		APIKey:    "my-api-key",
		Model:     "claude-opus-4",
		MaxTokens: config.DefaultMaxTokens,
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify the file was created.
	configPath := filepath.Join(dir, "claude", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Load again and check values.
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}

	if loaded.APIKey != "my-api-key" {
		t.Errorf("expected APIKey 'my-api-key', got %q", loaded.APIKey)
	}
	if loaded.Model != "claude-opus-4" {
		t.Errorf("expected model 'claude-opus-4', got %q", loaded.Model)
	}
}
