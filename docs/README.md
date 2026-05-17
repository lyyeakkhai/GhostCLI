# GhostCLI Architectural Documentation

This directory contains the technical specifications, architectural designs, and implementation plans for the **GhostCLI** project—a high-performance, Go-based proxy for Claude Code.

## 📁 Directory Structure & File Responsibilities

### 🏗️ Core Architecture & Strategy
- **[architecture.md](./architecture.md)**: The high-level vision. Explains the transition from Node.js to Go and the "Unified Protocol" innovation.
- **[new-architecture.md](./new-architecture.md)**: Part 1 of the technical deep-dive, focusing on the communication protocols, HTTP/JSON ingestion, and zero-buffer parsing.

### 📅 Planning & Strategy (Located in `/planning`)
- **[plan.md](../planning/plan.md)**: The master implementation roadmap. Outlines the project philosophy (SOLID/OOP), folder structure, and phased build approach.
- **[gemini-bridge-strategy.md](../planning/gemini-bridge-strategy.md)**: [DRAFT] Design for the "Session Bridge" that leverages existing Google credentials for free model access.

### 🔌 Protocol & Connection Research (Located in `/research`)
- **[claudecode.md](../research/claudecode.md)**: A detailed breakdown of the Claude Code tool's protocol. Explains the client-server relationship, SSE event types, and tool-calling mechanics.

### ⚙️ Binary Converter Engine (Core Logic)
Located in the `binary-converter-engine/` subfolder, these files define the heart of the proxy:
- **[detailed-design.md](./binary-converter-engine/detailed-design.md)**: Technical specifications for the engine components (Parser, Formatter, Router) and low-level socket management.
- **[translation-logic.md](./binary-converter-engine/translation-logic.md)**: The blueprint for mapping Anthropic JSON objects to our internal Unified structures.
- **[provider-routing.md](./binary-converter-engine/provider-routing.md)**: Explains the dynamic registry and routing strategy for different providers.
- **[reusable-patterns.md](./binary-converter-engine/reusable-patterns.md)**: Defines the "Pattern-First" design, grouping providers into reusable families (OpenAI-Compat, Anthropic-Native, etc.).
- **[translator-pairs.md](./binary-converter-engine/translator-pairs.md)**: Details the Symmetric Translator Pair (Encoder/Decoder) architecture required for every provider.

### 🛡️ User Experience & Security
- **[onboarding-flow.md](./onboarding-flow.md)**: Design for the interactive "First-Run" experience, including provider selection and automatic background orchestration.
- **[security-storage.md](./security-storage.md)**: The security specification for local API key storage using OS-native keyrings (Keychain, Windows Credential Manager).

---

## 🛠️ Design Philosophy
1. **Performance:** Go-native, zero-buffer, streaming-first.
2. **Scalability:** Pattern-first design allows adding dozens of providers with minimal code.
3. **Security:** OS-native secret management; no plain-text keys on disk.
4. **UX:** Seamless "one-command" setup and execution.
