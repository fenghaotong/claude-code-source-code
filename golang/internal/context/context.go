// Package context gathers system and user context to inject into the system prompt.
package context

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxGitStatusChars = 2000
	claudeMDFilename  = "CLAUDE.md"
)

// SystemContext holds environment information injected at conversation start.
type SystemContext struct {
	GitStatus string
	Date      string
}

// UserContext holds user-configured context.
type UserContext struct {
	ClaudeMD string
	Date     string
}

// GetSystemContext collects git status and other system information.
func GetSystemContext(cwd string) *SystemContext {
	sc := &SystemContext{
		Date: time.Now().Format("2006-01-02"),
	}

	if status := getGitStatus(cwd); status != "" {
		sc.GitStatus = status
	}

	return sc
}

// GetUserContext reads CLAUDE.md files from cwd and parent directories.
func GetUserContext(cwd string) *UserContext {
	uc := &UserContext{
		Date: time.Now().Format("2006-01-02"),
	}

	if md := findClaudeMD(cwd); md != "" {
		uc.ClaudeMD = md
	}

	return uc
}

// BuildSystemPrompt assembles the full system prompt from context values.
func BuildSystemPrompt(sc *SystemContext, uc *UserContext, base string) string {
	var parts []string

	if base != "" {
		parts = append(parts, base)
	}

	if uc.ClaudeMD != "" {
		parts = append(parts, fmt.Sprintf("<claude_md>\n%s\n</claude_md>", uc.ClaudeMD))
	}

	parts = append(parts, fmt.Sprintf("Today's date is %s.", sc.Date))

	if sc.GitStatus != "" {
		parts = append(parts, sc.GitStatus)
	}

	return strings.Join(parts, "\n\n")
}

// --- helpers ---

func getGitStatus(cwd string) string {
	if !isGitRepo(cwd) {
		return ""
	}

	branch := runGit(cwd, "rev-parse", "--abbrev-ref", "HEAD")
	status := runGit(cwd, "--no-optional-locks", "status", "--short")
	log := runGit(cwd, "--no-optional-locks", "log", "--oneline", "-n", "5")
	userName := runGit(cwd, "config", "user.name")

	if len(status) > maxGitStatusChars {
		status = status[:maxGitStatusChars] + "\n... (truncated)"
	}

	var lines []string
	lines = append(lines, "This is the git status at the start of the conversation.")

	if branch != "" {
		lines = append(lines, "Current branch: "+branch)
	}
	if userName != "" {
		lines = append(lines, "Git user: "+userName)
	}
	if status == "" {
		lines = append(lines, "Status:\n(clean)")
	} else {
		lines = append(lines, "Status:\n"+status)
	}
	if log != "" {
		lines = append(lines, "Recent commits:\n"+log)
	}

	return strings.Join(lines, "\n\n")
}

func isGitRepo(cwd string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = cwd
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func runGit(cwd string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// findClaudeMD searches for CLAUDE.md starting from cwd upward to the repo root.
func findClaudeMD(cwd string) string {
	dir := cwd
	for {
		candidate := filepath.Join(dir, claudeMDFilename)
		if data, err := os.ReadFile(candidate); err == nil {
			return string(data)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
