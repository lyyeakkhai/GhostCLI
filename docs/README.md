# GhostCLI Documentation

> **Start Here**: Read [ARCHITECTURE.md](./ARCHITECTURE.md) for the complete system overview.

This directory contains the technical documentation for **GhostCLI**—a high-performance, Go-based proxy that connects Claude Code to various LLM providers.

## 📖 Documentation Structure

### 🏗️ Top-Level Architecture
**Start with this document to understand the entire system:**
- **[ARCHITECTURE.md](./ARCHITECTURE.md)** - Complete system architecture, design principles, and component overview

### 📐 Architecture Deep-Dives
**Detailed architectural documentation:**
- [architecture/overview.md](./architecture/overview.md) - Detailed component interactions and data flow
- [architecture/communication-protocol.md](./architecture/communication-protocol.md) - HTTP/SSE protocol specifications

### 🔧 Component Documentation
**Implementation details for each component:**
- [components/translation-engine.md](./components/translation-engine.md) - Core translation logic (Anthropic ↔ Unified Protocol)
- [components/provider-adapters.md](./components/provider-adapters.md) - Provider integration patterns
- [components/http-server.md](./components/http-server.md) - HTTP server and routing
- [components/cli.md](./components/cli.md) - Command-line interface and configuration
- [components/security.md](./components/security.md) - API key storage and security
- [components/observability.md](./components/observability.md) - Logging and metrics

### 🔌 Provider Integration
**Guides for working with providers:**
- [providers/patterns/](./providers/patterns/) - Provider pattern specifications
- [providers/adding-providers.md](./providers/adding-providers.md) - Step-by-step guide to add new providers

### 🔬 Research & Reference
**Protocol research and analysis:**
- [research/claudecode.md](./research/claudecode.md) - Claude Code protocol analysis

### 🚀 Development
**Development and contribution guides:**
- [development/contributing.md](./development/contributing.md) - Contribution guidelines
- [development/roadmap.md](./development/roadmap.md) - Implementation roadmap
- [development/testing.md](./development/testing.md) - Testing strategy

---

## 🎯 Quick Navigation

### For New Users
1. Read [ARCHITECTURE.md](./ARCHITECTURE.md) - Understand what GhostCLI is and how it works
2. Check [Getting Started](./getting-started/) - Installation and setup guides
3. Review [CLI Documentation](./components/cli.md) - Learn command-line usage

### For Developers
1. Read [ARCHITECTURE.md](./ARCHITECTURE.md) - System overview
2. Review [Component Documentation](./components/) - Understand each component
3. Check [Development Guide](./development/contributing.md) - Contribution workflow
4. See [Implementation Roadmap](./development/roadmap.md) - Current status and next steps

### For Provider Integration
1. Read [Provider Adapters](./components/provider-adapters.md) - Understand adapter architecture
2. Review [Provider Patterns](./providers/patterns/) - Identify which pattern to use
3. Follow [Adding Providers Guide](./providers/adding-providers.md) - Step-by-step implementation

---

## 🛠️ Design Philosophy

1. **Performance**: Go-native, zero-buffer parsing, streaming-first architecture
2. **Scalability**: Pattern-based design enables adding providers with minimal code
3. **Security**: OS-native secret management, no plain-text keys on disk
4. **User Experience**: One-command setup, seamless Claude Code integration

---

## 📂 Directory Overview

```
docs/
├── ARCHITECTURE.md              # 👈 START HERE - Complete system overview
├── README.md                    # This file
│
├── architecture/                # Architectural deep-dives
│   ├── overview.md
│   └── communication-protocol.md
│
├── components/                  # Component implementation details
│   ├── translation-engine.md
│   ├── provider-adapters.md
│   ├── http-server.md
│   ├── cli.md
│   ├── security.md
│   └── observability.md
│
├── providers/                   # Provider integration guides
│   ├── patterns/
│   └── adding-providers.md
│
├── research/                    # Protocol research
│   └── claudecode.md
│
└── development/                 # Development guides
    ├── contributing.md
    ├── roadmap.md
    └── testing.md
```
