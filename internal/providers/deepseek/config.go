// Package deepseek provides the provider adapter for the DeepSeek API.
// DeepSeek implements the OpenAI Chat Completions wire format, so this
// adapter is a thin configuration wrapper around the OpenAI-compatible base.
//
// Requirements: 6
package deepseek

// DefaultBaseURL is the root URL for the DeepSeek Chat Completions API.
// The base adapter will append "/chat/completions" to this path.
const DefaultBaseURL = "https://api.deepseek.com/v1"

// ProviderName is the canonical identifier for the DeepSeek provider.
const ProviderName = "deepseek"

// DefaultModelMap maps Anthropic model names to DeepSeek model identifiers.
// Requirements: 6.3, 19
var DefaultModelMap = map[string]string{
	// Claude 3.5 Sonnet family → deepseek-chat (most capable general model)
	"claude-3-5-sonnet-20241022": "deepseek-chat",
	"claude-3-5-sonnet-20240620": "deepseek-chat",
	"claude-3-5-sonnet":          "deepseek-chat",

	// Claude 3.5 Haiku → deepseek-chat (smaller/faster tier)
	"claude-3-5-haiku-20241022": "deepseek-chat",
	"claude-3-5-haiku":          "deepseek-chat",

	// Claude 3 family → deepseek-chat
	"claude-3-opus-20240229":   "deepseek-chat",
	"claude-3-sonnet-20240229": "deepseek-chat",
	"claude-3-haiku-20240307":  "deepseek-chat",
}
