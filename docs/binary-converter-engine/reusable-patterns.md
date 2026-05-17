# Reusable Provider Patterns

You are exactly right—most LLM providers follow one of a few common "patterns." By grouping providers into **Pattern Families**, we can reuse the same translator logic for dozens of different backends.

## 1. The Three Major Patterns

In the current LLM ecosystem, there are three dominant patterns that the Binary Converter Engine must support:

### Pattern A: The OpenAI-Compatible Pattern
- **Providers:** DeepSeek, Kimi, Nvidia NIM, Fireworks, Doubleword, Groq, Together AI.
- **Characteristics:** 
  - Uses `POST /v1/chat/completions`.
  - Uses `messages` with `role` and `content`.
  - Streaming uses standard OpenAI SSE chunks.
- **Reuse Strategy:** We build **one** `OpenAIAdapter`. We simply pass it a different `BaseURL` and `APIKey` for each provider.

### Pattern B: The Anthropic-Native Pattern
- **Providers:** Anthropic (Official), OpenRouter, KiroCC Gateway.
- **Characteristics:**
  - Uses `POST /v1/messages`.
  - Uses `anthropic-version` headers.
  - Native support for `tool_use` and `tool_result` blocks.
- **Reuse Strategy:** This is a "Passthrough" pattern. The engine performs minimal translation (like usage normalization) but keeps the JSON structure as-is.

### Pattern C: The AWS / EventStream Pattern
- **Providers:** Kiro, Amazon Bedrock.
- **Characteristics:**
  - Uses a binary-framed "EventStream" protocol over HTTP.
  - Requires specific signature V4 signing (for Bedrock).
  - Different field names (e.g., `maxTokens` vs `max_tokens`).
- **Reuse Strategy:** A specialized adapter that handles binary framing and the specific AWS message format.

---

## 2. Implementing Pattern Reuse in Go

Instead of writing a new file for every provider, we use a **Configuration-Driven** approach.

```go
// internal/providers/openai/base.go

type OpenAIConfig struct {
    BaseURL   string
    ModelMap  map[string]string
    AuthType  string // "Bearer" or "X-API-Key"
}

// We implement the translator logic ONCE here
func (a *OpenAIAdapter) StreamChat(...) { 
    // Uses a.Config.BaseURL
}
```

Then, in our Registry, we just instantiate the same code with different configs:

```go
// internal/providers/registry.go

var Registry = map[string]Provider{
    "deepseek": openai.NewAdapter(openai.DeepSeekConfig),
    "kimi":     openai.NewAdapter(openai.KimiConfig),
    "nvidia":   openai.NewAdapter(openai.NvidiaConfig),
}
```

---

## 3. Why This is "Best Practice"

1.  **Lower Maintenance:** If OpenAI updates their API format, we only fix it in **one** place (the Pattern A translator) and all 10+ OpenAI-compatible providers are fixed instantly.
2.  **Fast Expansion:** Adding a new provider like "Groq" takes 30 seconds—just add their URL to the config map.
3.  **Consistency:** All providers in the same "Pattern Family" will have identical behavior for complex features like streaming and error handling.

---

## 5. The "Pattern-First" Philosophy

Instead of asking "How do I support DeepSeek?", we ask **"Which pattern does DeepSeek follow?"**

### Pattern Selection Flowchart
1.  **Does it use `/v1/chat/completions`?** → Use **Pattern A** (OpenAI).
2.  **Does it use `/v1/messages` and Anthropic headers?** → Use **Pattern B** (Anthropic).
3.  **Does it use AWS EventStream or binary framing?** → Use **Pattern C** (AWS).
4.  **None of the above?** → Only then do we create a unique **Custom Pattern**.
5.  **Pattern D (Gemini Bridge):** [DRAFT] A specialized session-hijacking pattern for free access via official CLIs.

---

## 6. 3-Step Guide to Adding a New Provider

Because of this reusable architecture, adding a new provider (e.g., "Groq") no longer requires writing a "Binary Converter." It only requires a **Configuration Entry**.

### Step 1: Define the Config
```go
var GroqConfig = ProviderConfig{
    Name:        "groq",
    BaseURL:     "https://api.groq.com/openai",
    DefaultModel: "llama-3.3-70b-versatile",
    Pattern:     PatternOpenAI, // Reuse Pattern A
}
```

### Step 2: Register it
```go
// Just one line in internal/providers/registry.go
Registry["groq"] = factory.Create(GroqConfig)
```

### Step 3: Use it
```bash
ghostcli --provider groq
```

---

## 8. Real-World Example: Matching Kiro

Let's see how the engine handles your **Kiro API Key** using this philosophy.

### Step 1: Pattern Identification
Kiro does **not** use the standard OpenAI JSON format. It uses the **AWS EventStream** protocol (binary framing). Therefore, the engine matches Kiro to **Pattern C**.

### Step 2: Configuration Mapping
The engine uses a pre-defined configuration for the Kiro pattern:
- **Auth Type:** Uses the `ksk_` key in a custom `X-Kiro-Key` or `Authorization` header.
- **Base URL:** `https://api.kiro.dev` (or the local `kirocc` gateway).
- **Translator Pair:** It selects the **Pattern C Encoder** (which handles binary framing) and the **Pattern C Decoder** (which parses the AWS-style response).

### Step 3: The Runtime "Match"
When you run the tool:
```bash
# The engine sees "--provider kiro"
# It looks up "kiro" in the Registry Map.
# It finds: "kiro" -> Pattern C Adapter
ghostcli --provider kiro --api-key ksk_12345
```

### Why this is powerful:
If another provider (like **Amazon Bedrock**) also uses the EventStream protocol, we don't write a new engine. We just say: *"Bedrock is also Pattern C,"* and your Kiro translation code is instantly reused for Amazon's official Claude models.
