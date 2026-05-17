# Module 02: Translation Engine

## Overview

The Translation Engine is the core of GhostCLI, responsible for bidirectional translation between Anthropic Messages API format and the internal Unified Protocol. It enables provider-agnostic message representation and streaming.

## Responsibilities

- Parse incoming Anthropic JSON requests
- Convert to UnifiedChatRequest (provider-agnostic format)
- Convert UnifiedStreamEvent back to Anthropic SSE format
- Normalize token usage across providers
- Handle streaming with zero-buffer design
- Support tool calls and thinking blocks

## Architecture

```
Translation Engine
├── Inbound Translation
│   └── AnthropicIn Parser (Anthropic JSON → UnifiedChatRequest)
├── Unified Protocol
│   ├── UnifiedChatRequest (request representation)
│   └── UnifiedStreamEvent (streaming event representation)
├── Outbound Translation
│   └── AnthropicOut Formatter (UnifiedStreamEvent → Anthropic SSE)
└── Streaming Pipeline
    ├── Token Usage Normalization
    └── Context Propagation
```

## Related Requirements

- **Requirement 2**: Anthropic Request Parsing
- **Requirement 5**: Streaming Response Translation
- **Requirement 14**: Request Context Propagation
- **Requirement 15**: Token Usage Normalization
- **Requirement 24**: Streaming Performance Optimization
- **Requirement 25**: Thinking Block Handling

## Key Components

### AnthropicIn Parser
- Streaming JSON decoder (no buffering)
- Extract model, messages, system, temperature, max_tokens, tools, stream
- Normalize system prompt (string or array of content blocks)
- Convert multi-part content blocks
- Handle tool definitions

### Unified Protocol
**UnifiedChatRequest**: Provider-agnostic request format
- Model, System, Messages, MaxTokens, Temperature, TopP
- Tools (normalized format)
- Stream flag

**UnifiedStreamEvent**: Provider-agnostic streaming event
- Event types: message_start, content_block_delta, message_stop, error
- Content deltas (text, tool calls, thinking)
- Token usage
- Finish reason

### AnthropicOut Formatter
- Convert UnifiedStreamEvent to Anthropic SSE format
- Write SSE events with proper formatting
- Flush immediately after each event (zero-buffer)
- Track and inject token usage
- Handle message_stop event

### Token Usage Normalization
- Maintain running count of input_tokens and output_tokens
- Inject usage into events that lack it
- Estimate tokens from content length if unavailable

## Data Flow

```
Claude Code
    ↓ (Anthropic JSON)
AnthropicIn Parser
    ↓ (UnifiedChatRequest)
Provider Router
    ↓
Provider Adapter
    ↓ (UnifiedStreamEvent channel)
AnthropicOut Formatter
    ↓ (Anthropic SSE)
Claude Code
```

## Implementation Details

See [design.md](./design.md) for detailed implementation specifications.
