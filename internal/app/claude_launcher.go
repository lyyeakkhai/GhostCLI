package app

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

// ClaudeLauncher handles detecting and spawning the Claude Code CLI as a child
// process, automatically wiring ANTHROPIC_BASE_URL so Claude talks to GhostCLI
// instead of the real Anthropic API.
type ClaudeLauncher struct {
	logger   *slog.Logger
	baseURL  string
	cmd      *exec.Cmd
	launched bool
}

// NewClaudeLauncher creates a launcher that will start Claude Code pointing at
// the given base URL (e.g. "http://localhost:3200").
func NewClaudeLauncher(logger *slog.Logger, baseURL string) *ClaudeLauncher {
	return &ClaudeLauncher{
		logger:  logger,
		baseURL: baseURL,
	}
}

// Launch detects the Claude Code binary in PATH and starts it with the
// ANTHROPIC_BASE_URL environment variable set. It returns an error if Claude
// is not installed or cannot be started.
func (c *ClaudeLauncher) Launch() error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude code not found in PATH: %w\n\nInstall Claude Code from: https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview", err)
	}

	c.logger.Info("launching Claude Code", "path", claudePath, "anthropic_base_url", c.baseURL)

	// Build the command
	c.cmd = exec.Command(claudePath)

	// Inherit the current environment and override ANTHROPIC_BASE_URL
	env := os.Environ()
	env = append(env, fmt.Sprintf("ANTHROPIC_BASE_URL=%s", c.baseURL))
	c.cmd.Env = env

	// Wire stdin/stdout/stderr so Claude runs interactively in the same terminal
	c.cmd.Stdin = os.Stdin
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Claude Code: %w", err)
	}

	c.launched = true
	c.logger.Info("Claude Code started", "pid", c.cmd.Process.Pid)

	// Monitor the process in the background so we can log when it exits
	go c.wait()

	return nil
}

// wait blocks until the Claude process exits and logs the result.
func (c *ClaudeLauncher) wait() {
	err := c.cmd.Wait()
	if err != nil {
		c.logger.Warn("Claude Code exited with error", "error", err)
	} else {
		c.logger.Info("Claude Code exited cleanly")
	}
}

// Stop terminates the Claude Code process if it is still running.
func (c *ClaudeLauncher) Stop() error {
	if !c.launched || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	c.logger.Info("stopping Claude Code", "pid", c.cmd.Process.Pid)

	// Try graceful interrupt first, then force kill
	done := make(chan error, 1)
	go func() {
		// Send interrupt signal
		done <- c.cmd.Process.Signal(os.Interrupt)
	}()

	select {
	case <-done:
		// Signal sent; Wait() will eventually return
	case <-time.After(2 * time.Second):
		_ = c.cmd.Process.Kill()
	}

	return nil
}
