# The "Translator Pair" Architecture

You are correct—every provider needs more than one "translator." Specifically, each provider adapter acts as a **Symmetric Translator Pair**.

## 1. The Two Directions of Translation

For any provider (like Kiro, DeepSeek, or OpenAI), the adapter must handle two distinct translation tasks:

### Direction A: The Request Encoder (Outbound)
**Flow:** `UnifiedChatRequest` → `Provider Native JSON`
- **When:** When the engine is about to send the request to the provider's API.
- **Task:** Map our internal binary struct to the provider's specific JSON schema (e.g., changing `messages` to `prompt`, or `tools` to `functions`).

### Direction B: The Response Decoder (Inbound)
**Flow:** `Provider Native Stream` → `UnifiedStreamEvent`
- **When:** As the provider's API streams back chunks of text or tool calls.
- **Task:** Take the provider's raw SSE data and "unwrap" it into our `UnifiedStreamEvent` format so the engine can understand it.

---

## 2. Example: The Kiro Translator Pair

| Side | Logic | Example |
| :--- | :--- | :--- |
| **Encoder** | Maps Unified to Kiro/AWS format. | Changes `max_tokens` to `maxTokensToSample`. |
| **Decoder** | Maps Kiro SSE to Unified events. | Takes `{"completion": "Hello"}` and creates a `EventContentDelta`. |

---

## 3. Why This "Pair" is Encapsulated

By keeping both translators inside a single **Provider Adapter**, we gain several benefits:

1.  **Internal Consistency:** The logic for how a provider *asks* for a tool and how it *responds* with a tool is kept in one place.
2.  **Clean Core Engine:** The core engine doesn't need to know that Kiro has two translators. It just calls `provider.StreamChat()`, and the adapter handles the internal encoding and decoding automatically.
3.  **Model-Specific Tweaks:** If a provider has two different models that need slightly different JSON formats, that complexity is hidden inside the adapter's pair.

---

## 4. Visualizing the Full Loop

```text
[ Claude Code ] 
      ↓ (Anthropic JSON)
[ Core: AnthropicIn Parser ]
      ↓ (Unified Struct)
[ Provider Adapter ]
    ├── Encoder (Unified → Provider JSON)  ──→ [ Provider API ]
    └── Decoder (Provider SSE → Unified Event) ←── [ Provider API ]
      ↓ (Unified Event Channel)
[ Core: AnthropicOut Formatter ]
      ↓ (Anthropic SSE)
[ Claude Code ]
```

Each "Provider Adapter" in the diagram is that **Translator Pair** you mentioned.
