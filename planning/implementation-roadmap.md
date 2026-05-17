# GhostCLI Implementation Roadmap

## Overview
This roadmap outlines the phased implementation strategy for GhostCLI, a high-performance Go-based proxy for Claude Code. It follows the Unified Protocol architecture and the 6-module design outlined in the technical specifications.

## Phase 1: Foundation & Entry Point
**Modules:** CLI (04), HTTP Server (01)
**Goal:** Establish the project skeleton, command-line interface, and the HTTP front door.

*   **1.1 Project Initialization:** Set up the Go module (`go mod init`), standard directory structure (`cmd/`, `internal/`), and core dependencies (e.g., Cobra, Viper).
*   **1.2 CLI Scaffolding:** Implement the basic CLI entry point using Cobra. Define core flags (`--port`, `--mode`, `--api-key`).
*   **1.3 HTTP Server:** Implement the fast HTTP router. Set up the `/v1/messages` endpoint and basic middleware (CORS, request logging).
*   **1.4 Health Checks:** Implement readiness and liveness probe endpoints.

## Phase 2: Core Data Types & Translation Engine
**Modules:** Translation Engine (02)
**Goal:** Build the universal translation layer that bridges Anthropic requests and internal formats.

*   **2.1 Unified Protocol Types:** Define the core Go structs: `UnifiedChatRequest` and `UnifiedStreamEvent`.
*   **2.2 Anthropic Ingestion (Parser):** Implement the `AnthropicIn` module using zero-buffer JSON parsing (`json.NewDecoder`) to map incoming Anthropic payloads to `UnifiedChatRequest`.
*   **2.3 Anthropic Egress (Formatter):** Implement the `AnthropicOut` module to convert channels of `UnifiedStreamEvent` into outbound Anthropic SSE HTTP streams, strictly handling token usage counting required by Claude Code.

## Phase 3: The Provider Ecosystem
**Modules:** Provider Adapters (03)
**Goal:** Implement the routing logic and the specific LLM provider integrations.

*   **3.1 Provider Interface & Router:** Define the standard `Provider` interface (`StreamChat`) and implement the Provider Factory/Router for dynamic instantiation based on user config.
*   **3.2 Base Patterns:** Implement reusable adapter patterns, primarily the "OpenAI-Compatible" base struct to minimize redundant code.
*   **3.3 DeepSeek Adapter:** Implement the DeepSeek provider (translating Unified -> DeepSeek API -> Unified stream).
*   **3.4 Kimi Adapter:** Implement the Kimi provider.
*   **3.5 Kiro (Anthropic Native) Adapter:** Implement the pass-through/native Anthropic adapter.

## Phase 4: Security & Observability
**Modules:** Security (05), Observability (06)
**Goal:** Secure credentials and ensure the system is observable and maintainable.

*   **4.1 OS-Native Secret Management:** Implement secure local API key storage using OS keyrings (Keychain, Windows Credential Manager) to ensure no plaintext keys on disk.
*   **4.2 Structured Logging:** Integrate `slog` across all modules for consistent, parseable debug and error logging.
*   **4.3 Metrics:** Add telemetry to track translation latency, provider response times, and streaming performance.

## Phase 5: CLI Polish & Distribution
**Modules:** CLI (04)
**Goal:** Finalize the user experience and prepare for automated cross-platform releases.

*   **5.1 Interactive Onboarding:** Implement the "First-Run" interactive flow for provider selection and initial configuration.
*   **5.2 Configuration Management:** Finalize environment variable parsing and multi-backend YAML/JSON configuration support.
*   **5.3 Release Automation:** Configure GoReleaser and GitHub Actions for automated, cross-platform static binary builds (Windows, macOS, Linux).