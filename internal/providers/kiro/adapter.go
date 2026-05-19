// Package kiro provides the provider adapter for the Kiro API.
// Kiro uses the AWS EventStream binary framing protocol (Bedrock-compatible),
// so this adapter delegates all heavy lifting to the AWS EventStream base
// adapter and only supplies Kiro-specific configuration (base URL, chat path,
// model map).
//
// Requirements: 4
package kiro

import (
	"log/slog"

	"ghostcli/internal/providers/base"
)

// Adapter is the Kiro provider adapter.
// It embeds the AWS EventStream base adapter configured for the Kiro endpoint.
//
// Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7
type Adapter struct {
	*base.AWSAdapter
}

// NewAdapter creates a new Kiro provider adapter using the production base URL.
//
// apiKey is the Kiro/AWS API key, sent as a Bearer token in the Authorization
// header on every outbound request.
//
// modelMap overrides the default model mapping. Pass nil to use DefaultModelMap.
//
// logger is the structured logger; slog.Default() is used when nil is passed.
//
// Requirements: 4
func NewAdapter(apiKey string, modelMap map[string]string, logger *slog.Logger) *Adapter {
	return NewAdapterWithBaseURL(DefaultBaseURL, apiKey, modelMap, logger)
}

// NewAdapterWithBaseURL is like NewAdapter but accepts an explicit base URL.
// This is primarily useful in tests to redirect traffic to a fake HTTP server
// instead of the real Kiro API.
//
// Requirements: 4
func NewAdapterWithBaseURL(baseURL, apiKey string, modelMap map[string]string, logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}

	// Fall back to the built-in mapping when the caller does not supply one.
	if modelMap == nil {
		modelMap = DefaultModelMap
	}

	cfg := base.AWSAdapterConfig{
		// Provider identity
		Name: ProviderName,

		// Base URL (overridable for testing).
		BaseURL: baseURL,

		// Chat path for the Kiro messages endpoint.
		ChatPath: DefaultChatPath,

		// API key is sent as a Bearer token.
		// Requirements: 4
		APIKey: apiKey,

		// Model mapping (Anthropic → Bedrock/Kiro).
		// Requirements: 4.5, 19
		ModelMap: modelMap,

		Logger: logger,
	}

	return &Adapter{
		AWSAdapter: base.NewAWSAdapter(cfg),
	}
}
