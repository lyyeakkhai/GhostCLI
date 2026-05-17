# Provider Routing & Mapping Strategy

This document explains how the **Binary Converter Engine** maps the user's provider choice to the correct translation logic at runtime.

## 1. The Provider Selection Flow

When a user starts the engine (e.g., via `ghostcli --provider kiro`), the following sequence occurs:

1.  **Initialization:** The engine registers all available **Provider Adapters** (DeepSeek, Kiro, Kimi, etc.) into a central `Registry`.
2.  **Request Ingestion:** The `AnthropicIn` parser converts the inbound Claude Code JSON into the internal `UnifiedChatRequest` struct.
3.  **Router Lookup:** The `ProviderRouter` looks up the adapter corresponding to the user's choice (e.g., `kiro`).
4.  **Translation (Encoder):** The selected adapter's `Encode` method is called. It takes the `UnifiedChatRequest` and transforms it into the specific JSON format required by that provider.
5.  **Execution:** The adapter sends the request to the provider's API.

---

## 2. The Provider Registry (The "Map")

In Go, this is implemented as a thread-safe map of provider IDs to their respective interface implementations.

```go
// internal/providers/registry.go

var Registry = map[string]Provider{
    "deepseek": &deepseek.Adapter{},
    "kiro":     &kiro.Adapter{},
    "kimi":     &kimi.Adapter{},
    "openai":   &openai.Adapter{},
}
```

---

## 3. Dynamic Mapping Example: Kiro

If the user selects **Kiro**, the routing engine follows this logic:

### A. The Unified Request
The `AnthropicIn` parser creates a binary Go struct:
```go
req := &types.UnifiedChatRequest{
    Model: "claude-3-7-sonnet",
    Messages: []UnifiedMessage{...},
    // ...
}
```

### B. The Kiro Translator (Adapter)
The `kiro.Adapter` implements the `Provider` interface. Its job is to take that `UnifiedChatRequest` and format it for the Kiro/AWS protocol.

```go
func (a *KiroAdapter) StreamChat(ctx context.Context, req *types.UnifiedChatRequest) {
    // 1. Map model name if needed
    targetModel := a.MapModel(req.Model) // e.g. "claude-sonnet-4.6"

    // 2. Convert Unified format to Kiro's specific format
    kiroPayload := a.ConvertToKiroFormat(req)

    // 3. Send to Kiro Gateway (kirocc) or API
    // ...
}
```

---

## 4. Why Use This Pattern?

1.  **Isolation (SRP):** The Kiro translator only knows about Kiro. The DeepSeek translator only knows about DeepSeek.
2.  **Extensibility (OCP):** To add a new provider (e.g., "Google Gemini"), you simply create a new `gemini.Adapter` and add one line to the `Registry` map. You don't have to touch the core engine logic.
3.  **Consistency:** Regardless of the provider, the engine's "Front Door" (Anthropic protocol) and "Core" (Unified format) stay exactly the same.

---

## 5. Implementation Best Practices

### Model Aliasing
Since Claude Code sends specific Anthropic model names (like `claude-3-5-sonnet-20241022`), every adapter should have a **Model Map** to translate these to what the specific provider expects.

*Example for DeepSeek:*
- `claude-3-5-sonnet` -> `deepseek-v4-pro`
- `claude-3-haiku` -> `deepseek-v4-lite`

### Fallback Logic
If a user tries to use a provider that isn't in the registry, the engine should fail gracefully at startup with a clear list of supported providers.
