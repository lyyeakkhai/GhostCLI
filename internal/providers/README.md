# Providers Package

This package defines the core `Provider` interface and related components for integrating LLM providers into GhostCLI.

## Overview

The providers package implements a pattern-based architecture that enables GhostCLI to support multiple LLM providers (DeepSeek, Kimi, OpenAI, Kiro, etc.) through a unified interface. Each provider adapter translates between the internal unified protocol and the provider's native API format.

## Provider Interface

The `Provider` interface defines the contract that all provider adapters must implement:

```go
type Provider interface {
    StreamChat(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error)
    Name() string
    SupportsTools() bool
    SupportsThinking() bool
    MapModel(anthropicModel string) string
}
```

### Methods

#### StreamChat

```go
StreamChat(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error)
```

Initiates a streaming chat request to the provider. This is the core method that:
- Accepts a context for cancellation and timeout control
- Takes a `UnifiedChatRequest` containing normalized request data
- Returns a channel of `UnifiedStreamEvent` objects for streaming responses
- Returns an error if the request cannot be initiated

**Implementation Requirements:**
- The returned channel must be closed when the stream completes or encounters an error
- The implementation must respect context cancellation and close the provider connection immediately
- Error events should be emitted through the channel rather than returned directly when possible
- The channel should be buffered appropriately to prevent blocking

#### Name

```go
Name() string
```

Returns the unique identifier for this provider (e.g., "deepseek", "kimi", "openai"). This identifier is used for:
- Provider selection in configuration
- Logging and telemetry
- Registry lookups

#### SupportsTools

```go
SupportsTools() bool
```

Indicates whether this provider supports function/tool calling. Returns `true` if the provider can:
- Handle tool definitions in requests
- Emit tool call events in responses
- Process tool results in subsequent messages

#### SupportsThinking

```go
SupportsThinking() bool
```

Indicates whether this provider supports extended thinking blocks. Returns `true` if the provider can emit thinking events that show the model's reasoning process before generating the final response.

#### MapModel

```go
MapModel(anthropicModel string) string
```

Translates Anthropic model names to provider-specific model identifiers. For example:
- `"claude-3-5-sonnet"` → `"deepseek-chat"` (DeepSeek)
- `"claude-3-5-sonnet"` → `"moonshot-v1-128k"` (Kimi)
- `"claude-3-5-sonnet"` → `"gpt-4o"` (OpenAI)

If no mapping exists, implementations should return the original model name unchanged.

## Provider Patterns

GhostCLI uses pattern-based provider adapters to minimize code duplication:

### Pattern A: OpenAI-Compatible
For providers using OpenAI Chat Completions format (DeepSeek, Kimi, OpenAI, Groq, etc.)

### Pattern B: Anthropic-Native
For providers using Anthropic Messages API format (Anthropic, OpenRouter, Kiro Gateway)

### Pattern C: AWS EventStream
For providers using AWS EventStream protocol (Kiro, Amazon Bedrock)

## Implementation Guidelines

When implementing a new provider adapter:

1. **Choose the appropriate base pattern** - Most providers will use the OpenAI-compatible pattern
2. **Implement the Provider interface** - All five methods must be implemented
3. **Handle context cancellation** - Respect `ctx.Done()` and close connections immediately
4. **Emit events promptly** - Don't buffer events; emit them as soon as they're received
5. **Normalize token usage** - Ensure usage data is included in events
6. **Handle errors gracefully** - Emit error events through the channel when possible
7. **Test thoroughly** - Write unit tests for model mapping, streaming, and error cases

## Example Usage

```go
// Create a provider instance
provider := &DeepSeekAdapter{
    config: Config{
        Name:    "deepseek",
        BaseURL: "https://api.deepseek.com",
        APIKey:  "sk-...",
    },
}

// Create a request
req := &protocol.UnifiedChatRequest{
    Model:     "claude-3-5-sonnet",
    MaxTokens: 1000,
    Messages: []protocol.Message{
        {Role: "user", Content: "Hello!"},
    },
    Stream: true,
}

// Stream the response
ctx := context.Background()
eventChan, err := provider.StreamChat(ctx, req)
if err != nil {
    log.Fatal(err)
}

// Process events
for event := range eventChan {
    switch event.Type {
    case protocol.EventToken:
        fmt.Print(event.Content)
    case protocol.EventError:
        log.Printf("Error: %v", event.Error)
    case protocol.EventStop:
        fmt.Println("\nDone!")
    }
}
```

## Related Packages

- `internal/engine/protocol` - Defines `UnifiedChatRequest` and `UnifiedStreamEvent`
- `internal/providers/registry` - Provider registration and lookup
- `internal/providers/factory` - Provider instantiation with dependency injection
- `internal/providers/base` - Base adapter implementations for each pattern
