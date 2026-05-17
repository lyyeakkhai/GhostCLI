# Communication Protocol

> **Parent**: [overview.md](./overview.md) | **Related**: [data-flow.md](./data-flow.md)

This document details the HTTP and SSE (Server-Sent Events) protocols used by GhostCLI for communication with Claude Code and LLM providers.

## Protocol Overview

GhostCLI implements the **Anthropic Messages API** protocol on the client-facing side and translates to various provider protocols on the backend.

```
Claude Code ←─ Anthropic Protocol ─→ GhostCLI ←─ Provider Protocols ─→ LLM APIs
```

## Anthropic Messages API (Client-Facing)

### Endpoint

```
POST /v1/messages
Host: localhost:3200
Content-Type: application/json
anthropic-version: 2023-06-01
```

### Request Format

```json
{
  "model": "claude-3-7-sonnet-20250219",
  "max_tokens": 4096,
  "temperature": 1.0,
  "stream": true,
  "system": "You are a helpful assistant.",
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "tools": [
    {
      "name": "read_file",
      "description": "Read a file from disk",
      "input_schema": {
        "type": "object",
        "properties": {
          "path": {"type": "string"}
        },
        "required": ["path"]
      }
    }
  ]
}
```

### Response Format (Streaming)

GhostCLI returns Server-Sent Events (SSE) with the following event types:

#### 1. Message Start
```
event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-3-7-sonnet-20250219","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}
```

#### 2. Content Block Start
```
event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
```

#### 3. Content Block Delta (Text)
```
event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}
```

#### 4. Content Block Delta (Tool Use)
```
event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}
```

#### 5. Content Block Stop
```
event: content_block_stop
data: {"type":"content_block_stop","index":0}
```

#### 6. Message Delta (Usage Update)
```
event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":15}}
```

#### 7. Message Stop
```
event: message_stop
data: {"type":"message_stop"}
```

### Error Response

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid model: claude-3-7-sonnet"
  }
}
```

## HTTP Protocol Details

### Request Lifecycle

```
1. Client (Claude Code) opens connection
   │
   ├─▶ TCP handshake (localhost:3200)
   │
   ├─▶ HTTP POST /v1/messages
   │   ├─▶ Headers: Content-Type, anthropic-version
   │   └─▶ Body: JSON request
   │
   ├─▶ Server processes request
   │
   ├─▶ Server responds with SSE stream
   │   ├─▶ Headers: Content-Type: text/event-stream
   │   └─▶ Body: SSE events (chunked transfer)
   │
   └─▶ Connection closes after final event
```

### Zero-Buffer Parsing

GhostCLI uses streaming JSON parsing to avoid loading the entire request into memory:

```go
// ✅ Efficient: Streams directly to struct
func parseRequest(r *http.Request) (*UnifiedChatRequest, error) {
    var req AnthropicRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        return nil, err
    }
    return toUnified(req), nil
}
```

**Benefits**:
- No intermediate string allocation
- Constant memory usage
- Handles large chat histories efficiently

### Header Management

#### Required Headers (Inbound)
```
Content-Type: application/json
anthropic-version: 2023-06-01  (validated)
```

#### Optional Headers (Inbound)
```
Authorization: Bearer sk-...  (captured but not used)
x-api-key: sk-...             (captured but not used)
```

#### Response Headers (Outbound)
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no  (disables nginx buffering)
```

## Server-Sent Events (SSE) Protocol

### SSE Format

Each SSE message follows this structure:

```
event: <event_type>\n
data: <json_payload>\n
\n
```

**Rules**:
- Each field ends with `\n`
- Messages end with `\n\n` (blank line)
- Data must be valid JSON
- Event type is optional (defaults to "message")

### SSE Streaming Implementation

```go
func streamResponse(w http.ResponseWriter, events <-chan UnifiedStreamEvent) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    flusher := w.(http.Flusher)
    
    for event := range events {
        // Format as Anthropic SSE
        sseData := formatAnthropicSSE(event)
        
        // Write event
        fmt.Fprintf(w, "event: %s\n", sseData.Type)
        fmt.Fprintf(w, "data: %s\n\n", sseData.JSON)
        
        // Flush immediately (critical for streaming)
        flusher.Flush()
    }
}
```

**Critical**: `Flush()` must be called after each event to ensure immediate delivery to the client.

## Provider Protocols

GhostCLI translates the Anthropic protocol to various provider-specific protocols.

### Pattern A: OpenAI-Compatible

**Providers**: DeepSeek, Kimi, Nvidia NIM, Groq

**Request Format**:
```json
{
  "model": "deepseek-v4-pro",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello"}
  ],
  "max_tokens": 4096,
  "temperature": 1.0,
  "stream": true,
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "read_file",
        "description": "Read a file",
        "parameters": {
          "type": "object",
          "properties": {"path": {"type": "string"}},
          "required": ["path"]
        }
      }
    }
  ]
}
```

**Response Format** (SSE):
```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]
```

### Pattern B: Anthropic-Native

**Providers**: Anthropic (official), OpenRouter, KiroCC

**Request/Response**: Same as Anthropic Messages API (passthrough)

**Translation**: Minimal (usage normalization only)

### Pattern C: AWS EventStream

**Providers**: Kiro, Amazon Bedrock

**Request Format**:
```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "max_tokens": 4096,
  "messages": [
    {"role": "user", "content": [{"type": "text", "text": "Hello"}]}
  ],
  "temperature": 1.0
}
```

**Response Format**: Binary-framed EventStream

```
[Binary Frame Header]
{
  "type": "content_block_delta",
  "index": 0,
  "delta": {"type": "text_delta", "text": "Hello"}
}
```

## Protocol Translation

### Request Translation Flow

```
Anthropic Request
    │
    ├─▶ Parse JSON → AnthropicRequest struct
    │
    ├─▶ Map to Unified format
    │   ├─▶ Model name mapping
    │   ├─▶ System prompt normalization
    │   ├─▶ Message content flattening
    │   └─▶ Tool definition conversion
    │
    ├─▶ Select provider adapter
    │
    └─▶ Encode to provider format
        ├─▶ OpenAI: messages array
        ├─▶ Anthropic: native format
        └─▶ AWS: EventStream format
```

### Response Translation Flow

```
Provider Response (SSE)
    │
    ├─▶ Parse SSE chunk
    │
    ├─▶ Decode provider-specific JSON
    │
    ├─▶ Map to UnifiedStreamEvent
    │   ├─▶ Content delta extraction
    │   ├─▶ Tool call parsing
    │   └─▶ Usage normalization
    │
    ├─▶ Send to event channel
    │
    └─▶ Format as Anthropic SSE
        └─▶ Stream to Claude Code
```

## Complex Content Handling

### Multi-Part User Messages

Anthropic supports multi-part content in user messages:

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "Here is the file:"},
    {"type": "tool_result", "tool_use_id": "toolu_123", "content": "file contents"}
  ]
}
```

**Translation Strategy**:
- **OpenAI**: Concatenate text blocks, map tool_result to role: "tool"
- **Anthropic**: Pass through as-is
- **AWS**: Convert to native format

### Assistant Tool Calls

Anthropic assistant messages can contain tool calls:

```json
{
  "role": "assistant",
  "content": [
    {"type": "text", "text": "I'll read the file."},
    {
      "type": "tool_use",
      "id": "toolu_456",
      "name": "read_file",
      "input": {"path": "/tmp/file.txt"}
    }
  ]
}
```

**Translation Strategy**:
- **OpenAI**: Map to `tool_calls` array
- **Anthropic**: Pass through as-is
- **AWS**: Convert to native tool format

## Usage Tracking

### The Usage Problem

Claude Code **requires** usage information in every response. If missing, it crashes or hangs.

**Provider Behavior**:
- **OpenAI**: Sends usage only in final chunk
- **Anthropic**: Sends usage in message_delta event
- **AWS**: May omit usage entirely

### GhostCLI Solution

```go
type UsageTracker struct {
    inputTokens  int
    outputTokens int
}

func (t *UsageTracker) Update(event UnifiedStreamEvent) {
    if event.Usage != nil {
        if event.Usage.InputTokens > 0 {
            t.inputTokens = event.Usage.InputTokens
        }
        if event.Usage.OutputTokens > 0 {
            t.outputTokens = event.Usage.OutputTokens
        }
    }
}

func (t *UsageTracker) GetCurrent() *UsageInfo {
    return &UsageInfo{
        InputTokens:  t.inputTokens,
        OutputTokens: t.outputTokens,
    }
}
```

**Strategy**:
1. Track usage across all events
2. Inject last-known usage into every Anthropic event
3. Ensure Claude Code always receives valid usage data

## Context Propagation

### Cancellation Handling

When Claude Code disconnects (Ctrl+C), GhostCLI must immediately stop the upstream request:

```go
func (a *Adapter) StreamChat(ctx context.Context, req *UnifiedChatRequest) (<-chan UnifiedStreamEvent, error) {
    events := make(chan UnifiedStreamEvent)
    
    go func() {
        defer close(events)
        
        // Create provider request with context
        providerReq, _ := http.NewRequestWithContext(ctx, "POST", a.baseURL, body)
        
        resp, err := a.client.Do(providerReq)
        if err != nil {
            // Context cancelled, exit immediately
            return
        }
        
        // Stream response
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            select {
            case <-ctx.Done():
                // Client disconnected, stop reading
                return
            case events <- parseEvent(scanner.Text()):
                // Continue streaming
            }
        }
    }()
    
    return events, nil
}
```

**Benefits**:
- Saves API costs (stops generating tokens)
- Frees resources immediately
- Prevents orphaned requests

## Health Checks

### Readiness Probe

```
GET /health
```

**Response**:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "provider": "deepseek"
}
```

### Liveness Probe

```
GET /ping
```

**Response**:
```
pong
```

## Error Handling

### Client Errors (4xx)

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Missing required field: messages"
  }
}
```

**HTTP Status**: 400 Bad Request

### Provider Errors (5xx)

```json
{
  "type": "error",
  "error": {
    "type": "api_error",
    "message": "Provider API returned 503: Service Unavailable"
  }
}
```

**HTTP Status**: 502 Bad Gateway

### Streaming Errors

If an error occurs mid-stream, send an error event:

```
event: error
data: {"type":"error","error":{"type":"api_error","message":"Connection lost"}}
```

Then close the connection.

## Performance Optimizations

### Connection Pooling

```go
var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
    Timeout: 0,  // No timeout for streaming
}
```

### Buffering Strategy

```go
// Use buffered writer for SSE
writer := bufio.NewWriterSize(w, 4096)
defer writer.Flush()

for event := range events {
    fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event.Type, event.JSON)
    writer.Flush()  // Flush after each event
}
```

## Related Documentation

- [Architecture Overview](./overview.md) - System architecture
- [Data Flow](./data-flow.md) - Detailed request/response flow
- [Translation Engine](../components/translation-engine.md) - Translation logic
- [HTTP Server](../components/http-server.md) - Server implementation
