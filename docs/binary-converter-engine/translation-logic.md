# Claude Code JSON Translation Logic

This document details how the **Binary Converter Engine** translates inbound Anthropic JSON objects (from Claude Code) into the internal `UnifiedChatRequest` format, and how it handles complex multi-part content.

## 1. Request Translation (Anthropic → Unified)

When Claude Code sends a request to `/v1/messages`, the engine must map the fields to the `UnifiedChatRequest` struct.

### Field Mapping Table

| Anthropic Field | Unified Field | Notes |
| :--- | :--- | :--- |
| `model` | `Model` | Usually remapped via a provider-specific lookup table. |
| `system` | `System` | Can be a `string` or an `array` of content blocks. |
| `messages` | `Messages` | An array of roles (`user`, `assistant`) and content. |
| `max_tokens` | `MaxTokens` | Defaults to 4096 if not specified. |
| `temperature` | `Temperature` | Passed through directly. |
| `stream` | `Stream` | Usually `true` for Claude Code. |
| `tools` | `Tools` | Normalized into a standard function-calling format. |

---

## 2. Handling Complex Content Blocks

Claude Code often sends `content` as an array rather than a simple string. The engine must iterate through these blocks and flatten or normalize them.

### A. User Messages (Multi-part)
A single user message might contain text and the results of a tool execution.

**Anthropic Format:**
```json
{
  "role": "user",
  "content": [
    { "type": "text", "text": "Here is the file content:" },
    {
      "type": "tool_result",
      "tool_use_id": "toolu_123",
      "content": "Hello World!"
    }
  ]
}
```

**Unified Logic:**
- **Text Blocks:** Concatenated into the main `Content` string or stored as structured parts if the target provider supports multi-part user messages (like OpenAI).
- **Tool Results:** These are extracted and mapped to the provider's specific "tool response" role (e.g., `role: tool` in OpenAI).

### B. Assistant Messages (Tool Use)
Assistant messages often contain a mix of "thinking" text and tool calls.

**Anthropic Format:**
```json
{
  "role": "assistant",
  "content": [
    { "type": "text", "text": "I will check the directory." },
    {
      "type": "tool_use",
      "id": "toolu_456",
      "name": "ls",
      "input": { "path": "." }
    }
  ]
}
```

**Unified Logic:**
- The engine identifies `tool_use` blocks and maps them to the provider's tool call structure.
- Any preceding `text` block is preserved as the assistant's verbal response.

---

## 3. System Prompt Normalization

Anthropic allows the `system` field to be an array of blocks.

**Translation Rule:**
If `system` is an array, the engine filters for all blocks with `type: "text"` and joins them with a newline (`\n`) to create a single string for the `UnifiedChatRequest.System` field.

---

## 4. Tool Definition Translation

Claude Code's tool definitions must be converted into the target provider's schema.

**Anthropic Tool:**
```json
{
  "name": "read_file",
  "description": "Reads a file from disk",
  "input_schema": {
    "type": "object",
    "properties": {
      "path": { "type": "string" }
    },
    "required": ["path"]
  }
}
```

**OpenAI-Compatible Translation:**
The `input_schema` is directly mapped to the `parameters` field of the OpenAI tool structure.

---

## 5. Streaming Event Translation (Provider → Anthropic)

The reverse translation happens as chunks arrive from the provider.

1. **Token Delta:** Map provider's `delta.content` to Anthropic's `content_block_delta` with `type: text_delta`.
2. **Tool Call Delta:** Map provider's `delta.tool_calls.arguments` to Anthropic's `content_block_delta` with `type: input_json_delta`.
3. **Usage:** Ensure `usage` fields are captured from the final provider chunks and injected into the Anthropic `message_delta` event.
