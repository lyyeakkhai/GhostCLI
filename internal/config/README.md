# Configuration Management Module

This module implements the configuration management layer for GhostCLI, providing support for multiple configuration sources with proper precedence and validation.

## Features

### Configuration Sources (Priority Order)
1. **CLI Flags** (highest priority)
2. **Environment Variables**
3. **Configuration File** (YAML or JSON)
4. **Defaults** (lowest priority)

### Supported Configuration Formats
- **YAML** (`.yaml`, `.yml`)
- **JSON** (`.json`)

### Configuration Structure

```go
type Config struct {
    Port           int                          // Server port (default: 3200)
    Host           string                       // Server host (default: 127.0.0.1)
    Timeout        int                          // Request timeout in seconds (default: 300)
    ActiveProvider string                       // Active provider name
    Providers      map[string]ProviderConfig    // Provider configurations
    CORSOrigin     string                       // CORS origin (default: *)
    Verbose        bool                         // Debug logging (default: false)
}

type ProviderConfig struct {
    Name     string            // Provider name
    Pattern  string            // Provider pattern: openai, anthropic, aws
    BaseURL  string            // Provider API base URL
    APIKey   string            // Provider API key
    ModelMap map[string]string // Model name mappings
}
```

## Usage

### Loading Configuration

```go
import "ghostcli/internal/config"

// Load with CLI flags
flags := map[string]interface{}{
    "port":        8080,
    "provider":    "deepseek",
    "api-key":     "sk-...",
    "verbose":     true,
}

cfg, err := config.Load("/path/to/config.yaml", flags)
if err != nil {
    log.Fatal(err)
}
```

### Environment Variables

The following environment variables are supported:

- `GHOST_PORT` - Server port
- `GHOST_HOST` - Server host
- `GHOST_TIMEOUT` - Request timeout in seconds
- `GHOST_PROVIDER` - Active provider name
- `GHOST_CORS_ORIGIN` - CORS origin
- `GHOST_VERBOSE` - Enable debug logging (true/1)
- `{PROVIDER}_API_KEY` - Provider-specific API key (e.g., `DEEPSEEK_API_KEY`)

### Configuration File Locations

1. Path specified by `--config` flag
2. Default location: `~/.config/ghost/config.yaml`

### Example Configuration Files

See `config.example.yaml` and `config.example.json` in the project root for complete examples.

## Validation

The configuration system validates:

- Port range (1-65535)
- Non-empty host
- Positive timeout
- Active provider exists in providers map
- Provider pattern is valid (openai, anthropic, or aws)
- Required provider fields (name, pattern, base_url, api_key)

## API

### Main Functions

- `DefaultConfig() *Config` - Returns configuration with defaults
- `Load(configPath string, cliFlags map[string]interface{}) (*Config, error)` - Loads and merges configuration
- `(c *Config) Validate() error` - Validates configuration
- `(c *Config) GetProviderConfig(name string) (*ProviderConfig, error)` - Gets provider configuration
- `(c *Config) GetActiveProviderConfig() (*ProviderConfig, error)` - Gets active provider configuration

### Provider Configuration

- `(pc *ProviderConfig) Validate() error` - Validates provider configuration

## Testing

Run tests with:

```bash
go test ./internal/config/...
```

All configuration loading, merging, and validation tests pass successfully.

## Implementation Details

### Configuration Merging

The `Load` function implements a three-stage merge:

1. **File Loading**: Parses YAML or JSON configuration file
2. **Environment Merge**: Overlays environment variables
3. **Flag Merge**: Applies CLI flags (highest priority)

### Provider Pattern Support

Three provider patterns are supported:

- **openai**: OpenAI-compatible APIs (DeepSeek, Kimi, OpenAI)
- **anthropic**: Anthropic-native APIs
- **aws**: AWS EventStream protocol (Kiro)

### Model Mapping

Each provider can define model name mappings to translate Anthropic model names to provider-specific names:

```yaml
model_map:
  claude-3-5-sonnet: "deepseek-chat"
  claude-3-opus: "deepseek-chat"
```

## Requirements Satisfied

This implementation satisfies the following requirements from the spec:

- **Requirement 9**: CLI Configuration Management
  - CLI flags for port, provider, api-key, verbose, config
  - Environment variable support
  - Configuration validation
  - Flag priority over environment variables

- **Requirement 21**: Configuration File Support
  - YAML and JSON file support
  - Multiple provider configurations
  - Active provider selection
  - Configuration file merging with CLI flags
  - Default config location support
  - Parse error handling
