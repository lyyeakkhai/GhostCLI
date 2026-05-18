package providers

import (
	"context"

	"ghostcli/internal/engine/protocol"
)

// Provider defines the interface that all LLM provider adapters must implement.
// This abstraction enables the proxy to support multiple providers (DeepSeek, Kimi,
// OpenAI, Kiro, etc.) through a unified interface while maintaining provider-specific
// optimizations and features.
type Provider interface {
	// StreamChat initiates a streaming chat request to the provider.
	// It accepts a context for cancellation and a UnifiedChatRequest containing
	// the normalized request data. Returns a channel of UnifiedStreamEvent objects
	// that will emit streaming responses, and an error if the request cannot be initiated.
	//
	// The returned channel will be closed when the stream completes or encounters an error.
	// Callers should monitor the context for cancellation and handle error events from the stream.
	StreamChat(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error)

	// Name returns the unique identifier for this provider (e.g., "deepseek", "kimi", "openai").
	// This identifier is used for provider selection, logging, and configuration lookup.
	Name() string

	// SupportsTools indicates whether this provider supports function/tool calling.
	// Returns true if the provider can handle tool definitions in requests and
	// emit tool call events in responses.
	SupportsTools() bool

	// SupportsThinking indicates whether this provider supports extended thinking blocks.
	// Returns true if the provider can emit thinking events that show the model's
	// reasoning process before generating the final response.
	SupportsThinking() bool

	// MapModel translates Anthropic model names to provider-specific model identifiers.
	// For example, "claude-3-5-sonnet" might map to "deepseek-chat" for DeepSeek,
	// "moonshot-v1-128k" for Kimi, or "gpt-4o" for OpenAI.
	//
	// If no mapping exists for the given model name, implementations should return
	// the original model name unchanged.
	MapModel(anthropicModel string) string
}
