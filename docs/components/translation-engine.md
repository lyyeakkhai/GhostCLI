# Translation Engine

> **Component**: Core | **Layer**: 3 (Translation) | **Related**: [provider-adapters.md](./provider-adapters.md), [http-server.md](./http-server.md)

The **Translation Engine** is the heart of GhostCLI. It performs bidirectional translation between the Anthropic Messages API protocol and the internal Unified Protocol, enabling seamless communication with multiple LLM providers.

## Overview

```
┌──────────────────────────────────────────────────────────┐
│              Translation Engine                          │
│                                                          │
│  ┌────────────────┐         ┌────────────────┐          │
│  │  AnthropicIn   │         │  AnthropicOut  │          │
│  │    (Parser)    │         │  (Formatter)   │          │
│  └────────┬───────┘         └────────▲───────┘          │
│           │                          │                   │
│           ▼                          │                   │
│  ┌─────────────────────────────────────────────┐        │
│  │       Unified Protocol Layer                │        │
│  │  (UnifiedChatRequest / UnifiedStreamEvent)  │        │
│  └─────────────────────────────────────────────┘        │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

## Core Objectives

1. **Zero-Latency Passthrough**: Minimize time-to-first-token (TTFT) by processing streams as they arrive
2. **Memory Efficiency**: Use stream-based processing to handle large payloads without OOM risks
3. **Robustness**: Gracefully handle malformed responses and missing fields
4. **Consistency**: Normalize provider differences into a unified format

## Components

### 1. AnthropicIn (Inbound Parser)

**Purpose**: Convert incoming Anthropic JSON requests into the internal `UnifiedChatRequest` format.

**Input**: HTTP request body (Anthropic Messages API format)
**Output**: `UnifiedChatRequest` struct

#### Implementation

```go
package engine

import (
    "encoding/json"
    "net/http"
)

// AnthropicIn parses incoming Anthropic requests
type AnthropicIn struct{}

func (p *AnthropicIn) Parse(r *http.Request) (*UnifiedChatRequest, error) {
    // Validate anthropic-version header
    version := r.Header.Get("anthropic-version")
    if version == "" {
        return nil, errors.New("missing anthropic-version header")
    }
    
    // Zero-buffer parsing
    var anthropicReq AnthropicRequest
    if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
        return nil, fmt.Errorf("invalid JSON: %w", err)
    }
    
    // Convert to unified format
    return p.toUnified(&anthropicReq), nil
}

func (p *AnthropicIn) toUnified(req *AnthropicRequest) *UnifiedChatRequest {
    return &UnifiedChatRequest{
        Model:       req.Model,
        Messages:    p.convertMessages(req.Messages),
        System:      p.normalizeSystem(req.System),
        MaxTokens:   req.MaxTokens,
        Temperature: req.Temperature,
        Stream:      req.Stream,
        Tools:       p.convertTools(req.Tools),
    }
}
```

#### Field Mapping

| Anthropic Field | Unified Field | Transformation |
|----------------|---------------|----------------|
| `model` | `Model` | Direct copy (remapped later by adapter) |
| `messages` | `Messages` | Content block normalization |
| `system` | `System` | Array → string concatenation |
| `max_tokens` | `MaxTokens` | Direct copy (default: 4096) |
| `temperature` | `Temperature` | Direct copy |
| `stream` | `Stream` | Direct copy (usually `true`) |
| `tools` | `Tools` | Schema normalization |

#### System Prompt Normalization

Anthropic allows `system` to be either a string or an array of content blocks:

```go
func (p *AnthropicIn) normalizeSystem(system interface{}) string {
    switch v := system.(type) {
    case string:
        return v
    case []interface{}:
        var parts []string
        for _, block := range v {
            if m, ok := block.(map[string]interface{}); ok {
                if m["type"] == "text" {
                    parts = append(parts, m["text"].(string))
                }
            }
        }
        return strings.Join(parts, "\n")
    default:
        return ""
    }
}
```

#### Message Content Normalization

Messages can contain multiple content blocks (text, tool_use, tool_result):

```go
func (p *AnthropicIn) convertMessages(messages []AnthropicMessage) []UnifiedMessage {
    var unified []UnifiedMessage
    
    for _, msg := range messages {
        um := UnifiedMessage{
            Role: msg.Role,
        }
        
        // Handle content (string or array)
        switch content := msg.Content.(type) {
        case string:
            um.Content = content
            
        case []interface{}:
            var textParts []string
            var toolUses []UnifiedToolUse
            var toolResults []UnifiedToolResult
            
            for _, block := range content {
                b := block.(map[string]interface{})
                
                switch b["type"] {
                case "text":
                    textParts = append(textParts, b["text"].(string))
                    
                case "tool_use":
                    toolUses = append(toolUses, UnifiedToolUse{
                        ID:    b["id"].(string),
                        Name:  b["name"].(string),
                        Input: b["input"],
                    })
                    
                case "tool_result":
                    toolResults = append(toolResults, UnifiedToolResult{
                        ToolUseID: b["tool_use_id"].(string),
                        Content:   b["content"].(string),
                    })
                }
            }
            
            um.Content = strings.Join(textParts, "\n")
            um.ToolUses = toolUses
            um.ToolResults = toolResults
        }
        
        unified = append(unified, um)
    }
    
    return unified
}
```

#### Tool Definition Conversion

```go
func (p *AnthropicIn) convertTools(tools []AnthropicTool) []UnifiedTool {
    var unified []UnifiedTool
    
    for _, tool := range tools {
        unified = append(unified, UnifiedTool{
            Name:        tool.Name,
            Description: tool.Description,
            InputSchema: tool.InputSchema,  // Pass through as-is
        })
    }
    
    return unified
}
```

### 2. AnthropicOut (Outbound Formatter)

**Purpose**: Convert internal `UnifiedStreamEvent` objects into Anthropic SSE format.

**Input**: Channel of `UnifiedStreamEvent`
**Output**: HTTP response stream (SSE)

#### Implementation

```go
type AnthropicOut struct {
    usageTracker *UsageTracker
}

func (f *AnthropicOut) Stream(w http.ResponseWriter, events <-chan UnifiedStreamEvent) error {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    flusher := w.(http.Flusher)
    
    // Send message_start event
    f.writeEvent(w, "message_start", f.createMessageStart())
    flusher.Flush()
    
    contentIndex := 0
    
    for event := range events {
        switch event.Type {
        case "content_start":
            f.writeEvent(w, "content_block_start", map[string]interface{}{
                "type":          "content_block_start",
                "index":         contentIndex,
                "content_block": map[string]interface{}{"type": "text", "text": ""},
            })
            
        case "content_delta":
            f.usageTracker.Update(event)
            f.writeEvent(w, "content_block_delta", map[string]interface{}{
                "type":  "content_block_delta",
                "index": contentIndex,
                "delta": map[string]interface{}{"type": "text_delta", "text": event.Content},
            })
            
        case "content_stop":
            f.writeEvent(w, "content_block_stop", map[string]interface{}{
                "type":  "content_block_stop",
                "index": contentIndex,
            })
            contentIndex++
            
        case "tool_call":
            f.writeEvent(w, "content_block_delta", map[string]interface{}{
                "type":  "content_block_delta",
                "index": contentIndex,
                "delta": map[string]interface{}{
                    "type":         "input_json_delta",
                    "partial_json": event.ToolCall.PartialJSON,
                },
            })
            
        case "usage":
            f.usageTracker.Update(event)
            
        case "stop":
            // Send message_delta with usage
            f.writeEvent(w, "message_delta", map[string]interface{}{
                "type": "message_delta",
                "delta": map[string]interface{}{
                    "stop_reason":   event.StopReason,
                    "stop_sequence": nil,
                },
                "usage": f.usageTracker.GetCurrent(),
            })
            
            // Send message_stop
            f.writeEvent(w, "message_stop", map[string]interface{}{
                "type": "message_stop",
            })
        }
        
        flusher.Flush()
    }
    
    return nil
}

func (f *AnthropicOut) writeEvent(w http.ResponseWriter, eventType string, data interface{}) {
    jsonData, _ := json.Marshal(data)
    fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
}
```

#### Usage Tracking

**The Problem**: Claude Code requires usage information in every response. Providers send usage at different times (or not at all).

**The Solution**: Track usage across all events and inject it into the final `message_delta` event.

```go
type UsageTracker struct {
    inputTokens  int
    outputTokens int
    mu           sync.Mutex
}

func (t *UsageTracker) Update(event UnifiedStreamEvent) {
    t.mu.Lock()
    defer t.mu.Unlock()
    
    if event.Usage != nil {
        if event.Usage.InputTokens > 0 {
            t.inputTokens = event.Usage.InputTokens
        }
        if event.Usage.OutputTokens > 0 {
            t.outputTokens = event.Usage.OutputTokens
        }
    }
}

func (t *UsageTracker) GetCurrent() map[string]int {
    t.mu.Lock()
    defer t.mu.Unlock()
    
    return map[string]int{
        "input_tokens":  t.inputTokens,
        "output_tokens": t.outputTokens,
    }
}
```

### 3. Unified Protocol Types

The internal data structures that decouple Anthropic from providers.

#### UnifiedChatRequest

```go
type UnifiedChatRequest struct {
    Model       string           `json:"model"`
    Messages    []UnifiedMessage `json:"messages"`
    System      string           `json:"system,omitempty"`
    MaxTokens   int              `json:"max_tokens"`
    Temperature float64          `json:"temperature"`
    Stream      bool             `json:"stream"`
    Tools       []UnifiedTool    `json:"tools,omitempty"`
}

type UnifiedMessage struct {
    Role        string              `json:"role"`
    Content     string              `json:"content"`
    ToolUses    []UnifiedToolUse    `json:"tool_uses,omitempty"`
    ToolResults []UnifiedToolResult `json:"tool_results,omitempty"`
}

type UnifiedToolUse struct {
    ID    string      `json:"id"`
    Name  string      `json:"name"`
    Input interface{} `json:"input"`
}

type UnifiedToolResult struct {
    ToolUseID string `json:"tool_use_id"`
    Content   string `json:"content"`
}

type UnifiedTool struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    InputSchema interface{} `json:"input_schema"`
}
```

#### UnifiedStreamEvent

```go
type UnifiedStreamEvent struct {
    Type       string     `json:"type"`  // "content_start", "content_delta", "tool_call", "usage", "stop"
    Content    string     `json:"content,omitempty"`
    ToolCall   *ToolCall  `json:"tool_call,omitempty"`
    Usage      *UsageInfo `json:"usage,omitempty"`
    StopReason string     `json:"stop_reason,omitempty"`
}

type ToolCall struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    PartialJSON string `json:"partial_json"`
}

type UsageInfo struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}
```

## Translation Pipeline

### Request Flow

```
HTTP Request (Anthropic JSON)
    │
    ├─▶ json.NewDecoder(r.Body)
    │   └─▶ AnthropicRequest struct
    │
    ├─▶ AnthropicIn.Parse()
    │   ├─▶ Validate headers
    │   ├─▶ Normalize system prompt
    │   ├─▶ Convert messages
    │   └─▶ Convert tools
    │
    └─▶ UnifiedChatRequest
        └─▶ Passed to Provider Adapter
```

### Response Flow

```
Provider Adapter
    │
    ├─▶ Decode provider SSE
    │   └─▶ Provider-specific event
    │
    ├─▶ Map to UnifiedStreamEvent
    │   ├─▶ Extract content delta
    │   ├─▶ Parse tool calls
    │   └─▶ Normalize usage
    │
    ├─▶ Send to event channel
    │
    └─▶ AnthropicOut.Stream()
        ├─▶ Format as Anthropic SSE
        ├─▶ Track usage
        └─▶ Write to HTTP response
```

## Performance Optimizations

### Zero-Buffer Parsing

```go
// ❌ Bad: Loads entire body into memory
body, _ := ioutil.ReadAll(r.Body)
var req AnthropicRequest
json.Unmarshal(body, &req)

// ✅ Good: Streams directly to struct
var req AnthropicRequest
json.NewDecoder(r.Body).Decode(&req)
```

**Benefits**:
- No intermediate string allocation
- Constant memory usage
- Handles large chat histories (100+ messages)

### Streaming Response

```go
// ✅ Constant memory, immediate delivery
for event := range eventChannel {
    sseData := formatter.ToSSE(event)
    fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sseData.Type, sseData.JSON)
    w.(http.Flusher).Flush()  // Critical: flush immediately
}
```

**Benefits**:
- O(1) memory usage (not O(n))
- Immediate token delivery (low TTFT)
- Handles infinite streams

### Struct-Based Processing

```go
// ✅ Binary structs (fast)
type UnifiedChatRequest struct {
    Model    string
    Messages []UnifiedMessage
}

// ❌ Map-based (slow)
type Request map[string]interface{}
```

**Benefits**:
- Type safety at compile time
- No reflection overhead
- Better CPU cache locality

## Error Handling

### Parsing Errors

```go
func (p *AnthropicIn) Parse(r *http.Request) (*UnifiedChatRequest, error) {
    var req AnthropicRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        return nil, &ParseError{
            Type:    "invalid_request_error",
            Message: fmt.Sprintf("Invalid JSON: %v", err),
        }
    }
    
    // Validate required fields
    if req.Model == "" {
        return nil, &ParseError{
            Type:    "invalid_request_error",
            Message: "Missing required field: model",
        }
    }
    
    return p.toUnified(&req), nil
}
```

### Streaming Errors

```go
func (f *AnthropicOut) Stream(w http.ResponseWriter, events <-chan UnifiedStreamEvent) error {
    for event := range events {
        if event.Type == "error" {
            f.writeEvent(w, "error", map[string]interface{}{
                "type": "error",
                "error": map[string]interface{}{
                    "type":    "api_error",
                    "message": event.Content,
                },
            })
            return errors.New(event.Content)
        }
        
        // Normal event processing...
    }
    return nil
}
```

## Testing

### Unit Tests

```go
func TestAnthropicIn_Parse(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *UnifiedChatRequest
        wantErr bool
    }{
        {
            name: "simple request",
            input: `{
                "model": "claude-3-7-sonnet",
                "messages": [{"role": "user", "content": "Hello"}],
                "max_tokens": 1024
            }`,
            want: &UnifiedChatRequest{
                Model:     "claude-3-7-sonnet",
                Messages:  []UnifiedMessage{{Role: "user", Content: "Hello"}},
                MaxTokens: 1024,
            },
        },
        {
            name:    "invalid JSON",
            input:   `{invalid}`,
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(tt.input))
            req.Header.Set("anthropic-version", "2023-06-01")
            
            parser := &AnthropicIn{}
            got, err := parser.Parse(req)
            
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.want, got)
            }
        })
    }
}
```

### Integration Tests

```go
func TestTranslationPipeline(t *testing.T) {
    // Create test server
    events := make(chan UnifiedStreamEvent, 10)
    events <- UnifiedStreamEvent{Type: "content_start"}
    events <- UnifiedStreamEvent{Type: "content_delta", Content: "Hello"}
    events <- UnifiedStreamEvent{Type: "content_stop"}
    events <- UnifiedStreamEvent{Type: "stop", StopReason: "end_turn"}
    close(events)
    
    // Format as Anthropic SSE
    recorder := httptest.NewRecorder()
    formatter := &AnthropicOut{usageTracker: &UsageTracker{}}
    err := formatter.Stream(recorder, events)
    
    assert.NoError(t, err)
    assert.Contains(t, recorder.Body.String(), "event: content_block_delta")
    assert.Contains(t, recorder.Body.String(), "Hello")
}
```

## Related Documentation

- [Provider Adapters](./provider-adapters.md) - Provider-specific translation
- [HTTP Server](./http-server.md) - HTTP routing and middleware
- [Communication Protocol](../architecture/communication-protocol.md) - Protocol details
- [Architecture Overview](../architecture/overview.md) - System architecture
