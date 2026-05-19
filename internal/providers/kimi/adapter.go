// Package kimi provides the provider adapter for the Kimi (Moonshot AI) API.
// Kimi exposes an OpenAI-compatible Chat Completions endpoint, so this adapter
// delegates all heavy lifting to the OpenAI-compatible base adapter and only
// supplies Kimi-specific configuration (base URL, model map).
//
// Requirements: 7
package kimi

import (
	"log/slog"

	"ghostcli/internal/providers/base"
)

// Adapter is the Kimi provider adapter.
// It embeds the OpenAI-compatible base adapter configured for api.moonshot.cn.
//
// Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7
type Adapter struct {
	*base.OpenAIAdapter
}

// NewAdapter creates a new Kimi provider adapter using the production base
// URL (https://api.moonshot.cn/v1).
//
// apiKey is the Moonshot AI API key, sent as a Bearer token in the Authorization
// header on every outbound request.
//
// modelMap overrides the default model mapping. Pass nil to use DefaultModelMap.
//
// logger is the structured logger; a no-op logger is used when nil is passed.
//
// Requirements: 7
func NewAdapter(apiKey string, modelMap map[string]string, logger *slog.Logger) *Adapter {
	return NewAdapterWithBaseURL(DefaultBaseURL, apiKey, modelMap, logger)
}

// NewAdapterWithBaseURL is like NewAdapter but accepts an explicit base URL.
// This is primarily useful in tests to redirect traffic to a fake HTTP server
// instead of the real Kimi API.
//
// Requirements: 7
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
		// Requirements: 7.4
		BaseURL: baseURL,

		// API key is sent as a Bearer token.
		// Requirements: 7.7
		APIKey:     apiKey,
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer ",

		// Model mapping (Anthropic → Kimi/Moonshot).
		// Requirements: 7.3, 19
		ModelMap: modelMap,
	}

	return &Adapter{
		OpenAIAdapter: base.NewOpenAIAdapter(cfg, logger),
	}
}
