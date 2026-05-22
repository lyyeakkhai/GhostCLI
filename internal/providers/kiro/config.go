// Package kiro provides the provider adapter for the Kiro API.
// Kiro uses the AWS EventStream binary framing protocol (Bedrock-compatible),
// so this adapter is a thin configuration wrapper around the AWS EventStream
// base adapter and only supplies Kiro-specific configuration (base URL,
// chat path, model map).
//
// Requirements: 4
package kiro

// DefaultBaseURL is the root URL for the Kiro API endpoint.
const DefaultBaseURL = "https://codewhisperer.us-east-1.amazonaws.com"

// DefaultChatPath is the path appended to DefaultBaseURL for chat completions.
const DefaultChatPath = "/v1/messages"

// ProviderName is the canonical identifier for the Kiro provider.
const ProviderName = "kiro"

// DefaultModelMap maps Anthropic model names to Kiro/Bedrock model identifiers.
// Kiro uses Amazon Bedrock model IDs which differ from plain Anthropic model names.
//
// Requirements: 4.5, 19
var DefaultModelMap = map[string]string{
	// Claude 3.5 Sonnet family → Bedrock Claude 3.5 Sonnet
	"claude-3-5-sonnet-20241022": "anthropic.claude-3-5-sonnet-20241022-v2:0",
	"claude-3-5-sonnet-20240620": "anthropic.claude-3-5-sonnet-20240620-v1:0",
	"claude-3-5-sonnet":          "anthropic.claude-3-5-sonnet-20241022-v2:0",

	// Claude 3.5 Haiku → Bedrock Claude 3.5 Haiku
	"claude-3-5-haiku-20241022": "anthropic.claude-3-5-haiku-20241022-v1:0",
	"claude-3-5-haiku":          "anthropic.claude-3-5-haiku-20241022-v1:0",

	// Claude 3 Opus → Bedrock Claude 3 Opus
	"claude-3-opus-20240229": "anthropic.claude-3-opus-20240229-v1:0",
	"claude-3-opus":          "anthropic.claude-3-opus-20240229-v1:0",

	// Claude 3 Sonnet → Bedrock Claude 3 Sonnet
	"claude-3-sonnet-20240229": "anthropic.claude-3-sonnet-20240229-v1:0",
	"claude-3-sonnet":          "anthropic.claude-3-sonnet-20240229-v1:0",

	// Claude 3 Haiku → Bedrock Claude 3 Haiku
	"claude-3-haiku-20240307": "anthropic.claude-3-haiku-20240307-v1:0",
	"claude-3-haiku":          "anthropic.claude-3-haiku-20240307-v1:0",
}
