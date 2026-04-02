// Package config manages configuration for the Claude Code Go client.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	DefaultModel     = "claude-opus-4-5"
	DefaultMaxTokens = 8192
	APIBaseURL       = "https://api.anthropic.com"
	APIVersion       = "2023-06-01"
)

// Config holds the configuration for the Claude Code client.
type Config struct {
	APIKey    string `json:"api_key,omitempty"`
	Model     string `json:"model,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

// configFile is the on-disk representation.
type configFile struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// Load reads the configuration from environment variables and the config file.
// Environment variables take precedence over the config file.
func Load() (*Config, error) {
	cfg := &Config{
		Model:     DefaultModel,
		MaxTokens: DefaultMaxTokens,
	}

	// Load from config file first.
	if err := cfg.loadFromFile(); err != nil {
		// Config file is optional — only fail on unexpected errors.
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	// Environment variables override file settings.
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		cfg.APIKey = key
	}
	if model := os.Getenv("CLAUDE_MODEL"); model != "" {
		cfg.Model = model
	}

	return cfg, nil
}

// loadFromFile reads config from the platform-appropriate config directory.
func (c *Config) loadFromFile() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var f configFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	if f.APIKey != "" {
		c.APIKey = f.APIKey
	}
	if f.Model != "" {
		c.Model = f.Model
	}
	return nil
}

// Save writes the configuration to the config file.
func (c *Config) Save() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	f := configFile{APIKey: c.APIKey, Model: c.Model}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}

// configFilePath returns the platform-appropriate config file path.
func configFilePath() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			return "", fmt.Errorf("APPDATA not set")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(home, "Library", "Application Support")
	default:
		// Linux / other UNIX
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			baseDir = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(home, ".config")
		}
	}

	return filepath.Join(baseDir, "claude", "config.json"), nil
}
