# Claude Code: Protocol & Connection Details

This document provides a detailed breakdown of how **Claude Code** (Anthropic's CLI agent) communicates with its backend. Understanding this protocol is essential for building proxies, adapters, or custom backends that can "brain-swap" Claude Code.

## 1. Connection Architecture

Claude Code is designed to talk to the Anthropic Messages API. It acts as the **HTTP Client**, and your engine (the proxy) acts as the **HTTP Server**.

### Is it a Socket?
- **Technically Yes:** All HTTP communication happens over TCP sockets.
- **Protocol-wise No:** You do **not** need to implement a raw socket protocol or a custom binary protocol. Claude Code uses standard **RESTful HTTP/1.1 or HTTP/2**.
- **WebSockets:** For the core model API (`/v1/messages`), WebSockets are **not** used. However, Claude Code uses a WebSocket bridge for its `remote-control` feature (connecting the CLI to a browser), but the model engine itself only sees incoming HTTP requests.

### The Interception Flow
1. **Engine Starts:** The Go engine starts an HTTP server and binds to a local port (e.g., `127.0.0.1:3200`).
2. **User sets environment variable:** `export ANTHROPIC_BASE_URL=http://localhost:3200`
3. **Claude Code launches:** It initializes and reads the base URL.
4. **Model Requests (The "Trigger"):** Whenever Claude Code needs to think, it **opens a connection** to your server. It sends a `POST /v1/messages` request.
5. **Streaming (Long-lived HTTP):** If the request asks for `stream: true`, your engine keeps that specific HTTP connection open and sends data in chunks (SSE) until the response is complete, then closes that connection.

---

## 2. Request Format (Inbound to Proxy)

Claude Code sends standard Anthropic Messages API requests.

### Endpoint
`POST /v1/messages`

### Essential Headers
- `x-api-key`: The API key (passed from `ANTHROPIC_AUTH_TOKEN`).
- `anthropic-version`: Usually `2023-06-01`.
- `content-type`: `application/json`.
- `anthropic-beta`: May include beta features like `prompt-caching-2024-07-31` or `max-tokens-3-5-sonnet-2024-07-15`.

### Request Body Structure
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "messages": [
    {
      "role": "user",
      "content": "List the files in this directory."
    }
  ],
  "system": "You are a helpful coding assistant...",
  "max_tokens": 4096,
  "stream": true,
  "tools": [
    {
      "name": "ls",
      "description": "List files in a directory",
      "input_schema": {
        "type": "object",
        "properties": {
          "path": { "type": "string" }
        }
      }
    }
  ]
}
```

---

## 3. Response Format (Outbound to Claude Code)

### Non-Streaming (JSON)
Used for small responses or when `stream: false` is set.
```json
{
  "id": "msg_01X97p7sg94DpxvNycSTmXfR",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "I will list the files for you."
    },
    {
      "type": "tool_use",
      "id": "toolu_01A09z...",
      "name": "ls",
      "input": { "path": "." }
    }
  ],
  "model": "claude-3-5-sonnet-20241022",
  "stop_reason": "tool_use",
  "usage": {
    "input_tokens": 150,
    "output_tokens": 50
  }
}
```

### Streaming (Server-Sent Events)
Claude Code heavily relies on streaming for its "live thinking" UX. The proxy must emit these events in order:

| Event Type | Data Payload (JSON) | Description |
|---|---|---|
| `message_start` | `{"type":"message_start","message":{...}}` | Contains the message ID, role, and initial usage. |
| `content_block_start` | `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` | Signals the start of a new content block. |
| `content_block_delta` | `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"..."}}` | Streamed text chunks. |
| `content_block_delta` | `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"..."}}` | Streamed tool input JSON chunks. |
| `content_block_stop` | `{"type":"content_block_stop","index":0}` | Signals the end of a block. |
| `message_delta` | `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":N}}` | Final status and usage. |
| `message_stop` | `{"type":"message_stop"}` | Terminating event. |

---

## 4. Tool Use Protocol

This is the most critical part of Claude Code's functionality.

1. **Request:** Claude Code sends `tools` in the request body.
2. **Assistant Call:** The model returns a `tool_use` block (either in JSON or via `input_json_delta` stream).
3. **Execution:** Claude Code receives the `tool_use`, executes the tool locally (e.g., runs `ls`), and gathers the result.
4. **Follow-up:** Claude Code sends a *new* request to the proxy. The `messages` array now includes the previous `assistant` message (containing the `tool_use`) followed by a `user` message containing a `tool_result` block.

### `tool_result` Example
```json
{
  "role": "user",
  "content": [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_01A09z...",
      "content": "README.md\nsrc/\npackage.json"
    }
  ]
}
```

---

## 5. Proxy Implementation Gotchas

### Usage Normalization
Claude Code is very sensitive to the `usage` field. Some backends (like DeepSeek or OpenRouter) might omit `input_tokens` in the `message_start` event or `output_tokens` in the `message_delta` event. If these are missing, Claude Code may crash with a "$.input_tokens is undefined" error. The proxy should inject `{ "input_tokens": 0, "output_tokens": 0 }` if they are missing.

### Thinking Blocks
Some providers (like Doubleword or DeepSeek) may return "thinking" or "reasoning" content. If these are sent as a special block type (like `thinking`), they should be stripped or converted to `text` blocks if Claude Code doesn't expect them, as they might cause parsing errors in the CLI.

### Content-Type
Even when streaming, the initial response headers must be correct:
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`
