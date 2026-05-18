package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"ghostcli/internal/engine/protocol"
)

// UsageNormalizer ensures consistent token usage information across all streaming events.
// It tracks token counts and injects them into events that lack usage data.
type UsageNormalizer struct {
	logger *slog.Logger
}

// NewUsageNormalizer creates a new usage normalizer instance.
func NewUsageNormalizer(logger *slog.Logger) *UsageNormalizer {
	return &UsageNormalizer{
		logger: logger,
	}
}

// Normalize processes a stream of UnifiedStreamEvent and ensures every event
// has token usage information. It maintains running counts and injects the
// last known values into events that don't provide usage data.
//
// If estimateTokens is true, it will estimate token counts based on content
// length when no usage data is available from the provider.
func (n *UsageNormalizer) Normalize(
	ctx context.Context,
	eventChan <-chan protocol.UnifiedStreamEvent,
	estimateTokens bool,
) <-chan protocol.UnifiedStreamEvent {
	normalizedChan := make(chan protocol.UnifiedStreamEvent, 10)

	go func() {
		defer close(normalizedChan)

		// Track running token counts
		var lastInputTokens int
		var lastOutputTokens int
		var totalContentLength int

		for {
			select {
			case <-ctx.Done():
				n.logger.Debug("usage normalization cancelled by context")
				return

			case event, ok := <-eventChan:
				if !ok {
					// Input channel closed
					n.logger.Debug("usage normalization completed",
						"final_input_tokens", lastInputTokens,
						"final_output_tokens", lastOutputTokens)
					return
				}

				// Update running counts if event has usage data
				if event.Usage != nil {
					if event.Usage.PromptTokens > 0 {
						lastInputTokens = event.Usage.PromptTokens
					}
					if event.Usage.CompletionTokens > 0 {
						lastOutputTokens = event.Usage.CompletionTokens
					}
				} else if estimateTokens {
					// Estimate tokens if no usage data provided
					event.Usage = &protocol.Usage{
						PromptTokens:     lastInputTokens,
						CompletionTokens: lastOutputTokens,
					}

					// Update output token estimate based on content
					if event.Content != "" {
						contentTokens := estimateTokenCount(event.Content)
						totalContentLength += contentTokens
						event.Usage.CompletionTokens = totalContentLength
						lastOutputTokens = totalContentLength
					}
				} else {
					// Inject last known counts
					event.Usage = &protocol.Usage{
						PromptTokens:     lastInputTokens,
						CompletionTokens: lastOutputTokens,
					}
				}

				// Update total tokens
				if event.Usage != nil {
					event.Usage.TotalTokens = event.Usage.PromptTokens + event.Usage.CompletionTokens
				}

				// Forward normalized event
				select {
				case normalizedChan <- event:
				case <-ctx.Done():
					n.logger.Debug("usage normalization cancelled while sending event")
					return
				}
			}
		}
	}()

	return normalizedChan
}

// estimateTokenCount provides a rough estimate of token count based on content length.
// This is a simple heuristic: approximately 4 characters per token for English text.
// This is used as a fallback when providers don't report token usage.
func estimateTokenCount(content string) int {
	if content == "" {
		return 0
	}

	// Count UTF-8 characters (not bytes)
	charCount := utf8.RuneCountInString(content)

	// Rough heuristic: ~4 characters per token
	// This varies by language and tokenizer, but provides a reasonable estimate
	tokenCount := charCount / 4
	if tokenCount == 0 && charCount > 0 {
		tokenCount = 1 // Minimum 1 token for non-empty content
	}

	return tokenCount
}

// EstimateTokensForRequest estimates the token count for a complete request.
// This is useful for pre-flight validation and cost estimation.
func EstimateTokensForRequest(req *protocol.UnifiedChatRequest) int {
	totalTokens := 0

	// Estimate system prompt tokens
	if req.System != "" {
		totalTokens += estimateTokenCount(req.System)
	}

	// Estimate message tokens
	for _, msg := range req.Messages {
		// Add role overhead (approximately 4 tokens per message for role markers)
		totalTokens += 4
		totalTokens += estimateTokenCount(msg.Content)
	}

	// Estimate tool definition tokens (tools add significant overhead)
	for _, tool := range req.Tools {
		// Tool name and description
		totalTokens += estimateTokenCount(tool.Name)
		totalTokens += estimateTokenCount(tool.Description)
		
		// Input schema (rough estimate: 50 tokens per tool schema)
		totalTokens += 50
	}

	return totalTokens
}

// NormalizeUsageInPlace updates a UnifiedStreamEvent with normalized usage data.
// This is a helper for single-event normalization without streaming.
func NormalizeUsageInPlace(event *protocol.UnifiedStreamEvent, lastInputTokens, lastOutputTokens int) {
	if event.Usage == nil {
		event.Usage = &protocol.Usage{
			PromptTokens:     lastInputTokens,
			CompletionTokens: lastOutputTokens,
		}
	} else {
		// Fill in missing values
		if event.Usage.PromptTokens == 0 {
			event.Usage.PromptTokens = lastInputTokens
		}
		if event.Usage.CompletionTokens == 0 {
			event.Usage.CompletionTokens = lastOutputTokens
		}
	}

	// Update total
	event.Usage.TotalTokens = event.Usage.PromptTokens + event.Usage.CompletionTokens
}

// ValidateUsage checks if usage data is reasonable and logs warnings for anomalies.
func (n *UsageNormalizer) ValidateUsage(usage *protocol.Usage, context string) {
	if usage == nil {
		n.logger.Warn("missing usage data", "context", context)
		return
	}

	// Check for negative values
	if usage.PromptTokens < 0 || usage.CompletionTokens < 0 || usage.TotalTokens < 0 {
		n.logger.Warn("negative token count detected",
			"context", context,
			"prompt_tokens", usage.PromptTokens,
			"completion_tokens", usage.CompletionTokens,
			"total_tokens", usage.TotalTokens)
	}

	// Check for inconsistent totals
	expectedTotal := usage.PromptTokens + usage.CompletionTokens
	if usage.TotalTokens != expectedTotal && usage.TotalTokens != 0 {
		n.logger.Warn("inconsistent token total",
			"context", context,
			"reported_total", usage.TotalTokens,
			"calculated_total", expectedTotal)
	}

	// Check for unreasonably large values (likely an error)
	const maxReasonableTokens = 1_000_000
	if usage.TotalTokens > maxReasonableTokens {
		n.logger.Warn("unusually large token count",
			"context", context,
			"total_tokens", usage.TotalTokens)
	}
}

// FormatUsageString creates a human-readable usage summary.
func FormatUsageString(usage *protocol.Usage) string {
	if usage == nil {
		return "no usage data"
	}

	var parts []string
	if usage.PromptTokens > 0 {
		parts = append(parts, fmt.Sprintf("input: %d", usage.PromptTokens))
	}
	if usage.CompletionTokens > 0 {
		parts = append(parts, fmt.Sprintf("output: %d", usage.CompletionTokens))
	}
	if usage.TotalTokens > 0 {
		parts = append(parts, fmt.Sprintf("total: %d", usage.TotalTokens))
	}

	if len(parts) == 0 {
		return "no tokens"
	}

	return strings.Join(parts, ", ")
}
