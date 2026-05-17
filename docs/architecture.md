# GhostCLI System Architecture

> **Start Here**: This document provides the top-level architectural overview of GhostCLI. For detailed component documentation, see the [components/](./components/) directory.

## What is GhostCLI?

**GhostCLI** (also known as **DeepClaude-Go**) is a high-performance, open-source proxy that connects **Claude Code** to various LLM providers (DeepSeek, OpenAI, Kimi, Kiro, etc.). It acts as a translation layer, allowing you to use Claude Code's interface with any compatible LLM provider at a fraction of the cost.

### Key Innovation: Unified Protocol Architecture

GhostCLI implements a **Unified Protocol** that acts as an abstraction layer between Claude Code (which speaks Anthropic's API format) and various LLM providers (which each have their own formats).

```
┌─────────────┐
│ Claude Code │ (Anthropic API)
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────┐
│         GhostCLI Proxy              │
│  ┌───────────────────────────────┐  │
│  │   Unified Protocol Layer      │  │
│  └───────────────────────────────┘  │
└──────┬──────────────────┬───────────┘
       │                  │
       ▼                  ▼
┌──────────┐      ┌──────────────┐
│ DeepSeek │      │ Kimi / Kiro  │
└──────────┘      └──────────────┘
```

## Core Design Principles

### 1. Performance
- **Zero-buffer parsing**: Uses `json.NewDecoder` to avoid loading entire payloads into memory
- **Streaming-first**: Processes tokens as they arrive, minimizing time-to-first-token (TTFT)
- **Go-native**: Compiled to static binaries with no runtime dependencies

### 2. Scalability
- **Pattern-first design**: Groups providers into reusable families (OpenAI-Compatible, Anthropic-Native, AWS EventStream)
- **Extensibility**: Adding a new provider requires only a configuration entry, not new translation code

### 3. Security
- **OS-native secret management**: Uses system keyrings (Keychain, Windows Credential Manager, Linux Secret Service)
- **No plaintext keys**: API keys never stored in plain text on disk

### 4. User Experience
- **One-command setup**: Interactive first-run wizard for provider selection
- **Seamless integration**: Works with Claude Code via a single environment variable

## System Architecture

### High-Level Component View

```
┌─────────────────────────────────────────────────────────────┐
│                        GhostCLI                             │
│                                                             │
│  ┌──────────┐    ┌──────────────┐    ┌─────────────────┐  │
│  │   CLI    │───▶│ HTTP Server  │───▶│   Translation   │  │
│  │  Layer   │    │   (Router)   │    │     Engine      │  │
│  └──────────┘    └──────────────┘    └────────┬────────┘  │
│                                                │            │
│                                                ▼            │
│                                    ┌───────────────────┐   │
│                                    │ Provider Adapters │   │
│                                    │   (Patterns)      │   │
│                                    └─────────┬─────────┘   │
│                                              │             │
└──────────────────────────────────────────────┼─────────────┘
                                               │
                    ┌──────────────────────────┼──────────────────────┐
                    ▼                          ▼                      ▼
            ┌───────────────┐        ┌─────────────┐        ┌────────────┐
            │   DeepSeek    │        │    Kimi     │        │    Kiro    │
            │ (OpenAI API)  │        │ (OpenAI API)│        │(Anthropic) │
            └───────────────┘        └─────────────┘        └────────────┘
```

### Core Components

| Component | Purpose | Documentation |
|-----------|---------|---------------|
| **CLI Layer** | Command-line interface, flag parsing, interactive onboarding | [components/cli.md](./components/cli.md) |
| **HTTP Server** | HTTP routing, middleware, health checks | [components/http-server.md](./components/http-server.md) |
| **Translation Engine** | Bidirectional translation between Anthropic and Unified Protocol | [components/translation-engine.md](./components/translation-engine.md) |
| **Provider Adapters** | Provider-specific integrations using pattern-based architecture | [components/provider-adapters.md](./components/provider-adapters.md) |
| **Security** | OS-native keyring integration, credential management | [components/security.md](./components/security.md) |
| **Observability** | Structured logging, metrics, telemetry | [components/observability.md](./components/observability.md) |

## Request Flow

### 1. Ingestion (Claude Code → GhostCLI)

```
Claude Code
    │
    │ POST /v1/messages
    │ Content-Type: application/json
    │ { "model": "claude-3-7-sonnet", "messages": [...] }
    ▼
HTTP Server
    │
    │ Zero-buffer parsing
    ▼
AnthropicIn Parser
    │
    │ Converts to UnifiedChatRequest struct
    ▼
Unified Protocol Layer
```

### 2. Translation (Unified → Provider)

```
Unified Protocol Layer
    │
    │ UnifiedChatRequest
    ▼
Provider Router
    │
    │ Selects adapter based on --mode flag
    ▼
Provider Adapter (e.g., DeepSeek)
    │
    │ Encoder: Unified → Provider JSON
    │ { "model": "deepseek-v4-pro", "messages": [...] }
    ▼
Provider API (DeepSeek)
```

### 3. Streaming (Provider → Claude Code)

```
Provider API
    │
    │ SSE Stream: data: {"choices": [{"delta": {"content": "Hello"}}]}
    ▼
Provider Adapter
    │
    │ Decoder: Provider SSE → UnifiedStreamEvent
    ▼
AnthropicOut Formatter
    │
    │ UnifiedStreamEvent → Anthropic SSE
    │ event: content_block_delta
    │ data: {"type": "content_block_delta", "delta": {"text": "Hello"}}
    ▼
HTTP Server
    │
    │ Streams back to client
    ▼
Claude Code
```

## The Unified Protocol

The **Unified Protocol** is the internal data format that decouples the front door (Anthropic API) from the back door (provider APIs).

### Key Data Structures

```go
// Request format (internal)
type UnifiedChatRequest struct {
    Model       string
    Messages    []UnifiedMessage
    System      string
    MaxTokens   int
    Temperature float64
    Stream      bool
    Tools       []UnifiedTool
}

// Streaming event format (internal)
type UnifiedStreamEvent struct {
    Type    string  // "content_delta", "tool_call", "usage", etc.
    Content string
    Usage   *UsageInfo
}
```

### Why Unified Protocol?

1. **Decoupling**: The HTTP server never needs to know about provider-specific formats
2. **Consistency**: All providers produce the same internal event types
3. **Maintainability**: Changes to one provider don't affect others
4. **Testability**: Core logic can be tested without hitting real APIs

## Provider Patterns

Instead of writing custom translation code for every provider, GhostCLI groups providers into **Pattern Families** that share the same API structure.

### Pattern A: OpenAI-Compatible
**Providers**: DeepSeek, Kimi, Nvidia NIM, Fireworks, Groq, Together AI

**Characteristics**:
- Endpoint: `POST /v1/chat/completions`
- Format: `{"model": "...", "messages": [...]}`
- Streaming: Standard OpenAI SSE chunks

**Implementation**: One `OpenAIAdapter` with different base URLs

### Pattern B: Anthropic-Native
**Providers**: Anthropic (official), OpenRouter, KiroCC Gateway

**Characteristics**:
- Endpoint: `POST /v1/messages`
- Format: Native Anthropic with `anthropic-version` header
- Streaming: Anthropic SSE events

**Implementation**: Minimal translation (mostly passthrough)

### Pattern C: AWS EventStream
**Providers**: Kiro, Amazon Bedrock

**Characteristics**:
- Protocol: Binary-framed EventStream over HTTP
- Format: AWS-specific field names (`maxTokensToSample`)
- Auth: Signature V4 signing (for Bedrock)

**Implementation**: Specialized adapter for binary framing

### Adding New Providers

Thanks to the pattern-first design, adding a new provider is simple:

```go
// 1. Define configuration
var GroqConfig = ProviderConfig{
    Name:        "groq",
    BaseURL:     "https://api.groq.com/openai",
    DefaultModel: "llama-3.3-70b-versatile",
    Pattern:     PatternOpenAI,  // Reuse Pattern A
}

// 2. Register in one line
Registry["groq"] = factory.Create(GroqConfig)

// 3. Use it
// ghostcli --provider groq
```

## Technology Stack

### Core
- **Language**: Go 1.22+
- **HTTP Router**: Standard library `net/http` with custom middleware
- **JSON Parsing**: `encoding/json` with streaming decoder
- **Logging**: `log/slog` (structured logging)

### Security
- **Keyring**: `zalando/go-keyring` for OS-native secret storage

### CLI
- **Framework**: `spf13/cobra` for command-line interface
- **Config**: `spf13/viper` for configuration management
- **TUI**: `charmbracelet/bubbletea` for interactive prompts

### Distribution
- **Build**: `goreleaser` for cross-platform binary releases
- **CI/CD**: GitHub Actions for automated testing and releases
- **Package Managers**: Homebrew (macOS), Scoop (Windows), direct downloads (Linux)

## Project Structure

```
ghostcli/
├── cmd/
│   └── deepclaude/          # CLI entry point
├── internal/                # Private implementation
│   ├── app/                 # Application orchestration
│   ├── api/                 # HTTP server and routing
│   ├── engine/              # Translation engine
│   │   ├── protocol/        # Unified protocol types
│   │   └── pipeline/        # Transformation pipelines
│   ├── providers/           # Provider adapters
│   │   ├── base/            # Base abstractions
│   │   ├── factory/         # Provider factory
│   │   ├── deepseek/        # DeepSeek adapter
│   │   ├── kimi/            # Kimi adapter
│   │   └── kiro/            # Kiro adapter
│   └── telemetry/           # Logging and metrics
├── pkg/                     # Public utilities (optional)
└── docs/                    # Documentation
    ├── architecture/        # Architecture deep-dives
    ├── components/          # Component documentation
    ├── providers/           # Provider guides
    └── development/         # Development guides
```

## Engineering Principles

### SOLID Principles

1. **Single Responsibility**: Each component has one clear purpose
   - HTTP Server: routing and middleware
   - Translation Engine: format conversion
   - Provider Adapters: provider-specific logic

2. **Open/Closed**: System is open for extension, closed for modification
   - Add new providers without changing core engine
   - Add new patterns without modifying existing ones

3. **Liskov Substitution**: All provider adapters are interchangeable
   - Any `Provider` implementation can be swapped at runtime
   - Core engine doesn't depend on specific adapter implementations

4. **Interface Segregation**: Small, focused interfaces
   - `Provider` interface only defines `StreamChat()`
   - Components depend only on methods they use

5. **Dependency Inversion**: High-level modules depend on abstractions
   - HTTP Server depends on `Provider` interface, not concrete adapters
   - Enables dependency injection and testing

### Design Patterns

- **Factory Pattern**: `ProviderFactory` dynamically creates adapters
- **Strategy Pattern**: Provider adapters implement different translation strategies
- **Adapter Pattern**: Each provider adapter wraps a different API
- **Observer Pattern**: Streaming uses channels for event propagation

## Performance Characteristics

### Memory Efficiency
- **Zero-buffer parsing**: No intermediate string allocations
- **Streaming**: Constant memory usage regardless of response length
- **Struct-based**: Binary Go structs instead of map[string]interface{}

### Latency
- **Sub-millisecond overhead**: Translation adds <1ms to request time
- **Concurrent processing**: Goroutines handle multiple requests simultaneously
- **Context propagation**: Cancellation immediately stops upstream requests

### Scalability
- **Stateless**: No session state, can scale horizontally
- **Connection pooling**: Reuses HTTP connections to providers
- **Graceful degradation**: Continues serving requests if one provider fails

## Security Model

### API Key Storage
1. **Tier 1 (Preferred)**: OS-native keyring
   - macOS: Keychain with TouchID support
   - Windows: Credential Manager
   - Linux: Secret Service (DBus)

2. **Tier 2 (Fallback)**: Encrypted local file
   - AES encryption with machine-specific salt
   - Warning shown to user

3. **Tier 3 (Development)**: Environment variables
   - Session-only, not persisted
   - Suitable for CI/CD and testing

### Network Security
- **Local-only by default**: Binds to `127.0.0.1` (localhost)
- **HTTPS upstream**: All provider connections use TLS
- **No telemetry**: No data sent to external services

## Next Steps

### For New Users
1. Read [Getting Started Guide](./getting-started/quick-start.md)
2. Review [Installation Options](./getting-started/installation.md)
3. Configure your first provider

### For Developers
1. Review [Component Documentation](./components/)
2. Read [Development Guide](./development/contributing.md)
3. Check [Implementation Roadmap](./development/roadmap.md)

### For Provider Integration
1. Read [Provider Patterns](./providers/patterns/)
2. Follow [Adding Providers Guide](./providers/adding-providers.md)
3. Review existing adapter implementations

## Related Documentation

- **Architecture Deep-Dives**: [architecture/](./architecture/)
- **Component Details**: [components/](./components/)
- **Provider Guides**: [providers/](./providers/)
- **Development**: [development/](./development/)
- **Research**: [research/](./research/)
