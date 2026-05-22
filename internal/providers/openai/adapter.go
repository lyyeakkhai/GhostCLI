// Package openai provides the provider adapter for the OpenAI API.
// OpenAI is the originator of the Chat Completions wire format, so this
// adapter delegates all heavy lifting to the OpenAI-compatible base adapter
// and only supplies OpenAI-specific configuration (base URL, model map).
//
// Requirements: 8
package openai

import (
	"log/slog"

	"ghostcli/internal/providers/base"
)

// Adapter is the OpenAI provider adapter.
// It embeds the OpenAI-compatible base adapter configured for api.openai.com.
//
// Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7
type Adapter struct {
	*base.OpenAIAdapter
}

// NewAdapter creates a new OpenAI provider adapter using the production base
// URL (https://api.openai.com/v1).
//
// apiKey is the OpenAI API key, sent as a Bearer token in the Authorization
// header on every outbound request.
//
// modelMap overrides the default model mapping. Pass nil to use DefaultModelMap.
//
// logger is the structured logger; the default logger is used when nil is passed.
//
// Requirements: 8
func NewAdapter(apiKey string, modelMap map[string]string, logger *slog.Logger) *Adapter {
	return NewAdapterWithBaseURL(DefaultBaseURL, apiKey, modelMap, logger)
}

// NewAdapterWithBaseURL is like NewAdapter but accepts an explicit base URL.
// This is primarily useful in tests to redirect traffic to a fake HTTP server
// instead of the real OpenAI API.
//
// Requirements: 8
func NewAdapterWithBaseURL(baseURL, apiKey string, modelMap map[string]string, logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}

	// Fall back to the built-in mapping when the caller does not supply one.
	if modelMap == nil {
		modelMap = DefaultModelMap
	}

	cfg := base.OpenAIConfig{
		// Provider identity
		Name: ProviderName,

		// Base URL (overridable for testing).
		// Requirements: 8.4
		BaseURL: baseURL,

		// API key is sent as a Bearer token.
		// Requirements: 8.7
		APIKey:     apiKey,
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer ",

		// Model mapping (Anthropic → OpenAI).
		// Requirements: 8.3, 19
		ModelMap: modelMap,
	}

	return &Adapter{
		OpenAIAdapter: base.NewOpenAIAdapter(cfg, logger),
	}
}
