# Mission

## Project Name

**GhostCLI** (also known as **DeepClaude-Go**)

---

## Mission Statement

GhostCLI is a high-performance, open-source Go proxy that transparently bridges **Claude Code** to alternative LLM providers (DeepSeek, Kimi, OpenAI, Kiro), enabling 10× cost savings with zero changes to the user's Claude Code workflow.

---

## Problem We're Solving

Claude Code is tightly coupled to Anthropic's API, making it expensive and inflexible for developers who want to experiment with or save costs on alternative LLM providers. The previous Node.js proxy was slow, buffered requests unnecessarily, and struggled to normalise the varied API formats of different providers. There was no reliable, production-grade, single-binary tool that could act as a drop-in translation layer between Claude Code and any OpenAI-compatible or Anthropic-native provider.

---

## Target Users

| Audience | Need |
|---|---|
| **AI developers / power users** | Want Claude Code's UX with cheaper or alternative LLM backends |
| **Cost-conscious teams** | Need to reduce LLM spend without rewriting tooling |
| **Open-source contributors** | Want to add new provider adapters with minimal boilerplate |
| **DevOps / platform engineers** | Need a reliable, observable, cross-platform proxy binary |

---

## Success Looks Like

- [ ] Single binary distributable on Windows, macOS (Intel + Apple Silicon), and Linux with no runtime dependencies.
- [ ] Claude Code integrates with zero config change beyond `ANTHROPIC_BASE_URL=http://localhost:3200`.
- [ ] Translation engine adds **< 5 ms** of latency to TTFT (time-to-first-token).
- [ ] Supports at minimum: **DeepSeek**, **Kimi**, **OpenAI**, and **Kiro** provider adapters.
- [ ] Full streaming support (SSE) with accurate token-usage normalisation accepted by Claude Code.
- [ ] Interactive first-run wizard with OS-native secure API key storage.
- [ ] NPM wrapper (`npm install -g ghostcli` / `npx ghostcli`) that downloads the correct binary automatically.
- [ ] GitHub Actions CI/CD pipeline runs `go test` on Windows, Ubuntu, and macOS for every PR.

---

## Out of Scope

- **UI / Web dashboard** – GhostCLI is a headless CLI proxy; no browser interface is planned.
- **Cloud-hosted proxy service** – The tool runs locally on the developer's machine only.
- **Model fine-tuning or training** – GhostCLI routes and translates; it does not train models.
- **Direct Anthropic API feature parity for all edge cases** – Advanced Anthropic-only features (e.g., extended computer use, vision-specific APIs) may not be fully supported in every provider adapter.
- **Multi-user / multi-tenant server** – The proxy is designed for a single developer's local environment, not a shared server.
- **Non-Claude Code clients** – While technically usable, only Claude Code compatibility is the primary guarantee.
