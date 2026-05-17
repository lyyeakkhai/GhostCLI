# Code Standard

> This document defines the engineering standards for GhostCLI. All contributors and AI agents must follow these rules consistently.

---

## Naming Conventions

### Go Identifiers

| Item | Convention | Example |
|---|---|---|
| **Packages** | All lowercase, single word or `snake_case` only if unavoidable | `providers`, `anthropicin` |
| **Files** | `snake_case.go` | `anthropic_in.go`, `openai_base.go` |
| **Exported types / functions** | `PascalCase` | `UnifiedChatRequest`, `StreamChat` |
| **Unexported types / functions** | `camelCase` | `parseHeaders`, `tokenCount` |
| **Constants** | `SCREAMING_SNAKE_CASE` for true constants; `PascalCase` for typed consts | `MAX_TOKENS`, `EventTypeText` |
| **Interfaces** | Single-method interfaces: verb + `er`; multi-method: noun describing the role | `Provider`, `StreamReader` |
| **Test functions** | `Test<FunctionName>_<Scenario>` | `TestAnthropicIn_InvalidJSON` |
| **Mocks** | `Mock<InterfaceName>` in `_test.go` files | `MockProvider` |

### Folder / File Naming

- All directory names are **lowercase** (`internal/`, `providers/`, `telemetry/`).
- Test files mirror the file they test with a `_test.go` suffix.
- No `util.go` or `helpers.go` catch-all files — name by what they do.

---

## File & Folder Structure

The canonical structure is defined in `docs/planning/plan.md`. Summary:

```text
ghostcli/
├── cmd/
│   └── ghost/               # CLI entry point, flag parsing, setup wizard, version
├── internal/
│   ├── app/                 # Orchestration: App struct, lifecycle management
│   ├── api/                 # HTTP layer: server, handlers, middleware, health
│   ├── config/              # Configuration: structs, file loading, secure storage
│   ├── engine/
│   │   ├── protocol/        # UnifiedChatRequest, UnifiedStreamEvent, constants
│   │   ├── translator/      # AnthropicIn parser, AnthropicOut formatter
│   │   └── pipeline/        # Stream orchestration, token usage normalization
│   ├── providers/
│   │   ├── base/            # Reusable adapter patterns (OpenAI, Anthropic, AWS)
│   │   ├── factory/         # ProviderFactory: dynamic instantiation
│   │   ├── deepseek/
│   │   ├── kimi/
│   │   ├── openai/
│   │   └── kiro/
│   └── telemetry/           # slog logger, metrics
├── pkg/                     # Optional: exported reusable utilities
├── docs/                    # Architecture docs, research, planning
└── context/                 # Project context: mission, roadmap, standards
```

**Rules:**
- All implementation packages live in `internal/`. Nothing implementation-specific in `pkg/`.
- Never create `utils/` or `common/` packages. Group by **domain**, not by type.
- Each package has one clear responsibility (SRP).

---

## Formatting Rules

| Rule | Value |
|---|---|
| **Indentation** | Tabs (Go standard — enforced by `gofmt`) |
| **Line length** | Soft limit 100 chars; hard limit 120 chars |
| **Braces** | Opening brace on the same line (Go standard) |
| **Trailing commas** | Required in multi-line struct/slice literals (Go enforces this) |
| **Blank lines** | One blank line between top-level declarations |
| **Import grouping** | Stdlib → External → Internal (separated by blank lines) |
| **String quotes** | Double quotes `"` always (Go standard) |

Always run `gofmt -w .` before committing. CI will reject unformatted code.

---

## Commenting & Documentation

- **All exported symbols** must have a GoDoc comment starting with the symbol name.
  ```go
  // StreamChat sends a UnifiedChatRequest to the provider and returns a channel of events.
  func (a *OpenAIAdapter) StreamChat(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error) {
  ```
- **Unexported functions** should have comments when the logic is non-obvious.
- **Package-level comments** are required for every `internal/` package in a `doc.go` file.
- **Inline comments** (`//`) explain *why*, not *what*. Never restate what the code does.
- **TODO / FIXME:** Format as `// TODO(owner): description` or `// FIXME: description`.
- Avoid commented-out code in committed files. Use feature flags or delete.

---

## Error Handling

- **Never use `panic`** in library code (only allowed in `main()` for unrecoverable startup failures).
- **Always wrap errors** with context using `fmt.Errorf("component: %w", err)`.
- **Sentinel errors:** Define package-level sentinel errors with `errors.New` for expected error states.
  ```go
  var ErrProviderNotFound = errors.New("provider not registered")
  ```
- **Error propagation:** Errors bubble up through channels via `UnifiedStreamEvent{Type: EventTypeError}`.
- **HTTP errors:** Use appropriate status codes. Map provider errors → Anthropic error SSE events.
- **Log then return:** Log at the boundary where the error is handled, not at every layer.
- **No silent errors:** Never discard errors with `_`. If you intentionally ignore one, add a comment.

---

## Testing Standards

### What to Test

- All `internal/engine/translator/` functions (critical path — must have near-100% coverage).
- All provider adapter `StreamChat` methods (use mock HTTP servers).
- All `internal/config/` functions (config loading, merging, validation).
- Error paths: invalid JSON, provider timeouts, context cancellation.

### Naming

```go
func Test<Type>_<Method>_<Scenario>(t *testing.T) { ... }
// Examples:
func TestAnthropicInParser_Parse_ValidRequest(t *testing.T) { ... }
func TestAnthropicInParser_Parse_InvalidJSON(t *testing.T) { ... }
func TestOpenAIAdapter_StreamChat_ContextCancelled(t *testing.T) { ... }
```

### Rules

- Use `testing` stdlib + `testify/assert` for assertions.
- Use table-driven tests for parsing/formatting logic.
- Never hit real external APIs in unit tests — use `httptest.NewServer` for mocks.
- Subtests use `t.Run("scenario", func(t *testing.T) { ... })`.
- Test files live in the same package (`package foo`) for white-box or `package foo_test` for black-box.

---

## Git & Commits

### Branch Naming

```
feat/<short-description>       # New feature
fix/<short-description>        # Bug fix
chore/<short-description>      # Tooling, deps, CI
docs/<short-description>       # Documentation only
refactor/<short-description>   # No behaviour change
```

### Commit Message Format (Conventional Commits)

```
<type>(<scope>): <short summary>

[Optional body explaining WHY, not WHAT]

[Optional footer: Breaking changes, closes #issue]
```

**Types:** `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`  
**Scopes:** `engine`, `providers`, `api`, `config`, `cli`, `telemetry`, `build`

**Examples:**
```
feat(providers): add Kimi adapter with moonshot-v1-128k model mapping
fix(engine): inject last known token counts when provider omits usage
chore(ci): add macOS arm64 build target to GoReleaser config
```

### PR Process

1. Open a PR against `main`. Title must follow the commit convention above.
2. All CI checks must pass (lint, test on win/ubuntu/macos).
3. At least 1 approval required before merge.
4. Squash merge preferred to keep history clean.
5. Delete branch after merge.

---

## Linting / Tooling

| Tool | Purpose | Config |
|---|---|---|
| `gofmt` | Code formatting | Built-in (no config) |
| `golangci-lint` | Meta-linter (runs `govet`, `staticcheck`, `errcheck`, etc.) | `.golangci.yml` in root |
| `go test ./...` | Unit tests | Run in CI on win/ubuntu/macos |
| `go vet ./...` | Static analysis | Part of CI pipeline |
| `GoReleaser` | Cross-platform binary builds + GitHub Releases | `.goreleaser.yaml` in root |

**Key lint rules enforced by `golangci-lint`:**
- `errcheck` — all errors must be handled.
- `govet` — no `printf` format string mismatches, no suspicious code.
- `staticcheck` — no deprecated API usage.
- `gocyclo` — cyclomatic complexity ≤ 15 per function.

Run before every commit:
```bash
gofmt -w .
golangci-lint run ./...
go test ./...
```
