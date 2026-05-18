package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"ghostcli/internal/engine/protocol"
	"ghostcli/internal/engine/translator"
)

// StreamOrchestrator manages the streaming pipeline from provider to client.
// It handles context propagation, cancellation, and coordinates between
// the provider adapter and the Anthropic output formatter.
type StreamOrchestrator struct {
	logger    *slog.Logger
	formatter *translator.AnthropicOutFormatter
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
//
// The pipeline flow:
// 1. Receive UnifiedStreamEvent from provider adapter channel
// 2. Normalize token usage (inject last known counts if missing)
// 3. Convert to Anthropic SSE format via formatter
// 4. Write to HTTP response with immediate flushing
// 5. Handle context cancellation at any point
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
