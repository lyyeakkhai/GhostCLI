package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// runWithClaude starts the server and then launches Claude Code. It waits for
// either a shutdown signal, a server error, or Claude Code exiting.
func (a *App) runWithClaude() error {
	if a.Server == nil {
		return fmt.Errorf("server not initialized")
	}

	// Channel to capture OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Channel to capture server errors
	errChan := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		a.Logger.Info("starting GhostCLI proxy",
			"version", a.Version,
			"provider", a.ProviderName(),
			"addr", a.Server.Addr(),
		)
		errChan <- a.Server.Start()
	}()

	// Give the server a moment to start listening
	time.Sleep(100 * time.Millisecond)

	// Launch Claude Code pointing at the proxy
	if a.ClaudeLauncher != nil {
		if err := a.ClaudeLauncher.Launch(); err != nil {
			a.Logger.Error("failed to launch Claude Code", "error", err)
			// Continue running the proxy even if Claude fails to launch
		}
	}

	// Wait for either a signal, a server error, or Claude exiting
	select {
	case sig := <-sigChan:
		a.Logger.Info("shutdown signal received", "signal", sig.String())
		return a.gracefulShutdownWithClaude()

	case err := <-errChan:
		if err == http.ErrServerClosed {
			a.Logger.Info("server shut down gracefully")
			_ = a.stopClaude()
			return nil
		}
		if err != nil {
			a.Logger.Error("server exited with error", "error", err)
			_ = a.stopClaude()
			return fmt.Errorf("server error: %w", err)
		}
		a.Logger.Info("server exited cleanly")
		_ = a.stopClaude()
		return nil
	}
}

// stopClaude stops the Claude Code process if it was launched.
func (a *App) stopClaude() error {
	if a.ClaudeLauncher != nil {
		return a.ClaudeLauncher.Stop()
	}
	return nil
}

// gracefulShutdownWithClaude stops both the server and Claude Code.
func (a *App) gracefulShutdownWithClaude() error {
	_ = a.stopClaude()
	return a.gracefulShutdown()
}

// Run starts the application and blocks until a shutdown signal is received
// or the server exits unexpectedly. It performs graceful shutdown on SIGINT
// or SIGTERM, giving active connections up to the configured timeout to finish.
// If LaunchClaude is true, Claude Code is spawned after the server starts.
func (a *App) Run() error {
	if a.LaunchClaude {
		return a.runWithClaude()
	}
	return a.runProxyOnly()
}

// runProxyOnly starts just the proxy server without Claude Code.
func (a *App) runProxyOnly() error {
	if a.Server == nil {
		return fmt.Errorf("server not initialized")
	}

	// Channel to capture OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Channel to capture server errors
	errChan := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		a.Logger.Info("starting GhostCLI proxy",
			"version", a.Version,
			"provider", a.ProviderName(),
			"addr", a.Server.Addr(),
		)
		errChan <- a.Server.Start()
	}()

	// Wait for either a signal or a server error
	select {
	case sig := <-sigChan:
		a.Logger.Info("shutdown signal received", "signal", sig.String())
		return a.gracefulShutdown()

	case err := <-errChan:
		if err == http.ErrServerClosed {
			a.Logger.Info("server shut down gracefully")
			return nil
		}
		if err != nil {
			a.Logger.Error("server exited with error", "error", err)
			return fmt.Errorf("server error: %w", err)
		}
		a.Logger.Info("server exited cleanly")
		return nil
	}
}

// gracefulShutdown stops the HTTP server gracefully, waiting up to the
// configured timeout for active connections to finish.
func (a *App) gracefulShutdown() error {
	timeout := time.Duration(a.Config.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	a.Logger.Info("initiating graceful shutdown", "timeout", timeout.String())

	if err := a.Server.Shutdown(ctx); err != nil {
		a.Logger.Error("graceful shutdown failed", "error", err)
		return fmt.Errorf("shutdown: %w", err)
	}

	a.Logger.Info("graceful shutdown complete")
	return nil
}

// Shutdown performs an immediate graceful shutdown with the given context.
// This is useful for programmatic shutdown (e.g., in tests).
func (a *App) Shutdown(ctx context.Context) error {
	if a.Server == nil {
		return fmt.Errorf("server not initialized")
	}

	a.Logger.Info("shutdown requested programmatically")
	return a.Server.Shutdown(ctx)
}
