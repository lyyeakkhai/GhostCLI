# GhostCLI Implementation Roadmap

> **Status**: Planning and architectural design phase | **Language**: Go 1.22+

This document outlines the phased implementation strategy for GhostCLI, following the Unified Protocol architecture and 6-module design.

## Project Status

**Current Phase**: Foundation & Architecture Design
**Next Phase**: Phase 1 - Foundation & Entry Point

## Implementation Phases

### Phase 1: Foundation & Entry Point
**Modules**: CLI (04), HTTP Server (01)
**Goal**: Establish project skeleton, command-line interface, and HTTP front door

**Tasks**:
- [x] Project initialization (Go module, directory structure)
- [ ] CLI scaffolding (Cobra, flag parsing)
- [ ] HTTP server implementation (routing, middleware)
- [ ] Health check endpoints
- [ ] Basic logging setup

**Deliverables**:
- Working HTTP server on localhost:3200
- CLI with `--port`, `--provider`, `--api-key` flags
- `/health` and `/ping` endpoints

---

### Phase 2: Core Data Types & Translation Engine
**Modules**: Translation Engine (02)
**Goal**: Build the universal translation layer

**Tasks**:
- [ ] Define Unified Protocol types (`UnifiedChatRequest`, `UnifiedStreamEvent`)
- [ ] Implement AnthropicIn parser (zero-buffer JSON parsing)
- [ ] Implement AnthropicOut formatter (SSE streaming)
- [ ] Usage tracking and normalization
- [ ] Unit tests for translation logic

**Deliverables**:
- Complete translation engine
- Anthropic request → Unified format
- Unified events → Anthropic SSE
- Usage normalization working

---

### Phase 3: The Provider Ecosystem
**Modules**: Provider Adapters (03)
**Goal**: Implement routing logic and provider integrations

**Tasks**:
- [ ] Define Provider interface
- [ ] Implement Provider Factory/Router
- [ ] Implement OpenAI-Compatible base pattern
- [ ] Implement DeepSeek adapter
- [ ] Implement Kimi adapter
- [ ] Implement Kiro (Anthropic-Native) adapter
- [ ] Model name mapping
- [ ] Provider-specific error handling

**Deliverables**:
- Working provider adapters for DeepSeek, Kimi, Kiro
- Pattern-based architecture
- Dynamic provider selection

---

### Phase 4: Security & Observability
**Modules**: Security (05), Observability (06)
**Goal**: Secure credentials and ensure system observability

**Tasks**:
- [ ] OS-native keyring integration (zalando/go-keyring)
- [ ] Encrypted file fallback storage
- [ ] API key validation
- [ ] Structured logging (slog) across all modules
- [ ] Metrics collection (optional Prometheus)
- [ ] Request timing and performance tracking

**Deliverables**:
- Secure API key storage (no plaintext)
- Comprehensive structured logging
- Optional metrics endpoint

---

### Phase 5: CLI Polish & Distribution
**Modules**: CLI (04)
**Goal**: Finalize user experience and prepare for release

**Tasks**:
- [ ] Interactive onboarding flow (Bubble Tea)
- [ ] Configuration file support (YAML/JSON)
- [ ] Multi-provider configuration
- [ ] Environment variable parsing
- [ ] GoReleaser configuration
- [ ] GitHub Actions CI/CD
- [ ] Cross-platform binary builds (Windows, macOS, Linux)
- [ ] NPM wrapper package (optional)

**Deliverables**:
- Polished CLI with interactive setup
- Automated cross-platform releases
- Installation via package managers

---

## Technology Stack

### Core
- **Language**: Go 1.22+
- **HTTP**: Standard library `net/http`
- **JSON**: `encoding/json` with streaming
- **Logging**: `log/slog`

### CLI
- **Framework**: `spf13/cobra`
- **Config**: `spf13/viper`
- **TUI**: `charmbracelet/bubbletea`

### Security
- **Keyring**: `zalando/go-keyring`

### Distribution
- **Build**: `goreleaser`
- **CI/CD**: GitHub Actions

---

## Project Structure

```
ghostcli/
├── cmd/
│   └── deepclaude/          # CLI entry point
├── internal/                # Private implementation
│   ├── app/                 # Application orchestration
│   ├── api/                 # HTTP server and routing
│   ├── engine/              # Translation engine
│   │   ├── protocol/        # Unified protocol types
│   │   └── pipeline/        # Transformation pipelines
│   ├── providers/           # Provider adapters
│   │   ├── base/            # Base abstractions
│   │   ├── factory/         # Provider factory
│   │   ├── deepseek/        # DeepSeek adapter
│   │   ├── kimi/            # Kimi adapter
│   │   └── kiro/            # Kiro adapter
│   └── telemetry/           # Logging and metrics
├── pkg/                     # Public utilities (optional)
└── docs/                    # Documentation
```

---

## Engineering Standards

### SOLID Principles

1. **Single Responsibility (SRP)**
   - Each package has one clear purpose
   - Translation engine only translates
   - Providers only communicate with APIs

2. **Open/Closed (OCP)**
   - System open for extension (new providers)
   - Closed for modification (core engine unchanged)

3. **Liskov Substitution (LSP)**
   - All Provider implementations are interchangeable
   - Core engine doesn't depend on specific adapters

4. **Interface Segregation (ISP)**
   - Small, focused interfaces
   - Provider interface only defines `StreamChat()`

5. **Dependency Inversion (DIP)**
   - High-level modules depend on abstractions
   - HTTP Server depends on Provider interface, not concrete adapters

### Design Patterns

- **Factory Pattern**: ProviderFactory for dynamic adapter creation
- **Strategy Pattern**: Provider adapters implement different strategies
- **Adapter Pattern**: Each provider wraps a different API
- **Observer Pattern**: Streaming uses channels for event propagation

---

## Cross-Platform Strategy

### Unified Codebase
- Use Go standard library (`os`, `path/filepath`, `net/http`)
- Avoid OS-specific APIs
- Use `filepath.Join()` for path handling

### Automated Builds (CI/CD)
GitHub Actions + GoReleaser automatically builds for:
- **Windows**: `GOOS=windows GOARCH=amd64` → `ghostcli.exe`
- **macOS**: `GOOS=darwin GOARCH=arm64` (M1/M2/M3) and `amd64` (Intel)
- **Linux**: `GOOS=linux GOARCH=amd64`

### Native Experience
- **Windows**: Single `.exe` binary
- **macOS**: Homebrew package
- **Linux**: Static binary (works on all distros)

---

## Distribution Strategy

### 1. NPM Package (Recommended)
```bash
npm install -g ghostcli
ghostcli --provider deepseek
```

**How it works**: NPM wrapper detects OS/architecture and downloads the correct Go binary from GitHub Releases.

### 2. On-Demand (No Install)
```bash
npx ghostcli --provider kimi
```

### 3. Package Managers
- **macOS**: `brew install ghostcli`
- **Windows**: `scoop install ghostcli`
- **Linux**: `curl -sSL https://ghostcli.io/install.sh | sh`

### 4. GitHub Releases (Manual)
Download pre-built binaries from GitHub Releases page.

---

## Testing Strategy

### Unit Tests
- Test each component in isolation
- Mock dependencies (providers, HTTP clients)
- Focus on transformation logic

### Integration Tests
- Test component interactions
- Use test providers (echo servers)
- Verify end-to-end flow

### Performance Tests
- Benchmark translation overhead
- Measure memory usage
- Test concurrent request handling

---

## Next Steps

### Immediate (Phase 1)
1. Initialize Go module and folder structure
2. Implement basic CLI with Cobra
3. Create HTTP server with routing
4. Add health check endpoints

### Short-term (Phase 2-3)
1. Define Unified Protocol types
2. Implement translation engine
3. Create provider adapters for DeepSeek, Kimi, Kiro

### Medium-term (Phase 4-5)
1. Add security (keyring integration)
2. Implement structured logging
3. Polish CLI with interactive onboarding
4. Set up automated releases

---

## Contributing

See [contributing.md](./contributing.md) for development workflow and guidelines.

---

## Related Documentation

- [ARCHITECTURE.md](../ARCHITECTURE.md) - System architecture
- [Components](../components/) - Component documentation
- [Provider Adapters](../components/provider-adapters.md) - Provider integration
