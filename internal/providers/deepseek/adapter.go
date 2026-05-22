// Package deepseek provides the provider adapter for the DeepSeek API.
// DeepSeek exposes an OpenAI-compatible Chat Completions endpoint, so this
// adapter delegates all heavy lifting to the OpenAI-compatible base adapter
// and only supplies DeepSeek-specific configuration (base URL, model map).
//
// Requirements: 6
package deepseek

import (
	"log/slog"

	"ghostcli/internal/providers/base"
)

// Adapter is the DeepSeek provider adapter.
// It embeds the OpenAI-compatible base adapter configured for api.deepseek.com.
//
// Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7
type Adapter struct {
	*base.OpenAIAdapter
}

// NewAdapter creates a new DeepSeek provider adapter using the production base
// URL (https://api.deepseek.com/v1).
//
// apiKey is the DeepSeek API key, sent as a Bearer token in the Authorization
// header on every outbound request.
//
// modelMap overrides the default model mapping. Pass nil to use DefaultModelMap.
//
// logger is the structured logger; a no-op logger is used when nil is passed.
//
// Requirements: 6
func NewAdapter(apiKey string, modelMap map[string]string, logger *slog.Logger) *Adapter {
	return NewAdapterWithBaseURL(DefaultBaseURL, apiKey, modelMap, logger)
}

// NewAdapterWithBaseURL is like NewAdapter but accepts an explicit base URL.
// This is primarily useful in tests to redirect traffic to a fake HTTP server
// instead of the real DeepSeek API.
//
// Requirements: 6
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
		// Requirements: 6.4
		BaseURL: baseURL,

		// API key is sent as a Bearer token.
		// Requirements: 6.7
		APIKey:     apiKey,
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer ",

		// Model mapping (Anthropic → DeepSeek).
		// Requirements: 6.3, 19
		ModelMap: modelMap,
	}

	return &Adapter{
		OpenAIAdapter: base.NewOpenAIAdapter(cfg, logger),
	}
}
