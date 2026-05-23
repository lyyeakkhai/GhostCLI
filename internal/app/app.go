// Package app provides the application orchestration layer for GhostCLI.
// It acts as a dependency-injection container, wiring together the configuration,
// telemetry, provider adapters, translation engine, and HTTP transport layers.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"ghostcli/internal/api"
	"ghostcli/internal/config"
	"ghostcli/internal/engine/translator"
	"ghostcli/internal/providers"
	"ghostcli/internal/telemetry"
)

// version is set at build time via ldflags.
var version = "dev"

// App is the top-level application container. It holds references to all
// major components and coordinates their lifecycle.
type App struct {
	Config         *config.Config
	Logger         *slog.Logger
	Provider       providers.Provider
	Registry       *providers.Registry
	Server         *api.Server
	Metrics        *telemetry.MetricsCollector
	Version        string
	ClaudeLauncher *ClaudeLauncher
	LaunchClaude   bool
}

// ProxyEngine implements the api.Engine interface by bridging the HTTP
// transport layer to the translation engine and provider adapter.
type ProxyEngine struct {
	parser    *translator.AnthropicInParser
	provider  providers.Provider
	logger    *slog.Logger
	metrics   *telemetry.MetricsCollector
	providerName string
	version   string
}

// NewProxyEngine creates a ProxyEngine that satisfies api.Engine.
func NewProxyEngine(
	parser *translator.AnthropicInParser,
	provider providers.Provider,
	logger *slog.Logger,
	metrics *telemetry.MetricsCollector,
	providerName string,
	version string,
) *ProxyEngine {
	return &ProxyEngine{
		parser:       parser,
		provider:     provider,
		logger:       logger,
		metrics:      metrics,
		providerName: providerName,
		version:      version,
	}
}

// ParseRequest decodes an incoming Anthropic Messages API request into the
// unified internal format.
func (e *ProxyEngine) ParseRequest(r *http.Request) (interface{}, error) {
	return e.parser.Parse(r.Body)
}

// StreamRequest parses the incoming request, initiates a provider stream, and
// writes Anthropic-compatible SSE events to the response writer.
func (e *ProxyEngine) StreamRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	timer := e.metrics.StartRequestTimer()
	defer timer.Stop(true) // updated on error path below

	// Parse request body
	req, err := e.parser.Parse(r.Body)
	if err != nil {
		timer.Stop(false)
		e.logger.Error("failed to parse request", "error", err)
		return fmt.Errorf("parse request: %w", err)
	}

	e.logger.Debug("streaming request",
		"model", req.Model,
		"provider", e.providerName,
	)

	// Map model name to provider-specific model
	req.Model = e.provider.MapModel(req.Model)

	// Initiate provider stream
	eventChan, err := e.provider.StreamChat(ctx, req)
	if err != nil {
		timer.Stop(false)
		e.logger.Error("provider stream failed", "error", err)
		return fmt.Errorf("provider stream: %w", err)
	}

	// Stream events through the Anthropic formatter
	formatter := translator.NewAnthropicOutFormatter(e.logger)
	if err := formatter.StreamToWriter(ctx, w, eventChan); err != nil {
		timer.Stop(false)
		e.logger.Error("formatter stream failed", "error", err)
		return fmt.Errorf("formatter stream: %w", err)
	}

	return nil
}

// ProviderName returns the active provider identifier.
func (e *ProxyEngine) ProviderName() string {
	return e.providerName
}

// Version returns the application version.
func (e *ProxyEngine) Version() string {
	return e.version
}

// NewApp creates an App by loading configuration, creating all components,
// and wiring them together.
func NewApp(cfg *config.Config, logger *slog.Logger) (*App, error) {
	return NewAppWithClaude(cfg, logger, false)
}

// NewAppWithClaude creates an App and optionally prepares Claude Code auto-launch.
func NewAppWithClaude(cfg *config.Config, logger *slog.Logger, launchClaude bool) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if logger == nil {
		logger = telemetry.Default()
	}

	// Metrics collector
	metrics := telemetry.NewMetricsCollector()

	// Provider factory
	factory := providers.NewFactory(cfg, logger)

	// Create the active provider
	provider, err := factory.CreateActiveProvider()
	if err != nil {
		return nil, fmt.Errorf("create active provider: %w", err)
	}

	// Provider registry (for future multi-provider support)
	registry := providers.NewRegistry()
	_ = registry.Register(cfg.ActiveProvider, provider) // safe: we just created it

	// Translation engine
	parser := translator.NewAnthropicInParser(logger)

	// HTTP engine adapter
	engine := NewProxyEngine(
		parser,
		provider,
		logger,
		metrics,
		cfg.ActiveProvider,
		version,
	)

	// HTTP server
	port := fmt.Sprintf("%d", cfg.Port)
	server := api.NewServer(port, engine, logger)

	app := &App{
		Config:       cfg,
		Logger:       logger,
		Provider:     provider,
		Registry:     registry,
		Server:       server,
		Metrics:      metrics,
		Version:      version,
		LaunchClaude: launchClaude,
	}

	// Prepare Claude launcher if requested
	if launchClaude {
		baseURL := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
		app.ClaudeLauncher = NewClaudeLauncher(logger, baseURL)
	}

	return app, nil
}

// ProviderName returns the name of the currently active provider.
func (a *App) ProviderName() string {
	if a.Provider == nil {
		return "unknown"
	}
	return a.Provider.Name()
}

// GetMetricsSnapshot returns a point-in-time snapshot of performance metrics.
func (a *App) GetMetricsSnapshot() telemetry.MetricsSnapshot {
	return a.Metrics.Snapshot()
}
