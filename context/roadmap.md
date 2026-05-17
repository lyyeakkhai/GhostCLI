# Roadmap

---

## Vision

GhostCLI becomes the **de-facto open-source bridge** between Claude Code (and similar Anthropic-protocol clients) and the broader LLM ecosystem. Any developer anywhere in the world can run `npx ghostcli --mode deepseek` and immediately use their preferred model at a fraction of the cost — with zero proprietary infrastructure.

Long-term, GhostCLI evolves into a composable proxy standard: a plug-and-play layer that any CLI AI tool can target, not just Claude Code.

---

## Phase 1 — Foundation

**Goal:** Establish the project skeleton, core data contracts, and basic HTTP front door.

| # | Task | Status |
|---|---|---|
| 1.1 | Go module init + folder structure (`cmd/`, `internal/`, `pkg/`, `docs/`) | ✅ Done |
| 1.2 | Core protocol types: `UnifiedChatRequest`, `UnifiedStreamEvent` | ✅ Done |
| 1.3 | Configuration management (YAML/JSON, env vars, CLI flags, merging) | 🔄 In Progress |
| 1.4 | Secure API key storage (OS Keyring + encrypted file fallback) | 🔄 In Progress |
| 1.5 | Basic HTTP server on `/v1/messages` + `/health` | ⏳ Queued |

---

## Phase 2 — Core Engine & Provider Ecosystem

**Goal:** Build the universal translation pipeline and initial provider adapters.

| # | Task | Status |
|---|---|---|
| 2.1 | `AnthropicIn` parser (zero-buffer streaming JSON → `UnifiedChatRequest`) | 🔄 In Progress |
| 2.2 | `AnthropicOut` formatter (SSE output + token usage normalisation) | 🔄 In Progress |
| 2.3 | Provider interface + thread-safe registry + factory | ⏳ Queued |
| 2.4 | Base adapter patterns (OpenAI-compatible, Anthropic-native, AWS EventStream) | ⏳ Queued |
| 2.5 | DeepSeek adapter (`api.deepseek.com`, OpenAI format) | ⏳ Queued |
| 2.6 | Kimi adapter (`api.moonshot.cn`, OpenAI format) | ⏳ Queued |
| 2.7 | OpenAI adapter (`api.openai.com`) | ⏳ Queued |
| 2.8 | Kiro adapter (Anthropic-native / AWS EventStream) | ⏳ Queued |
| 2.9 | Middleware: CORS, request logging, context propagation, timeout | ⏳ Queued |
| 2.10 | Tool-call support (end-to-end) | ⏳ Queued |
| 2.11 | Thinking-block support | ⏳ Queued |

---

## Phase 3 — Scale / Polish

**Goal:** Harden the system, complete observability, and ship distribution artifacts.

| # | Task | Status |
|---|---|---|
| 3.1 | Structured logging with `slog` across all modules | ⏳ Queued |
| 3.2 | TTFT + request duration metrics | ⏳ Queued |
| 3.3 | Interactive first-run wizard (provider selection, masked key input) | ⏳ Queued |
| 3.4 | `--version` with semver + Git commit + build date | ⏳ Queued |
| 3.5 | `--clear-keys` command with confirmation prompt | ⏳ Queued |
| 3.6 | HTTP client connection pooling + TCP reuse | ⏳ Queued |
| 3.7 | GitHub Actions CI (go test on Windows, Ubuntu, macOS) | ⏳ Queued |
| 3.8 | GoReleaser config (static binaries: win/mac/linux, amd64/arm64) | ⏳ Queued |
| 3.9 | NPM wrapper package (`npm install -g ghostcli`, checksum verified) | ⏳ Queued |
| 3.10 | Homebrew / Scoop distribution | ⏳ Queued |

---

## Icebox

> Ideas worth considering but not committed to any phase.

- **Web UI Dashboard** — Real-time stream viewer, token usage graphs, provider switcher.
- **Multi-backend round-robin / fallback** — If DeepSeek fails, automatically retry on Kimi.
- **Request caching** — Optional semantic cache layer to reduce redundant API calls.
- **Plugin system** — Allow third-party adapter packages outside `internal/`.
- **Gemini adapter** — Google Gemini via OpenAI-compatible endpoint.
- **AWS Bedrock adapter** — Route to Claude on Bedrock via AWS EventStream.
- **Rate limiting middleware** — Protect against accidental runaway usage.
- **TUI status dashboard** — `ghostcli status` shows live stream metrics in the terminal.

---

## Decisions Made

| Decision | Rationale | Date |
|---|---|---|
| **Rewrite from Node.js → Go** | Static binary distribution, zero-buffer streaming, superior cross-platform performance | Pre-project |
| **Unified Protocol Architecture** | Decouples the Anthropic front-door from provider back-ends; adding a new provider = one interface implementation, no core changes | Pre-project |
| **`internal/` for all core packages** | Enforces encapsulation; prevents external consumers from depending on implementation details | Pre-project |
| **Use `json.NewDecoder` (streaming)** | Avoids buffering entire request bodies; critical for low-latency streaming proxy | Pre-project |
| **Kiro uses Anthropic-native / AWS adapter** | Kiro exposes an Anthropic-compatible gateway; AWS EventStream used for Bedrock-style passthrough | 2026-05 |
| **Default port 3200** | Avoids collision with common dev servers (3000, 8080, 8000) | Pre-project |
| **OS Keyring + encrypted file fallback for secrets** | No plaintext keys on disk; uses machine UUID-derived key as fallback encryption | 2026-05 |
| **NPM wrapper as primary install UX** | Matches familiarity of `npm install -g @anthropic-ai/claude-code` for the target audience | Pre-project |
