# Module 04: CLI (Command-Line Interface)

## Overview

The CLI module provides the user-facing command-line interface for GhostCLI, including interactive setup, configuration management, and application bootstrapping.

## Responsibilities

- Parse command-line flags and environment variables
- Interactive first-run setup wizard
- Provider selection and API key input
- Configuration file loading (YAML/JSON)
- API key validation
- Version information display
- Application lifecycle management
- Configuration clearing

## Architecture

```
CLI
├── Entry Point (main.go)
├── Flag Parsing
│   ├── --port, --provider, --api-key
│   ├── --verbose, --config, --timeout
│   └── --cors-origin, --skip-setup, --skip-validation
├── Interactive Setup
│   ├── Welcome Screen
│   ├── Provider Selection
│   ├── API Key Input (masked)
│   └── Configuration Save
├── Configuration Management
│   ├── File Loading (YAML/JSON)
│   ├── Environment Variables
│   └── Flag Priority
└── Commands
    ├── Start (default)
    ├── --version
    └── --clear-keys
```

## Related Requirements

- **Requirement 9**: CLI Configuration Management
- **Requirement 11**: Interactive First-Run Setup
- **Requirement 12**: Provider API Key Validation
- **Requirement 21**: Configuration File Support
- **Requirement 26**: Version Information
- **Requirement 30**: Configuration Clear Command

## Key Features

### Interactive Setup Wizard

**Flow**:
1. Detect missing configuration
2. Display welcome message
3. Present provider list with pattern labels
4. Prompt for API key (masked input)
5. Validate API key format
6. Save to secure storage
7. Display success message with usage instructions

**UI Libraries**:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for TUI
- [Survey](https://github.com/AlecAivazis/survey) for prompts

### Configuration Priority

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Configuration file** (lowest priority)

### Configuration File Format

**YAML**:
```yaml
port: 3200
active_provider: deepseek
providers:
  deepseek:
    pattern: openai
    base_url: https://api.deepseek.com
    api_key: sk-...
  kimi:
    pattern: openai
    base_url: https://api.moonshot.cn
    api_key: sk-...
```

**JSON**:
```json
{
  "port": 3200,
  "active_provider": "deepseek",
  "providers": {
    "deepseek": {
      "pattern": "openai",
      "base_url": "https://api.deepseek.com",
      "api_key": "sk-..."
    }
  }
}
```

### API Key Validation

**Process**:
1. Send minimal test request to provider API
2. Check HTTP status code
3. Display result (success or error)
4. Exit with status code 1 on failure (unless --skip-validation)

**Validation Patterns**:
- DeepSeek: `sk-[a-zA-Z0-9]{32,}`
- Kimi: `sk-[a-zA-Z0-9]{32,}`
- OpenAI: `sk-[a-zA-Z0-9]{48,}`
- Kiro: `ksk_[a-zA-Z0-9]{32,}`

### Version Information

**Output Format**:
```
GhostCLI v1.0.0
Commit: a1b2c3d
Built: 2024-01-15T10:30:00Z
```

**Build-time Injection**:
```bash
go build -ldflags "-X main.Version=1.0.0 -X main.Commit=$(git rev-parse HEAD) -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

## Implementation Details

See [design.md](./design.md) for detailed implementation specifications.
