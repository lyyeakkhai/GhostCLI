// Package kimi provides the provider adapter for the Kimi (Moonshot AI) API.
// Kimi implements the OpenAI Chat Completions wire format, so this adapter is a
// thin configuration wrapper around the OpenAI-compatible base adapter.
//
// Requirements: 7
package kimi

// DefaultBaseURL is the root URL for the Moonshot AI Chat Completions API.
// The base adapter will append "/chat/completions" to this path.
const DefaultBaseURL = "https://api.moonshot.cn/v1"

// ProviderName is the canonical identifier for the Kimi provider.
const ProviderName = "kimi"

// DefaultModelMap maps Anthropic model names to Kimi (Moonshot AI) model identifiers.
// Requirements: 7.3, 19
var DefaultModelMap = map[string]string{
	// Claude 3.5 Sonnet family → moonshot-v1-128k (most capable, large context)
	"claude-3-5-sonnet-20241022": "moonshot-v1-128k",
	"claude-3-5-sonnet-20240620": "moonshot-v1-128k",
	"claude-3-5-sonnet":          "moonshot-v1-128k",

	// Claude 3 Opus → moonshot-v1-128k (highest capability tier)
	"claude-3-opus-20240229": "moonshot-v1-128k",
	"claude-3-opus":          "moonshot-v1-128k",

	// Claude 3 Haiku → moonshot-v1-8k (smaller/faster tier)
	"claude-3-haiku-20240307": "moonshot-v1-8k",
	"claude-3-haiku":          "moonshot-v1-8k",

	// Claude 3.5 Haiku → moonshot-v1-8k (smaller/faster tier)
	"claude-3-5-haiku-20241022": "moonshot-v1-8k",
	"claude-3-5-haiku":          "moonshot-v1-8k",
}

// DefaultModel is the fallback model when no mapping is found.
const DefaultModel = "moonshot-v1-32k"
