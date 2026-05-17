# DeepClaude-Go Project Plan

This document outlines the implementation strategy and best-practice project structure for the new production-grade DeepClaude-Go tool.

## 1. Project Philosophy & Engineering Standards

To ensure a production-grade, reliable, and open-source tool, we adhere to the following architectural standards:

### Strong OOP & SOLID Principles
- **Single Responsibility (SRP):** Each package has one job (e.g., `translators` only translates, `providers` only communicates with APIs).
- **Open/Closed (OCP):** The system is open for new providers but closed for modification of the core engine. You add a provider by implementing an interface, not by changing the router.
- **Liskov Substitution (LSP):** Any implementation of the `Provider` interface must be swappable without breaking the engine.
- **Interface Segregation (ISP):** We define small, focused interfaces to ensure components only depend on the methods they actually use.
- **Dependency Inversion (DIP):** High-level modules (the Server) depend on abstractions (the `Provider` interface), not on concrete implementations (e.g., a specific DeepSeek struct).

### Modular Architecture
- **Dependency Injection (DI):** We inject dependencies (loggers, configurations, adapters) during initialization to make the code highly testable.
- **Factory Pattern:** We use a `ProviderFactory` to dynamically instantiate the correct adapter based on user configuration.
- **Package-Based Encapsulation:** We strictly use Go's `internal/` directory to prevent leaking implementation details and enforce a clean public API.

## 2. Modular Folder Structure

```text
deepclaude-go/
├── cmd/
│   └── deepclaude/          # CLI Layer: Entry point, Flag parsing, App Bootstrap
├── internal/                
│   ├── app/                 # Orchestration Layer: Connects API, Engine, and Providers
│   ├── api/                 # Transport Layer: HTTP, Middleware, Request Validation
│   ├── engine/              # Logic Layer: The Translator Engine (Core)
│   │   ├── protocol/        # Internal Unified Protocol definitions
│   │   └── pipeline/        # Transformation and Streaming pipelines
│   ├── providers/           # Integration Layer: All external LLM adapters
│   │   ├── base/            # Base abstractions and shared adapter logic
│   │   ├── factory/         # ProviderFactory for dynamic instantiation
│   │   ├── deepseek/        
│   │   └── kimi/            
│   └── telemetry/           # Cross-cutting concerns: Logging, Metrics, Tracing
├── pkg/                     # (Optional) Reusable utilities for the community
└── docs/                    # Design docs, Diagrams, API specs
```

## 3. Implementation Roadmap

### Phase 1: Foundation
- [ ] Initialize Go module and folder structure.
- [ ] Define `internal/types` (The Unified Protocol).
- [ ] Implement `internal/api` basic server (routing and health checks).

### Phase 2: The Engine
- [ ] Implement `internal/engine` Anthropic parser (Inbound).
- [ ] Implement `internal/engine` SSE Formatter (Outbound).
- [ ] Build the Provider Map and Interface.

### Phase 3: Primary Adapters
- [ ] Implement DeepSeek Adapter (OpenAI format).
- [ ] Implement Kiro Adapter (Native Anthropic format).
- [ ] Implement Kimi Adapter (OpenAI format).

### Phase 4: CLI & Production
- [ ] CLI flag parsing (Port, Mode, API Keys).
- [ ] Structured logging (`slog`).
- [ ] Binary release automation (GoReleaser).

## 4. User Experience (CLI Design)

The goal is to make the tool "invisible" and easy to use with Claude Code.

### Installation & Distribution Strategy (The "One-Command" Experience)

To match the ease of use of `claude` or `gemini`, we will provide a **Global NPM Package** wrapper. This is the recommended path for most users.

1.  **The "Seamless" Path (Recommended):**
    *   **Install once:** `npm install -g deepclaude`
    *   **Run anytime:** `deepclaude --mode deepseek`
    *   *How it works:* The NPM package is a small wrapper that detects the user's OS (Windows, Mac, Linux) and architecture (Intel/M1) and automatically downloads the correct Go binary from our GitHub Releases.

2.  **The "On-Demand" Path (No Install):**
    *   `npx deepclaude --mode kimi`
    *   *This is perfect for users who want to try the tool without a permanent installation.*

3.  **The "System Native" Path (Package Managers):**
    *   **macOS:** `brew install deepclaude`
    *   **Windows:** `scoop install deepclaude`
    *   **Linux:** `curl -sSL https://deepclaude.io/install.sh | sh`

4.  **GitHub Releases (Manual):**
    *   We will use **GoReleaser** to automatically build and upload `.zip` and `.tar.gz` files for Windows (amd64/arm64), macOS (Apple Silicon/Intel), and Linux for every tagged release.

### Simple Usage
The simplest way to start the proxy:
```bash
# Provide API key via env var and start on default port 3200
export DEEPSEEK_API_KEY=sk-...
deepclaude --mode deepseek
```

### Zero-Config Integration with Claude Code
To make it work with Claude Code, the user simply sets one environment variable:
```bash
export ANTHROPIC_BASE_URL=http://localhost:3200
claude
```

### Advanced CLI Flags
```text
Flags:
  --port, -p      Port to listen on (default: 3200)
  --mode, -m      Target provider (deepseek, kimi, kiro, openai)
  --api-key, -k   API key for the selected provider
  --config, -c    Path to a YAML/JSON config file for multiple backends
  --verbose, -v   Enable debug logging
```

### Interactive Feedback
... (rest of the section)

## 5. Repository Prerequisites (What to Install)

When a user visits your repository, their requirements depend on their goal:

### Path A: "I just want to use it" (The User)
**Prerequisites: NONE.**
- They do **not** need to install Go.
- They do **not** need to install Node.js.
- **Action:** They simply go to the "Releases" page, download the binary for their OS, and run it.
- *This is the "Single Binary" advantage of Go.*

### Path B: "I want to build/develop it" (The Contributor)
**Prerequisites:**
1.  **Go (Golang) 1.22 or higher:** Required to compile the source code.
2.  **Git:** To clone the repository.
3.  **Make (Optional):** We will provide a `Makefile` for common tasks like `make build` or `make test`.

### Path C: "The Integrated Experience" (The Main Goal)
**Prerequisites:**
1.  **Claude Code:** This is the *reason* the tool exists. It must be installed globally on the user's system:
    `npm install -g @anthropic-ai/claude-code`
2.  **The DeepClaude Binary:** Actively running in a terminal.
3.  **Environment Variable:** `ANTHROPIC_BASE_URL` set to point at our proxy.

**Note:** Our tool acts as a "Companion" or "Bridge" for Claude Code. Without Claude Code, this tool is a standalone LLM proxy, but its true power is unlocked when used as the backend for Claude's terminal interface.

## 6. Cross-Platform Strategy (Windows, macOS, Linux)

Go is one of the best languages for cross-platform support because it compiles to **static binaries** with no external dependencies (like DLLs or Shared Libraries).

### 1. Unified Codebase
- We will avoid OS-specific APIs. Instead, we use Go's standard `os`, `path/filepath`, and `net/http` packages, which work identically on Windows, Linux, and macOS.
- **Paths:** We use `filepath.Join()` to handle the difference between Windows backslashes (`\`) and Unix forward slashes (`/`).

### 2. Automated Multi-Platform Builds (CI/CD)
We will use **GitHub Actions + GoReleaser**. Every time you push a version tag (e.g., `v1.0.0`), the system will automatically:
- **Build for Windows:** `GOOS=windows GOARCH=amd64` (produces `deepclaude.exe`)
- **Build for macOS:** `GOOS=darwin GOARCH=arm64` (for M1/M2/M3) and `amd64` (for Intel).
- **Build for Linux:** `GOOS=linux GOARCH=amd64`.

### 3. Native Experience on Each OS
- **Windows:** The binary will be a single `.exe`. We will support `.env` files and standard Windows environment variables.
- **macOS:** We will package the binary for easy installation via Homebrew.
- **Linux:** We will provide static binaries that run on any distribution (Ubuntu, Arch, Fedora, etc.) without needing extra libraries.

### 4. Cross-Platform Testing
- Our GitHub Actions pipeline will run `go test` on **Windows, Ubuntu, and macOS runners** to ensure that every pull request works on all three operating systems before it is merged.

---
*Next Step: Finalize the Provider Interface design in Phase 2 planning.*
