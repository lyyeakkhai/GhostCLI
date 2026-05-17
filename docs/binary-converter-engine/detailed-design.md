# Binary Converter Engine: Detailed Architecture & Best Practices

The **Binary Converter Engine** is the heart of the DeepClaude Go proxy. It is responsible for the bidirectional translation between the Anthropic Messages protocol and various provider-specific formats (e.g., OpenAI, AWS, Kiro).

## 1. Core Objectives
- **Zero-Latency Passthrough:** Minimize time-to-first-token (TTFT) by processing streams as they arrive.
- **Memory Efficiency:** Use stream-based processing to handle large payloads (long chat histories) without OOM risks.
- **Robustness:** Gracefully handle malformed upstream responses and missing fields (Usage Normalization).

---

## 2. Component Breakdown

### A. The Inbound Parser (`AnthropicIn`)
This component converts the raw HTTP request body from Claude Code into the `UnifiedChatRequest` struct.

**Best Practices:**
- Use `json.NewDecoder(r.Body)` to avoid `ioutil.ReadAll`.
- Validate the `anthropic-version` header to ensure protocol compatibility.
- Strip or normalize "thinking" blocks if the target provider doesn't support them.

### B. The Outbound Formatter (`AnthropicOut`)
This component converts the internal `UnifiedStreamEvent` channel back into the Anthropic-compatible SSE (`text/event-stream`) format.

**Best Practices:**
- Use a `bufio.Writer` for efficient SSE chunking.
- **Usage Tracking:** Maintain an internal counter for `input_tokens` and `output_tokens`. If the provider omits usage in an event, the engine must inject the last known (or calculated) values to prevent Claude Code crashes.
- Ensure proper `\n\n` termination for every SSE event.

### C. The Provider Router
A registry of available providers that selects the appropriate adapter based on the request.

**Best Practices:**
- Use a `Map` of `string -> Provider` interface for O(1) lookup.
- Support dynamic model remapping (e.g., `claude-3-7-sonnet` -> `deepseek-v4-pro`).

---

## 3. The "Converter" Pipeline (Pseudo-Code)

```go
func (e *Engine) HandleRequest(w http.ResponseWriter, r *http.Request) {
    // 1. Inbound Translation
    var unifiedReq types.UnifiedChatRequest
    if err := json.NewDecoder(r.Body).Decode(&unifiedReq); err != nil {
        http.Error(w, "Invalid Anthropic Request", 400)
        return
    }

    // 2. Route to Provider
    provider := e.Router.Get(unifiedReq.Model)
    
    // 3. Execution (Channel-based)
    eventChan, err := provider.StreamChat(r.Context(), &unifiedReq)
    if err != nil {
        http.Error(w, "Provider Error", 502)
        return
    }

    // 4. Outbound Translation (Streaming)
    w.Header().Set("Content-Type", "text/event-stream")
    for event := range eventChan {
        sseData := e.Formatter.ToAnthropicSSE(event)
        fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sseData.Type, sseData.Json)
        w.(http.Flusher).Flush()
    }
}
```

---

## 4. Performance & Best Practices

### Context Propagation
Always pass `r.Context()` to the provider. This ensures that if the user cancels the request (e.g., `Ctrl+C` in Claude Code), the connection to the upstream LLM provider is immediately severed, saving tokens and costs.

### Error Handling
- **API Errors:** Distinguish between 4xx (client error) and 5xx (provider error).
- **Streaming Errors:** If the stream breaks mid-way, emit an `error` type SSE event to Claude Code so it can fail gracefully instead of hanging.

### Tool Call Handling
Tool calls require precise JSON accumulation. The engine should provide a `ToolBuffer` helper to help adapters collect `partial_json` chunks before they are finalized, although the `UnifiedStreamEvent` should ideally pass these through as they arrive to maintain the "streaming" feel.

---

## 6. Networking & Socket Management

When the engine "listens on a port," it is performing low-level socket operations. In Go, this is abstracted but follows standard POSIX socket behavior.

### The "Listening Socket" (Server Side)
- **What it is:** A **TCP Listening Socket**.
- **Role:** It stays open for the entire duration the engine is running. It binds to an IP/Port (e.g., `127.0.0.1:3200`) and waits for Claude Code to knock.
- **Go Implementation:**
  ```go
  listener, err := net.Listen("tcp", "127.0.0.1:3200")
  // The 'listener' object is your primary Listening Socket.
  ```

### The "Connection Socket" (Per Request)
- **What it is:** An **Ephemeral Socket**.
- **Role:** Every time Claude Code sends a request, the Listening Socket "Accepts" the connection and spawns a new, temporary socket. This socket is used *only* for that single chat session.
- **Go Implementation:**
  ```go
  // http.Serve handles the 'Accept' loop automatically
  http.Serve(listener, handler) 
  ```

### Socket Lifecycle in the Engine
1. **Startup:** Engine opens the **Listening Socket** on `:3200`.
2. **Claude Code Request:** A **Connection Socket** is created.
3. **Data Exchange:** The engine reads the Anthropic JSON and writes back the SSE stream over this connection socket.
4. **Completion:** Once the stream ends (or the user cancels), the **Connection Socket** is closed, but the **Listening Socket** remains open for the next request.
