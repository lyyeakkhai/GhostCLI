# Tech Stack

> Snapshot of all technologies in use for GhostCLI. Update this document when dependencies are added or changed.

---

## Language(s)

| Language | Version | Role |
|---|---|---|
| **Go (Golang)** | 1.22+ | Primary language — all proxy, server, engine, and CLI code |

Go was chosen for its static binary compilation, superior streaming performance (zero-buffer I/O), and excellent cross-platform support with no runtime dependencies.

---

## Frontend

_GhostCLI is a headless CLI proxy. There is no frontend._

| Item | Value |
|---|---|
| UI | None — terminal-only |
| Planned UI | None in scope |

---

## Backend

| Item | Value |
|---|---|
| **Runtime** | Go standard library (`net/http`) |
| **HTTP Framework** | `net/http` (stdlib) — optionally `chi` router for middleware chaining |
| **API Style** | REST — Anthropic Messages API compatible (`POST /v1/messages`) |
| **Streaming** | Server-Sent Events (SSE / `text/event-stream`) |
| **JSON Parsing** | `encoding/json` with `json.NewDecoder` (zero-buffer streaming) |
| **Default Port** | `3200` |

---

## Database

_GhostCLI is stateless. No database is used._

| Item | Value |
|---|---|
| **Database** | None |
| **Persistent state** | Configuration file (YAML/JSON) + OS Keyring for secrets |
| **Config storage path** | `~/.config/ghost/` (cross-platform via `os.UserConfigDir()`) |
| **Secret storage** | OS-native keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service) |
| **Encrypted fallback** | `~/.config/ghost/secrets.db` — AES-encrypted, key derived from machine hardware UUID |

---

## Infrastructure

| Item | Value |
|---|---|
| **Hosting** | Local machine (developer's workstation) — no cloud required |
| **CI/CD** | **GitHub Actions** — runs `go test` on Windows, Ubuntu, macOS runners |
| **Binary Builds** | **GoReleaser** — automated cross-platform static binary builds on tag push |
| **Containers** | None (static binary removes the need for Docker) |
| **Release artifacts** | `.zip` / `.tar.gz` per platform, uploaded to GitHub Releases |

### Build Targets

| Platform | GOOS | GOARCH | Output |
|---|---|---|---|
| Windows x64 | `windows` | `amd64` | `ghost.exe` |
| macOS Apple Silicon | `darwin` | `arm64` | `ghost` |
| macOS Intel | `darwin` | `amd64` | `ghost` |
| Linux x64 | `linux` | `amd64` | `ghost` |

---

## Authentication

| Item | Value |
|---|---|
| **Proxy auth** | API keys passed via `Authorization: Bearer <key>` or `x-api-key` header to provider APIs |
| **Local auth** | None — proxy binds to `127.0.0.1` only (loopback) |
| **Key management** | CLI `--api-key` flag → stored in OS Keyring via `github.com/zalando/go-keyring` |
| **Fallback storage** | AES-256-GCM encrypted file, key derived from `github.com/denisbrodbeck/machineid` |

---

## Key Libraries / Packages

| Package | Purpose |
|---|---|
| `net/http` (stdlib) | HTTP server and client |
| `encoding/json` (stdlib) | Zero-buffer JSON streaming decoder/encoder |
| `log/slog` (stdlib, Go 1.21+) | Structured logging with configurable log levels |
| `context` (stdlib) | Request lifecycle, cancellation propagation |
| `os`, `path/filepath` (stdlib) | Cross-platform path handling |
| `github.com/go-chi/chi/v5` | Lightweight HTTP router and middleware (optional, TBD) |
| `github.com/zalando/go-keyring` | OS-native keyring access (macOS Keychain, Windows Credential Manager, Linux) |
| `github.com/denisbrodbeck/machineid` | Hardware UUID for encryption key derivation |
| `gopkg.in/yaml.v3` | YAML configuration file parsing |
| `github.com/spf13/cobra` _(optional)_ | CLI flag parsing (may use stdlib `flag` instead for simplicity) |
| `github.com/spf13/viper` _(optional)_ | Configuration management with env var binding |
| `github.com/stretchr/testify` | Test assertions (`assert`, `require`) |

> **Note:** The dependency list is minimal by design. Prefer stdlib where possible.

---

## Dev Tools

| Tool | Purpose |
|---|---|
| **Go 1.22+** | Compiler and toolchain |
| **`gofmt`** | Code formatting (built into Go toolchain) |
| **`golangci-lint`** | Meta-linter (`govet`, `staticcheck`, `errcheck`, `gocyclo`) |
| **`go test ./...`** | Unit test runner |
| **`go vet ./...`** | Static analysis |
| **GoReleaser** | Cross-platform binary release automation |
| **GitHub Actions** | CI/CD pipeline |
| **`make`** _(optional)_ | `Makefile` for common tasks (`make build`, `make test`, `make lint`) |
| **Git** | Version control |

### Local Dev Setup

```bash
# Prerequisites: Go 1.22+, Git
git clone https://github.com/<org>/ghostcli
cd ghostcli
go mod download
go build ./cmd/ghost
./ghost --mode deepseek --api-key sk-...
```

---

## External APIs / Services

| Provider | Base URL | Format | Auth |
|---|---|---|---|
| **DeepSeek** | `https://api.deepseek.com/v1/chat/completions` | OpenAI Chat Completions | `Authorization: Bearer <DEEPSEEK_API_KEY>` |
| **Kimi (Moonshot)** | `https://api.moonshot.cn/v1/chat/completions` | OpenAI Chat Completions | `Authorization: Bearer <KIMI_API_KEY>` |
| **OpenAI** | `https://api.openai.com/v1/chat/completions` | OpenAI Chat Completions | `Authorization: Bearer <OPENAI_API_KEY>` |
| **Kiro** | `https://api.kiro.aws/...` | Anthropic-native / AWS EventStream | AWS credentials / Kiro token |
| **GitHub Releases** | `https://github.com/<org>/ghostcli/releases` | Binary distribution | Public (read), GitHub Actions token (write) |

### Environment Variables

| Variable | Provider | Description |
|---|---|---|
| `DEEPSEEK_API_KEY` | DeepSeek | API key for DeepSeek |
| `KIMI_API_KEY` | Kimi | API key for Moonshot/Kimi |
| `OPENAI_API_KEY` | OpenAI | API key for OpenAI |
| `KIRO_API_KEY` | Kiro | API key / token for Kiro |
| `ANTHROPIC_BASE_URL` | Claude Code | Set to `http://localhost:3200` to point Claude Code at the proxy |
| `GHOST_PORT` | GhostCLI | Override default port (3200) |
| `GHOST_PROVIDER` | GhostCLI | Override active provider |
