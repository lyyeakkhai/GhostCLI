# GhostCLI (DeepClaude-Go) Context

## Project Overview
**GhostCLI** (also referred to as **DeepClaude-Go**) is a high-performance, open-source proxy designed to connect **Claude Code** to various LLM providers (e.g., DeepSeek, OpenAI, Kimi). 
The project is currently in the **planning and architectural design phase** and is being rewritten from Node.js to **Go (Golang)** to achieve better performance, zero-buffer streaming, and easy cross-platform distribution as a single static binary.

The core innovation is a **Unified Protocol Architecture**. It acts as an abstraction layer:
1. **Ingestion:** An HTTP server receives Anthropic API requests (from Claude Code) on `/v1/messages`.
2. **Translation to Unified:** Uses zero-buffer parsing to decode JSON directly into a `UnifiedChatRequest` Go struct.
3. **Routing:** A provider map selects the correct adapter based on the configured mode.
4. **Translation to Provider:** The adapter encodes the unified request into the target provider's native format and initiates an HTTP stream.
5. **Formatting to Output:** The proxy normalizes incoming provider streams into a `UnifiedStreamEvent` channel, which is then formatted back into Anthropic SSE format for Claude Code.

## Directory Overview & Key Files
The current repository primarily contains architectural specifications, implementation plans, and research.

*   `/docs/`: Technical specifications and architectural designs.
    *   `architecture.md`: High-level vision and component diagram.
    *   `new-architecture.md`: Communication protocols and HTTP/JSON ingestion details.
    *   `/binary-converter-engine/`: Core logic specifications (Parser, Formatter, Router, translation logic).
*   `/planning/`: Implementation roadmaps.
    *   `plan.md`: The master implementation roadmap, project philosophy, and planned folder structure.
*   `/research/`: Protocol and connection research.
    *   `claudecode.md`: Breakdown of the Claude Code tool's protocol.

The **planned** code directory structure will include:
*   `/cmd/deepclaude/`: CLI layer, entry point.
*   `/internal/`: Encapsulated logic (`app`, `api`, `engine`, `providers`, `telemetry`).

## Building and Running
*Note: The code implementation is currently pending based on the roadmap.*

**For Users (Planned):**
*   **Via NPM (Wrapper):** `npm install -g deepclaude`
*   **Run:** `deepclaude --mode deepseek` (Default port: 3200)
*   **Integration with Claude Code:**
    ```bash
    export ANTHROPIC_BASE_URL=http://localhost:3200
    export DEEPSEEK_API_KEY=sk-...
    claude
    ```

**For Developers (Planned):**
*   **Prerequisites:** Go (Golang) 1.22+
*   **Commands:** `make build`, `make test` (Makefile to be provided).

## Development Conventions & Philosophy
1.  **Strong OOP & SOLID Principles:** Strict adherence to Single Responsibility, Open/Closed, Liskov Substitution, Interface Segregation, and Dependency Inversion.
2.  **Modular Architecture:** Extensive use of Dependency Injection (DI) and Factory Patterns (e.g., `ProviderFactory`).
3.  **Encapsulation:** Strict use of Go's `internal/` directory to hide implementation details.
4.  **Performance:** Go-native, zero-buffer parsing (e.g., using `json.NewDecoder`), streaming-first.
5.  **Cross-Platform Strategy:** Unified codebase using standard Go packages (`os`, `path/filepath`, `net/http`) to ensure static binaries work flawlessly on Windows, macOS, and Linux. CI/CD automated via GitHub Actions and GoReleaser.
