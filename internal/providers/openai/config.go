// Package openai provides the provider adapter for the OpenAI API.
// OpenAI exposes the Chat Completions endpoint that all OpenAI-compatible
// adapters are modelled after. This adapter supplies OpenAI-specific
// configuration (base URL, model map) and delegates the rest to the base.
//
// Requirements: 8
package openai

// DefaultBaseURL is the root URL for the OpenAI Chat Completions API.
// The base adapter will append "/chat/completions" to this path.
const DefaultBaseURL = "https://api.openai.com/v1"

// ProviderName is the canonical identifier for the OpenAI provider.
const ProviderName = "openai"

// DefaultModelMap maps Anthropic model names to OpenAI model identifiers.
// Requirements: 8.3, 19
var DefaultModelMap = map[string]string{
	// Claude 3.5 Sonnet family → gpt-4o (most capable)
	"claude-3-5-sonnet-20241022": "gpt-4o",
	"claude-3-5-sonnet-20240620": "gpt-4o",
	"claude-3-5-sonnet":          "gpt-4o",

	// Claude 3.5 Haiku → gpt-4o-mini (fast, cost-efficient)
	"claude-3-5-haiku-20241022": "gpt-4o-mini",
	"claude-3-5-haiku":          "gpt-4o-mini",

	// Claude 3 Opus → gpt-4-turbo (high-capability)
	"claude-3-opus-20240229": "gpt-4-turbo",
	"claude-3-opus":          "gpt-4-turbo",

	// Claude 3 Sonnet / Haiku → gpt-4o / gpt-4o-mini
	"claude-3-sonnet-20240229": "gpt-4o",
	"claude-3-haiku-20240307":  "gpt-4o-mini",
	"claude-3-haiku":           "gpt-4o-mini",
}
