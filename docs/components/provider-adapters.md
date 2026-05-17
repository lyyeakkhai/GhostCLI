# Provider Adapters

> **Component**: Integration | **Layer**: 4 (Provider Integration) | **Related**: [translation-engine.md](./translation-engine.md)

Provider Adapters are the integration layer that connects GhostCLI to various LLM provider APIs. They implement a **pattern-based architecture** that enables code reuse across providers with similar API structures.

## Overview

```
┌──────────────────────────────────────────────────────────┐
│              Provider Adapter Layer                      │
│                                                          │
│  ┌────────────────────────────────────────────────┐     │
│  │         Provider Interface                     │     │
│  │  StreamChat(ctx, UnifiedRequest) → Events      │     │
│  └────────────────────────────────────────────────┘     │
│                        │                                 │
│         ┌──────────────┼──────────────┐                 │
│         │              │              │                 │
│         ▼              ▼              ▼                 │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐           │
│  │ Pattern A│   │ Pattern B│   │ Pattern C│           │
│  │  OpenAI  │   │Anthropic │   │   AWS    │           │
│  └──────────┘   └──────────┘   └──────────┘           │
│         │              │              │                 │
│         ▼              ▼              ▼                 │
│  DeepSeek, Kimi   Anthropic, Kiro   Bedrock           │
│  Groq, Nvidia     OpenRouter                           │
└──────────────────────────────────────────────────────────┘
```

## Core Concepts

### The Provider Interface

All provider adapters implement a single, simple interface:

```go
type Provider interface {
    StreamChat(ctx context.Context, req *UnifiedChatRequest) (<-chan UnifiedStreamEvent, error)
}
```

**Benefits**:
- **Polymorphism**: Any provider can be swapped at runtime
- **Testability**: Easy to mock for testing
- **Simplicity**: Single method to implement

### The Translator Pair Architecture

Each provider adapter acts as a **Symmetric Translator Pair**:

1. **Encoder** (Outbound): `UnifiedChatRequest` → Provider JSON
2. **Decoder** (Inbound): Provider SSE → `UnifiedStreamEvent`

```
┌─────────────────────────────────────────────────┐
│           Provider Adapter                      │
│                                                 │
│  UnifiedChatRequest                             │
│         │                                       │
│         ▼                                       │
│  ┌──────────────┐                               │
│  │   Encoder    │ → Provider JSON → HTTP POST   │
│  └──────────────┘                               │
│                                                 │
│  ┌──────────────┐                               │
│  │   Decoder    │ ← Provider SSE ← HTTP Stream  │
│  └──────────────┘                               │
│         │                                       │
│         ▼                                       │
│  UnifiedStreamEvent                             │
│                                                 │
└─────────────────────────────────────────────────┘
```

## Provider Patterns

Instead of writing custom code for every provider, GhostCLI groups providers into **Pattern Families** based on their API structure.

### Pattern A: OpenAI-Compatible

**Providers**: DeepSeek, Kimi, Nvidia NIM, Fireworks, Groq, Together AI, Doubleword

**Characteristics**:
- Endpoint: `POST /v1/chat/completions`
- Request format: OpenAI Chat Completions API
- Streaming: Standard OpenAI SSE chunks
- Tool calling: `tools` array with `function` type

**Implementation Strategy**: One `OpenAIAdapter` with different configurations

#### Request Format

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

#### Response Format (SSE)

```
data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}

data: [DONE]
```

#### Encoder Implementation

```go
type OpenAIAdapter struct {
    baseURL      string
    apiKey       string
    modelMap     map[string]string
    httpClient   *http.Client
}

func (a *OpenAIAdapter) Encode(req *UnifiedChatRequest) ([]byte, error) {
    openaiReq := OpenAIRequest{
        Model:       a.mapModel(req.Model),
        Messages:    a.convertMessages(req),
        MaxTokens:   req.MaxTokens,
        Temperature: req.Temperature,
        Stream:      true,
        Tools:       a.convertTools(req.Tools),
    }
    
    return json.Marshal(openaiReq)
}

func (a *OpenAIAdapter) convertMessages(req *UnifiedChatRequest) []OpenAIMessage {
    var messages []OpenAIMessage
    
    // Add system message if present
    if req.System != "" {
        messages = append(messages, OpenAIMessage{
            Role:    "system",
            Content: req.System,
        })
    }
    
    // Convert unified messages
    for _, msg := range req.Messages {
        openaiMsg := OpenAIMessage{
            Role:    msg.Role,
            Content: msg.Content,
        }
        
        // Handle tool calls
        if len(msg.ToolUses) > 0 {
            openaiMsg.ToolCalls = a.convertToolUses(msg.ToolUses)
        }
        
        messages = append(messages, openaiMsg)
        
        // Add tool results as separate messages
        for _, result := range msg.ToolResults {
            messages = append(messages, OpenAIMessage{
                Role:       "tool",
                Content:    result.Content,
                ToolCallID: result.ToolUseID,
            })
        }
    }
    
    return messages
}

func (a *OpenAIAdapter) convertTools(tools []UnifiedTool) []OpenAITool {
    var openaiTools []OpenAITool
    
    for _, tool := range tools {
        openaiTools = append(openaiTools, OpenAITool{
            Type: "function",
            Function: OpenAIFunction{
                Name:        tool.Name,
                Description: tool.Description,
                Parameters:  tool.InputSchema,
            },
        })
    }
    
    return openaiTools
}
```

#### Decoder Implementation

```go
func (a *OpenAIAdapter) Decode(sseChunk string) (*UnifiedStreamEvent, error) {
    // Handle [DONE] marker
    if strings.TrimSpace(sseChunk) == "[DONE]" {
        return &UnifiedStreamEvent{Type: "stop", StopReason: "end_turn"}, nil
    }
    
    // Parse JSON
    var chunk OpenAIStreamChunk
    if err := json.Unmarshal([]byte(sseChunk), &chunk); err != nil {
        return nil, err
    }
    
    if len(chunk.Choices) == 0 {
        return nil, nil
    }
    
    choice := chunk.Choices[0]
    
    // Handle content delta
    if choice.Delta.Content != "" {
        return &UnifiedStreamEvent{
            Type:    "content_delta",
            Content: choice.Delta.Content,
        }, nil
    }
    
    // Handle tool calls
    if len(choice.Delta.ToolCalls) > 0 {
        tc := choice.Delta.ToolCalls[0]
        return &UnifiedStreamEvent{
            Type: "tool_call",
            ToolCall: &ToolCall{
                ID:          tc.ID,
                Name:        tc.Function.Name,
                PartialJSON: tc.Function.Arguments,
            },
        }, nil
    }
    
    // Handle finish
    if choice.FinishReason != "" {
        event := &UnifiedStreamEvent{
            Type:       "stop",
            StopReason: choice.FinishReason,
        }
        
        // Extract usage if present
        if chunk.Usage != nil {
            event.Usage = &UsageInfo{
                InputTokens:  chunk.Usage.PromptTokens,
                OutputTokens: chunk.Usage.CompletionTokens,
            }
        }
        
        return event, nil
    }
    
    return nil, nil
}
```

#### Configuration-Based Instantiation

```go
type ProviderConfig struct {
    Name         string
    BaseURL      string
    DefaultModel string
    ModelMap     map[string]string
}

var DeepSeekConfig = ProviderConfig{
    Name:         "deepseek",
    BaseURL:      "https://api.deepseek.com",
    DefaultModel: "deepseek-v4-pro",
    ModelMap: map[string]string{
        "claude-3-7-sonnet": "deepseek-v4-pro",
        "claude-3-haiku":    "deepseek-v4-lite",
    },
}

var KimiConfig = ProviderConfig{
    Name:         "kimi",
    BaseURL:      "https://api.moonshot.cn",
    DefaultModel: "moonshot-v1-8k",
    ModelMap: map[string]string{
        "claude-3-7-sonnet": "moonshot-v1-8k",
        "claude-3-haiku":    "moonshot-v1-8k",
    },
}

// Factory creates adapters from config
func NewOpenAIAdapter(config ProviderConfig, apiKey string) *OpenAIAdapter {
    return &OpenAIAdapter{
        baseURL:  config.BaseURL,
        apiKey:   apiKey,
        modelMap: config.ModelMap,
        httpClient: &http.Client{
            Timeout: 0,  // No timeout for streaming
        },
    }
}
```

### Pattern B: Anthropic-Native

**Providers**: Anthropic (official), OpenRouter, KiroCC Gateway

**Characteristics**:
- Endpoint: `POST /v1/messages`
- Request format: Native Anthropic Messages API
- Streaming: Anthropic SSE events
- Tool calling: Native `tool_use` and `tool_result` blocks

**Implementation Strategy**: Minimal translation (mostly passthrough)

#### Implementation

```go
type AnthropicAdapter struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func (a *AnthropicAdapter) StreamChat(ctx context.Context, req *UnifiedChatRequest) (<-chan UnifiedStreamEvent, error) {
    // Minimal encoding (mostly passthrough)
    anthropicReq := a.toAnthropicFormat(req)
    
    body, _ := json.Marshal(anthropicReq)
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/messages", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("anthropic-version", "2023-06-01")
    httpReq.Header.Set("x-api-key", a.apiKey)
    
    resp, err := a.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    
    events := make(chan UnifiedStreamEvent)
    
    go func() {
        defer close(events)
        defer resp.Body.Close()
        
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            line := scanner.Text()
            
            // Parse SSE
            if strings.HasPrefix(line, "data: ") {
                data := strings.TrimPrefix(line, "data: ")
                event := a.parseAnthropicEvent(data)
                if event != nil {
                    events <- *event
                }
            }
        }
    }()
    
    return events, nil
}

func (a *AnthropicAdapter) parseAnthropicEvent(data string) *UnifiedStreamEvent {
    var event map[string]interface{}
    if err := json.Unmarshal([]byte(data), &event); err != nil {
        return nil
    }
    
    eventType := event["type"].(string)
    
    switch eventType {
    case "content_block_delta":
        delta := event["delta"].(map[string]interface{})
        if delta["type"] == "text_delta" {
            return &UnifiedStreamEvent{
                Type:    "content_delta",
                Content: delta["text"].(string),
            }
        }
        
    case "message_delta":
        delta := event["delta"].(map[string]interface{})
        usage := event["usage"].(map[string]interface{})
        return &UnifiedStreamEvent{
            Type:       "stop",
            StopReason: delta["stop_reason"].(string),
            Usage: &UsageInfo{
                OutputTokens: int(usage["output_tokens"].(float64)),
            },
        }
    }
    
    return nil
}
```

### Pattern C: AWS EventStream

**Providers**: Kiro, Amazon Bedrock

**Characteristics**:
- Protocol: Binary-framed EventStream over HTTP
- Request format: AWS-specific field names
- Streaming: Binary frames with JSON payloads
- Auth: Signature V4 signing (for Bedrock)

**Implementation Strategy**: Specialized adapter for binary framing

#### Request Format

```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "max_tokens": 4096,
  "messages": [
    {
      "role": "user",
      "content": [{"type": "text", "text": "Hello"}]
    }
  ],
  "temperature": 1.0
}
```

#### Implementation

```go
type AWSAdapter struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func (a *AWSAdapter) Encode(req *UnifiedChatRequest) ([]byte, error) {
    awsReq := map[string]interface{}{
        "anthropic_version": "bedrock-2023-05-31",
        "max_tokens":        req.MaxTokens,
        "messages":          a.convertMessages(req.Messages),
        "temperature":       req.Temperature,
    }
    
    if req.System != "" {
        awsReq["system"] = []map[string]string{
            {"type": "text", "text": req.System},
        }
    }
    
    return json.Marshal(awsReq)
}

func (a *AWSAdapter) Decode(frame []byte) (*UnifiedStreamEvent, error) {
    // Parse binary frame header
    // (Implementation depends on AWS EventStream spec)
    
    // Extract JSON payload
    var event map[string]interface{}
    if err := json.Unmarshal(frame, &event); err != nil {
        return nil, err
    }
    
    // Similar to Anthropic-Native decoding
    return a.parseEvent(event), nil
}
```

## Provider Registry

The **Provider Registry** is a central map that stores all available provider adapters.

```go
package providers

var Registry = make(map[string]Provider)

func init() {
    // Register OpenAI-compatible providers
    Registry["deepseek"] = NewOpenAIAdapter(DeepSeekConfig, os.Getenv("DEEPSEEK_API_KEY"))
    Registry["kimi"] = NewOpenAIAdapter(KimiConfig, os.Getenv("KIMI_API_KEY"))
    Registry["groq"] = NewOpenAIAdapter(GroqConfig, os.Getenv("GROQ_API_KEY"))
    
    // Register Anthropic-native providers
    Registry["anthropic"] = NewAnthropicAdapter(AnthropicConfig, os.Getenv("ANTHROPIC_API_KEY"))
    Registry["kiro"] = NewAnthropicAdapter(KiroConfig, os.Getenv("KIRO_API_KEY"))
    
    // Register AWS providers
    Registry["bedrock"] = NewAWSAdapter(BedrockConfig, os.Getenv("AWS_ACCESS_KEY"))
}

func Get(name string) (Provider, error) {
    provider, ok := Registry[name]
    if !ok {
        return nil, fmt.Errorf("unknown provider: %s", name)
    }
    return provider, nil
}
```

## Provider Router

The **Provider Router** selects the appropriate adapter based on the user's configuration.

```go
type Router struct {
    registry map[string]Provider
    mode     string
}

func NewRouter(mode string) *Router {
    return &Router{
        registry: Registry,
        mode:     mode,
    }
}

func (r *Router) Route(req *UnifiedChatRequest) (Provider, error) {
    provider, ok := r.registry[r.mode]
    if !ok {
        return nil, fmt.Errorf("provider not configured: %s", r.mode)
    }
    return provider, nil
}
```

## Adding New Providers

Thanks to the pattern-based architecture, adding a new provider is simple:

### Step 1: Identify the Pattern

**Question**: Which pattern does the provider follow?
- Uses `/v1/chat/completions`? → **Pattern A** (OpenAI)
- Uses `/v1/messages` with Anthropic format? → **Pattern B** (Anthropic)
- Uses AWS EventStream? → **Pattern C** (AWS)
- None of the above? → Create a new pattern

### Step 2: Define Configuration

```go
var GroqConfig = ProviderConfig{
    Name:         "groq",
    BaseURL:      "https://api.groq.com/openai",
    DefaultModel: "llama-3.3-70b-versatile",
    ModelMap: map[string]string{
        "claude-3-7-sonnet": "llama-3.3-70b-versatile",
        "claude-3-haiku":    "llama-3.1-8b-instant",
    },
}
```

### Step 3: Register in Registry

```go
func init() {
    Registry["groq"] = NewOpenAIAdapter(GroqConfig, os.Getenv("GROQ_API_KEY"))
}
```

### Step 4: Use It

```bash
export GROQ_API_KEY=gsk-...
ghostcli --provider groq
```

**That's it!** No new translation code needed.

## Model Mapping

Each provider has different model names. The adapter maps Anthropic model names to provider-specific names.

```go
func (a *OpenAIAdapter) mapModel(anthropicModel string) string {
    if mapped, ok := a.modelMap[anthropicModel]; ok {
        return mapped
    }
    return a.defaultModel
}
```

**Example**:
- Claude Code sends: `claude-3-7-sonnet-20250219`
- DeepSeek adapter maps to: `deepseek-v4-pro`
- Kimi adapter maps to: `moonshot-v1-8k`

## Error Handling

### Provider API Errors

```go
func (a *OpenAIAdapter) StreamChat(ctx context.Context, req *UnifiedChatRequest) (<-chan UnifiedStreamEvent, error) {
    resp, err := a.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("provider request failed: %w", err)
    }
    
    if resp.StatusCode != 200 {
        body, _ := ioutil.ReadAll(resp.Body)
        return nil, fmt.Errorf("provider error %d: %s", resp.StatusCode, body)
    }
    
    // Continue streaming...
}
```

### Streaming Errors

```go
go func() {
    defer close(events)
    
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            // Client disconnected
            return
        default:
            event, err := a.Decode(scanner.Text())
            if err != nil {
                events <- UnifiedStreamEvent{
                    Type:    "error",
                    Content: err.Error(),
                }
                return
            }
            if event != nil {
                events <- *event
            }
        }
    }
    
    if err := scanner.Err(); err != nil {
        events <- UnifiedStreamEvent{
            Type:    "error",
            Content: fmt.Sprintf("stream error: %v", err),
        }
    }
}()
```

## Testing

### Mock Provider

```go
type MockProvider struct {
    events []UnifiedStreamEvent
}

func (m *MockProvider) StreamChat(ctx context.Context, req *UnifiedChatRequest) (<-chan UnifiedStreamEvent, error) {
    ch := make(chan UnifiedStreamEvent, len(m.events))
    for _, event := range m.events {
        ch <- event
    }
    close(ch)
    return ch, nil
}
```

### Adapter Tests

```go
func TestOpenAIAdapter_Encode(t *testing.T) {
    adapter := NewOpenAIAdapter(DeepSeekConfig, "test-key")
    
    req := &UnifiedChatRequest{
        Model:     "claude-3-7-sonnet",
        Messages:  []UnifiedMessage{{Role: "user", Content: "Hello"}},
        MaxTokens: 1024,
    }
    
    encoded, err := adapter.Encode(req)
    assert.NoError(t, err)
    
    var openaiReq OpenAIRequest
    json.Unmarshal(encoded, &openaiReq)
    
    assert.Equal(t, "deepseek-v4-pro", openaiReq.Model)
    assert.Equal(t, "user", openaiReq.Messages[0].Role)
    assert.Equal(t, "Hello", openaiReq.Messages[0].Content)
}
```

## Performance Considerations

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

### Context Propagation

```go
func (a *OpenAIAdapter) StreamChat(ctx context.Context, req *UnifiedChatRequest) (<-chan UnifiedStreamEvent, error) {
    // Create request with context
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", a.baseURL, body)
    
    // Context cancellation will abort the request
    resp, err := a.httpClient.Do(httpReq)
    
    // ...
}
```

**Benefits**:
- Immediate cancellation when client disconnects
- Prevents wasted API calls
- Saves tokens and costs

## Related Documentation

- [Translation Engine](./translation-engine.md) - Core translation logic
- [Provider Patterns](../providers/patterns/) - Pattern-specific details
- [Adding Providers](../providers/adding-providers.md) - Step-by-step guide
- [Architecture Overview](../architecture/overview.md) - System architecture
