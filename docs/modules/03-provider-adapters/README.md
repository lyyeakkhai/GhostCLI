# Module 03: Provider Adapters

## Overview

The Provider Adapters module implements the integration layer for various LLM providers using a pattern-based architecture. It enables GhostCLI to support multiple providers with minimal code duplication.

## Responsibilities

- Define Provider interface for all adapters
- Implement pattern-based adapter families
- Route requests to appropriate provider
- Translate UnifiedChatRequest to provider-specific format
- Parse provider responses into UnifiedStreamEvent
- Map Anthropic model names to provider models
- Handle tool calls and thinking blocks per provider

## Architecture

```
Provider Adapters
├── Provider Interface
├── Provider Registry (thread-safe map)
├── Provider Factory (DI pattern)
├── Pattern Families
│   ├── Pattern A: OpenAI-Compatible
│   │   ├── DeepSeek Adapter
│   │   ├── Kimi Adapter
│   │   └── OpenAI Adapter
│   ├── Pattern B: Anthropic-Native
│   │   └── Kiro Adapter (passthrough)
│   └── Pattern C: AWS EventStream
│       └── Kiro Adapter (binary framing)
└── Shared Logic
    ├── Model Mapping
    ├── Tool Call Translation
    └── Error Handling
```

## Related Requirements

- **Requirement 3**: Provider Routing
- **Requirement 4**: Provider Adapter Interface
- **Requirement 6**: DeepSeek Provider Adapter
- **Requirement 7**: Kimi Provider Adapter
- **Requirement 8**: OpenAI Provider Adapter
- **Requirement 19**: Model Name Mapping
- **Requirement 20**: Tool Call Support
- **Requirement 27**: Provider Adapter Registry Initialization

## Pattern Families

### Pattern A: OpenAI-Compatible
**Providers**: DeepSeek, Kimi, OpenAI, Groq, Together AI, Fireworks

**Characteristics**:
- Endpoint: `POST /v1/chat/completions`
- Format: OpenAI Chat Completions JSON
- Streaming: Standard OpenAI SSE chunks
- Auth: Bearer token in Authorization header

**Reuse Strategy**: Single base adapter with different configs (BaseURL, APIKey, ModelMap)

### Pattern B: Anthropic-Native
**Providers**: Anthropic, OpenRouter, Kiro Gateway

**Characteristics**:
- Endpoint: `POST /v1/messages`
- Format: Anthropic Messages API JSON
- Streaming: Anthropic SSE events
- Auth: x-api-key header

**Reuse Strategy**: Minimal translation (mostly passthrough with usage normalization)

### Pattern C: AWS EventStream
**Providers**: Kiro, Amazon Bedrock

**Characteristics**:
- Protocol: Binary-framed EventStream over HTTP
- Format: AWS-specific message format
- Streaming: Binary event frames
- Auth: AWS Signature V4 (for Bedrock)

**Reuse Strategy**: Specialized adapter for binary framing and AWS format

## Provider Interface

```go
type Provider interface {
    StreamChat(ctx context.Context, req *UnifiedChatRequest) (<-chan UnifiedStreamEvent, error)
    Name() string
    SupportsTools() bool
    SupportsThinking() bool
    MapModel(anthropicModel string) string
}
```

## Model Mapping

Each provider maintains a mapping table:

```go
// DeepSeek
claude-3-5-sonnet → deepseek-chat
claude-3-opus     → deepseek-chat
claude-3-haiku    → deepseek-chat

// Kimi
claude-3-5-sonnet → moonshot-v1-128k
claude-3-haiku    → moonshot-v1-8k

// OpenAI
claude-3-5-sonnet → gpt-4o
claude-3-haiku    → gpt-4o-mini
```

## Implementation Details

See [design.md](./design.md) for detailed implementation specifications.

See [patterns/](./patterns/) for pattern-specific documentation:
- [Pattern A: OpenAI-Compatible](./patterns/openai-compatible.md)
- [Pattern B: Anthropic-Native](./patterns/anthropic-native.md)
- [Pattern C: AWS EventStream](./patterns/aws-eventstream.md)
