# GhostCLI Modular Documentation

This directory contains modular documentation organized by system component/feature. Each module has its own requirements, design, and implementation details.

## Module Structure

```
modules/
├── 01-http-server/          # HTTP server and routing
├── 02-translation-engine/   # Core translation logic
├── 03-provider-adapters/    # Provider integration patterns
├── 04-cli/                  # Command-line interface
├── 05-security/             # API key storage and security
└── 06-observability/        # Logging and metrics
```

## Module Overview

### 01. HTTP Server
**Purpose**: HTTP server initialization, routing, middleware, and health checks

**Key Components**:
- Server initialization and lifecycle
- Request routing (/v1/messages, /health)
- Middleware (CORS, logging, context)
- Graceful shutdown

**Related Requirements**: 1, 13, 22, 28, 29

---

### 02. Translation Engine
**Purpose**: Core translation between Anthropic format and Unified Protocol

**Key Components**:
- UnifiedChatRequest and UnifiedStreamEvent data structures
- AnthropicIn parser (Anthropic → Unified)
- AnthropicOut formatter (Unified → Anthropic SSE)
- Token usage normalization
- Streaming pipeline

**Related Requirements**: 2, 5, 14, 15, 24, 25

---

### 03. Provider Adapters
**Purpose**: Provider-specific integrations using pattern-based architecture

**Key Components**:
- Provider interface definition
- Provider registry and factory
- Pattern A: OpenAI-compatible adapters (DeepSeek, Kimi, OpenAI)
- Pattern B: Anthropic-native adapters
- Pattern C: AWS EventStream adapters (Kiro)
- Model name mapping
- Tool call translation

**Related Requirements**: 3, 4, 6, 7, 8, 19, 20, 27

---

### 04. CLI
**Purpose**: Command-line interface and user interaction

**Key Components**:
- Flag parsing and configuration
- Interactive first-run setup wizard
- Provider selection UI
- API key validation
- Version information
- Configuration file support

**Related Requirements**: 9, 11, 12, 21, 26, 30

---

### 05. Security
**Purpose**: Secure API key storage and credential management

**Key Components**:
- OS-native keyring integration (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- Encrypted file fallback
- Machine UUID-based encryption
- Secure storage interface

**Related Requirements**: 10

---

### 06. Observability
**Purpose**: Logging, metrics, and monitoring

**Key Components**:
- Structured logging with slog
- Performance metrics (TTFT, request duration)
- Error tracking and reporting
- Log levels and verbosity control

**Related Requirements**: 16, 17

---

## Cross-Module Concerns

### Error Handling
All modules follow consistent error handling patterns:
- Configuration errors: Fail fast at startup
- Network errors: Retry with exponential backoff
- Provider errors: Convert to Anthropic error format
- Streaming errors: Emit error events in SSE stream

### Context Propagation
Request context flows through all modules for cancellation support:
HTTP Server → Translation Engine → Provider Adapter

### Performance
All modules optimize for:
- Zero-buffer streaming
- Sub-5ms translation latency
- Immediate SSE flushing
- Connection pooling

---

## Navigation

- [01. HTTP Server](./01-http-server/README.md)
- [02. Translation Engine](./02-translation-engine/README.md)
- [03. Provider Adapters](./03-provider-adapters/README.md)
- [04. CLI](./04-cli/README.md)
- [05. Security](./05-security/README.md)
- [06. Observability](./06-observability/README.md)
