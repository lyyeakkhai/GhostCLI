# Architecture Overview

> **Parent**: [ARCHITECTURE.md](../ARCHITECTURE.md) | **Related**: [communication-protocol.md](./communication-protocol.md), [data-flow.md](./data-flow.md)

This document provides a detailed architectural overview of GhostCLI's internal structure and design decisions.

## System Context

GhostCLI sits between Claude Code and various LLM providers, acting as a protocol translator and request router.

```
┌─────────────────────────────────────────────────────────────────┐
│                     User's Development Environment              │
│                                                                 │
│  ┌──────────────┐                                               │
│  │ Claude Code  │ (Anthropic Messages API)                      │
│  │   Client     │                                               │
│  └──────┬───────┘                                               │
│         │                                                        │
│         │ ANTHROPIC_BASE_URL=http://localhost:3200              │
│         │                                                        │
│         ▼                                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    GhostCLI Proxy                       │   │
│  │                  (localhost:3200)                       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                           │
                           │ HTTPS
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│   DeepSeek    │  │     Kimi      │  │     Kiro      │
│  api.deepseek │  │  api.moonshot │  │  api.kiro.dev │
└───────────────┘  └───────────────┘  └───────────────┘
```

## Architectural Layers

GhostCLI is organized into distinct layers, each with clear responsibilities:

### Layer 1: Entry Point (CLI)
**Purpose**: User interaction and application bootstrap

**Responsibilities**:
- Parse command-line flags and arguments
- Load configuration from files and environment
- Interactive onboarding for first-run experience
- Launch HTTP server with configured providers
- Graceful shutdown handling

**Key Components**:
- `cmd/deepclaude/main.go`: Application entry point
- Flag parsing (Cobra)
- Configuration management (Viper)
- Interactive TUI (Bubble Tea)

### Layer 2: Transport (HTTP Server)
**Purpose**: HTTP protocol handling and routing

**Responsibilities**:
- Listen on configured port (default: 3200)
- Route `/v1/messages` to translation engine
- Provide health check endpoints
- CORS and security middleware
- Request/response logging

**Key Components**:
- HTTP router and middleware
- SSE (Server-Sent Events) streaming
- Context propagation for cancellation
- Error handling and status codes

### Layer 3: Translation (Engine)
**Purpose**: Bidirectional protocol translation

**Responsibilities**:
- Parse incoming Anthropic JSON → Unified format
- Route requests to appropriate provider adapter
- Normalize provider responses → Unified format
- Format Unified events → Anthropic SSE
- Usage tracking and normalization

**Key Components**:
- `AnthropicIn`: Inbound parser
- `AnthropicOut`: Outbound formatter
- `UnifiedChatRequest`: Internal request format
- `UnifiedStreamEvent`: Internal event format
- Provider router and registry

### Layer 4: Integration (Provider Adapters)
**Purpose**: Provider-specific API integration

**Responsibilities**:
- Encode Unified requests → Provider format
- Execute HTTP requests to provider APIs
- Decode provider responses → Unified events
- Handle provider-specific quirks
- Model name mapping

**Key Components**:
- Provider interface definition
- Pattern-based adapter implementations
- Provider factory for dynamic instantiation
- Base adapters for pattern reuse

### Layer 5: Cross-Cutting (Telemetry & Security)
**Purpose**: Observability and security concerns

**Responsibilities**:
- Structured logging (slog)
- Metrics collection
- API key storage (OS keyring)
- Configuration encryption
- Error tracking

**Key Components**:
- Logger initialization and context
- Keyring integration
- Metrics exporters
- Security utilities

## Component Interaction

### Startup Sequence

```
1. main()
   │
   ├─▶ Parse CLI flags
   │   └─▶ Load configuration
   │
   ├─▶ Initialize logger
   │
   ├─▶ Load API keys from keyring
   │
   ├─▶ Initialize provider registry
   │   ├─▶ Register DeepSeek adapter
   │   ├─▶ Register Kimi adapter
   │   └─▶ Register Kiro adapter
   │
   ├─▶ Create HTTP server
   │   ├─▶ Register /v1/messages handler
   │   ├─▶ Register /health handler
   │   └─▶ Apply middleware
   │
   └─▶ Start listening on port
       └─▶ Block until shutdown signal
```

### Request Processing Flow

```
HTTP Request
    │
    ├─▶ Middleware Chain
    │   ├─▶ CORS headers
    │   ├─▶ Request logging
    │   └─▶ Context injection
    │
    ├─▶ Route Handler (/v1/messages)
    │   │
    │   ├─▶ AnthropicIn Parser
    │   │   ├─▶ json.NewDecoder(r.Body)
    │   │   ├─▶ Validate anthropic-version
    │   │   └─▶ Create UnifiedChatRequest
    │   │
    │   ├─▶ Provider Router
    │   │   ├─▶ Lookup adapter by mode
    │   │   └─▶ Return Provider interface
    │   │
    │   ├─▶ Provider Adapter
    │   │   ├─▶ Encode: Unified → Provider JSON
    │   │   ├─▶ HTTP POST to provider API
    │   │   ├─▶ Decode: Provider SSE → Unified events
    │   │   └─▶ Return event channel
    │   │
    │   └─▶ AnthropicOut Formatter
    │       ├─▶ Read from event channel
    │       ├─▶ Format as Anthropic SSE
    │       ├─▶ Write to response stream
    │       └─▶ Flush after each event
    │
    └─▶ Response Complete
```

## Data Flow Architecture

### Request Transformation Pipeline

```
┌─────────────────────────────────────────────────────────────┐
│                    Inbound Pipeline                         │
└─────────────────────────────────────────────────────────────┘

Anthropic JSON (bytes)
    │
    │ json.NewDecoder
    ▼
Anthropic Struct (Go)
    │
    │ Field mapping
    ▼
UnifiedChatRequest (Go)
    │
    │ Provider-specific encoding
    ▼
Provider JSON (bytes)
    │
    │ HTTP POST
    ▼
Provider API

┌─────────────────────────────────────────────────────────────┐
│                   Outbound Pipeline                         │
└─────────────────────────────────────────────────────────────┘

Provider API
    │
    │ SSE stream
    ▼
Provider SSE Chunk (bytes)
    │
    │ Provider-specific decoding
    ▼
UnifiedStreamEvent (Go)
    │
    │ Channel send
    ▼
Event Channel (chan UnifiedStreamEvent)
    │
    │ Anthropic formatting
    ▼
Anthropic SSE (bytes)
    │
    │ HTTP response stream
    ▼
Claude Code
```

### Memory Management

**Zero-Buffer Parsing**:
```go
// ❌ Bad: Loads entire body into memory
body, _ := ioutil.ReadAll(r.Body)
var req AnthropicRequest
json.Unmarshal(body, &req)

// ✅ Good: Streams directly to struct
var req AnthropicRequest
json.NewDecoder(r.Body).Decode(&req)
```

**Streaming Response**:
```go
// ✅ Constant memory usage
for event := range eventChannel {
    sseData := formatter.ToSSE(event)
    fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sseData.Type, sseData.JSON)
    w.(http.Flusher).Flush()  // Send immediately
}
```

## Concurrency Model

### Goroutine Usage

```
Main Goroutine
    │
    ├─▶ HTTP Server (goroutine per request)
    │   │
    │   ├─▶ Request Handler
    │   │   │
    │   │   ├─▶ Provider Adapter
    │   │   │   │
    │   │   │   └─▶ HTTP Client (internal goroutines)
    │   │   │
    │   │   └─▶ Event Channel Reader
    │   │
    │   └─▶ Response Writer
    │
    └─▶ Signal Handler (shutdown)
```

### Context Propagation

```go
// Request context flows through entire pipeline
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()  // Carries cancellation signal
    
    // Parse request
    req := parseRequest(r.Body)
    
    // Call provider (context propagates)
    events, err := provider.StreamChat(ctx, req)
    
    // Stream response (context checked)
    for {
        select {
        case event := <-events:
            writeEvent(w, event)
        case <-ctx.Done():
            // Client disconnected, stop immediately
            return
        }
    }
}
```

**Benefits**:
- Immediate cancellation when client disconnects
- Prevents wasted API calls
- Saves tokens and costs
- Clean resource cleanup

## Error Handling Strategy

### Error Categories

1. **Client Errors (4xx)**
   - Invalid JSON format
   - Missing required fields
   - Unsupported model
   - **Action**: Return error to Claude Code immediately

2. **Provider Errors (5xx)**
   - Provider API down
   - Rate limiting
   - Authentication failure
   - **Action**: Return error with provider context

3. **Streaming Errors**
   - Connection interrupted mid-stream
   - Malformed SSE chunk
   - **Action**: Send error event, close stream gracefully

### Error Response Format

```go
// Anthropic error format
{
    "type": "error",
    "error": {
        "type": "invalid_request_error",
        "message": "Invalid model: claude-3-7-sonnet"
    }
}
```

## Configuration Management

### Configuration Sources (Priority Order)

1. **Command-line flags** (highest priority)
   ```bash
   ghostcli --provider deepseek --port 3200
   ```

2. **Environment variables**
   ```bash
   export GHOSTCLI_PROVIDER=deepseek
   export DEEPSEEK_API_KEY=sk-...
   ```

3. **Configuration file** (lowest priority)
   ```yaml
   # ~/.config/ghostcli/config.yaml
   provider: deepseek
   port: 3200
   ```

### Multi-Provider Configuration

```yaml
providers:
  deepseek:
    api_key: sk-...
    base_url: https://api.deepseek.com
    default_model: deepseek-v4-pro
  
  kimi:
    api_key: sk-...
    base_url: https://api.moonshot.cn
    default_model: moonshot-v1-8k
```

## Extensibility Points

### Adding New Components

1. **New Provider Pattern**
   - Implement `Provider` interface
   - Add pattern to factory
   - Register in provider registry

2. **New Middleware**
   - Implement `http.Handler` wrapper
   - Add to middleware chain
   - Configure via flags/config

3. **New Metrics**
   - Add metric definition
   - Instrument relevant code paths
   - Export via telemetry layer

4. **New CLI Commands**
   - Add Cobra command
   - Implement command logic
   - Register in root command

## Testing Strategy

### Unit Tests
- Test each component in isolation
- Mock dependencies (providers, HTTP clients)
- Focus on transformation logic

### Integration Tests
- Test component interactions
- Use test providers (echo servers)
- Verify end-to-end flow

### Performance Tests
- Benchmark translation overhead
- Measure memory usage
- Test concurrent request handling

## Deployment Architecture

### Single Binary Deployment

```
User Machine
    │
    ├─▶ ghostcli binary (static, no dependencies)
    │   └─▶ Listens on localhost:3200
    │
    ├─▶ Configuration
    │   ├─▶ ~/.config/ghostcli/config.yaml
    │   └─▶ OS Keyring (API keys)
    │
    └─▶ Claude Code
        └─▶ ANTHROPIC_BASE_URL=http://localhost:3200
```

**Benefits**:
- No installation dependencies
- Works offline (after initial setup)
- No external services required
- Complete user control

## Security Architecture

### Threat Model

**In Scope**:
- API key theft from disk
- Man-in-the-middle attacks (upstream)
- Unauthorized access to proxy

**Out of Scope**:
- Malicious Claude Code client
- Compromised OS keyring
- Physical access to machine

### Security Controls

1. **API Key Protection**
   - OS keyring storage (encrypted at rest)
   - Never logged or printed
   - Memory cleared after use

2. **Network Security**
   - Bind to localhost only (no external access)
   - HTTPS for all upstream connections
   - Certificate validation

3. **Input Validation**
   - JSON schema validation
   - Model name whitelist
   - Request size limits

## Performance Characteristics

### Latency Budget

```
Total Request Time: ~1000ms (typical)
├─▶ GhostCLI overhead: <1ms
│   ├─▶ JSON parsing: ~0.2ms
│   ├─▶ Translation: ~0.3ms
│   └─▶ Routing: ~0.1ms
├─▶ Network (to provider): ~50ms
├─▶ Provider processing: ~900ms
└─▶ Streaming back: ~50ms
```

### Memory Usage

```
Base memory: ~10MB (binary + runtime)
Per request: ~1KB (struct overhead)
Streaming: O(1) (constant, not O(n))
```

### Throughput

- **Concurrent requests**: Limited by provider rate limits
- **Single request**: Streaming starts immediately
- **Bottleneck**: Provider API, not GhostCLI

## Related Documentation

- [Communication Protocol](./communication-protocol.md) - HTTP/SSE protocol details
- [Translation Engine](../components/translation-engine.md) - Translation logic
- [Provider Adapters](../components/provider-adapters.md) - Provider integration
- [Data Flow](./data-flow.md) - Detailed request/response flow
