// Package pipeline provides streaming orchestration for the GhostCLI translation
// engine. It connects provider adapters to output formatters through a unified
// event channel, propagates context cancellation, and handles token usage
// normalization in transit.
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"ghostcli/internal/engine/protocol"
	"ghostcli/internal/engine/translator"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// Formatter is the interface satisfied by output formatters such as
// AnthropicOutFormatter. It consumes a channel of UnifiedStreamEvent objects
// and writes the translated stream to an http.ResponseWriter.
type Formatter interface {
	StreamToWriter(
		ctx context.Context,
		w http.ResponseWriter,
		eventChan <-chan protocol.UnifiedStreamEvent,
	) error
}

// Provider is the minimal interface a provider adapter must satisfy to be used
// by the pipeline. It mirrors the relevant portion of providers.Provider so
// that the pipeline package does not import the providers layer (avoiding
// circular dependencies).
type Provider interface {
	// StreamChat initiates a streaming chat request and returns a read-only
	// channel of UnifiedStreamEvent objects plus an initialisation error.
	StreamChat(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error)
}

// ---------------------------------------------------------------------------
// StreamOrchestrator — legacy concrete orchestrator (used by existing tests)
// ---------------------------------------------------------------------------

// StreamOrchestrator manages the streaming pipeline from provider to client.
// It handles context propagation, cancellation, and coordinates between
// the provider adapter and the Anthropic output formatter.
type StreamOrchestrator struct {
	logger     *slog.Logger
	formatter  *translator.AnthropicOutFormatter
	normalizer *UsageNormalizer
}

// NewStreamOrchestrator creates a new stream orchestrator instance.
func NewStreamOrchestrator(logger *slog.Logger) *StreamOrchestrator {
	return &StreamOrchestrator{
		logger:     logger,
		formatter:  translator.NewAnthropicOutFormatter(logger),
		normalizer: NewUsageNormalizer(logger),
	}
}

// StreamOptions configures the streaming behavior.
type StreamOptions struct {
	// EnableUsageNormalization ensures every event has token usage data
	EnableUsageNormalization bool

	// EstimateTokens enables token estimation when provider doesn't provide usage
	EstimateTokens bool
}

// DefaultStreamOptions returns the default streaming options.
func DefaultStreamOptions() StreamOptions {
	return StreamOptions{
		EnableUsageNormalization: true,
		EstimateTokens:           true,
	}
}

// Stream orchestrates the streaming pipeline from provider events to HTTP response.
// It propagates the request context for cancellation handling and normalizes
// token usage across events.
func (s *StreamOrchestrator) Stream(
	ctx context.Context,
	w http.ResponseWriter,
	eventChan <-chan protocol.UnifiedStreamEvent,
	opts StreamOptions,
) error {
	s.logger.Debug("starting stream orchestration")

	// Create a derived context that we can monitor
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create a normalized event channel if usage normalization is enabled
	var normalizedChan <-chan protocol.UnifiedStreamEvent
	if opts.EnableUsageNormalization {
		normalizedChan = s.normalizer.Normalize(streamCtx, eventChan, opts.EstimateTokens)
	} else {
		normalizedChan = eventChan
	}

	// Stream normalized events to the HTTP response writer
	if err := s.formatter.StreamToWriter(streamCtx, w, normalizedChan); err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			s.logger.Debug("stream cancelled by context", "error", err)
			return err
		}
		s.logger.Error("stream failed", "error", err)
		return fmt.Errorf("stream orchestration failed: %w", err)
	}

	s.logger.Debug("stream orchestration completed successfully")
	return nil
}

// StreamWithDefaults is a convenience method that uses default streaming options.
func (s *StreamOrchestrator) StreamWithDefaults(
	ctx context.Context,
	w http.ResponseWriter,
	eventChan <-chan protocol.UnifiedStreamEvent,
) error {
	return s.Stream(ctx, w, eventChan, DefaultStreamOptions())
}

// HandleCancellation monitors the context and logs cancellation events.
// This is useful for debugging and observability.
func (s *StreamOrchestrator) HandleCancellation(ctx context.Context, source string) {
	<-ctx.Done()

	switch ctx.Err() {
	case context.Canceled:
		s.logger.Info("stream cancelled", "source", source)
	case context.DeadlineExceeded:
		s.logger.Warn("stream deadline exceeded", "source", source)
	default:
		s.logger.Error("stream context error", "source", source, "error", ctx.Err())
	}
}

// ---------------------------------------------------------------------------
// StreamPipeline — interface-based pipeline (task 3.3)
// ---------------------------------------------------------------------------

// StreamPipeline orchestrates the full request → translate → stream lifecycle.
// It wires a Provider, an optional UsageNormalizer, and a Formatter together
// so that each component only knows about the pipeline's well-defined
// interfaces.
type StreamPipeline struct {
	provider   Provider
	formatter  Formatter
	normalizer *UsageNormalizer
	logger     *slog.Logger
}

// NewStreamPipeline creates a StreamPipeline that routes events from provider
// through the normalizer (if enabled) and into the formatter.
//
// Set normalizeUsage=true to wrap the raw provider channel with a
// UsageNormalizer that ensures every event carries non-zero usage data.
func NewStreamPipeline(
	provider Provider,
	formatter Formatter,
	logger *slog.Logger,
	normalizeUsage bool,
) *StreamPipeline {
	var n *UsageNormalizer
	if normalizeUsage {
		n = NewUsageNormalizer(logger)
	}
	return &StreamPipeline{
		provider:   provider,
		formatter:  formatter,
		normalizer: n,
		logger:     logger,
	}
}

// Execute runs the streaming pipeline for a single request:
//  1. Calls provider.StreamChat to obtain an upstream event channel.
//  2. Optionally wraps the channel with usage normalization.
//  3. Streams normalized events through the formatter to the response writer.
//
// Context cancellation at any stage propagates immediately: the provider
// adapter stops emitting events, the normalizer goroutine exits, and the
// formatter returns ctx.Err().
//
// Requirements: 14 (context propagation), 15 (token usage normalization),
// 24 (streaming performance – zero additional buffering added here).
func (p *StreamPipeline) Execute(
	ctx context.Context,
	w http.ResponseWriter,
	req *protocol.UnifiedChatRequest,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.logger.Debug("pipeline execute: starting StreamChat",
		"model", req.Model,
		"stream", req.Stream,
	)

	// Step 1 – open the upstream event channel.
	rawChan, err := p.provider.StreamChat(ctx, req)
	if err != nil {
		return fmt.Errorf("provider.StreamChat: %w", err)
	}

	// Step 2 – optionally normalise usage data.
	var eventChan <-chan protocol.UnifiedStreamEvent
	if p.normalizer != nil {
		eventChan = p.normalizer.Wrap(ctx, rawChan)
	} else {
		eventChan = rawChan
	}

	// Step 3 – stream to the formatter / response writer.
	if err := p.formatter.StreamToWriter(ctx, w, eventChan); err != nil {
		// Context cancellation is expected when the client disconnects; log at
		// debug level and propagate so callers can distinguish it from real errors.
		if ctx.Err() != nil {
			p.logger.Debug("pipeline execute: context cancelled", "reason", ctx.Err())
		} else {
			p.logger.Error("pipeline execute: formatter error", "error", err)
		}
		return err
	}

	p.logger.Debug("pipeline execute: completed")
	return nil
}
