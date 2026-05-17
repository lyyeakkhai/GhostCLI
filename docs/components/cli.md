# CLI (Command-Line Interface)

> **Component**: Entry Point | **Layer**: 1 (CLI) | **Related**: [security.md](./security.md)

The CLI component provides the user-facing command-line interface, including flag parsing, configuration management, and interactive onboarding.

## Overview

The CLI is responsible for:
1. Parsing command-line flags and arguments
2. Loading configuration from files and environment
3. Interactive first-run onboarding experience
4. Launching the HTTP server with configured providers
5. Graceful shutdown handling

## Command Structure

```bash
ghostcli [flags]

Flags:
  --port, -p      Port to listen on (default: 3200)
  --provider, -m  Target provider (deepseek, kimi, kiro, openai)
  --api-key, -k   API key for the selected provider
  --config, -c    Path to config file
  --verbose, -v   Enable debug logging
  --version       Show version information
  --help, -h      Show help message
```

## Usage Examples

### Basic Usage

```bash
# Start with DeepSeek
export DEEPSEEK_API_KEY=sk-...
ghostcli --provider deepseek

# Start with custom port
ghostcli --provider kimi --port 8080

# Start with inline API key (not recommended)
ghostcli --provider deepseek --api-key sk-...
```

### Integration with Claude Code

```bash
# Terminal 1: Start GhostCLI
ghostcli --provider deepseek

# Terminal 2: Use Claude Code
export ANTHROPIC_BASE_URL=http://localhost:3200
claude
```

## Interactive Onboarding

When run for the first time (or when no configuration exists), GhostCLI enters **Setup Mode**.

### Welcome Screen

```
╔════════════════════════════════════════════════════════════╗
║                                                            ║
║    G H O S T C L I                                         ║
║    Use Claude Code with any provider, 10x cheaper.         ║
║                                                            ║
╚════════════════════════════════════════════════════════════╝

This tool acts as a bridge between Claude Code and models like 
DeepSeek, Kiro, and Kimi.
```

### Provider Selection

```
? Select your preferred provider:
  > DeepSeek (OpenAI-Compatible)
    Kimi (OpenAI-Compatible)
    Kiro (Anthropic-Native)
    Anthropic (Anthropic-Native)
    Custom...
```

### API Key Input

```
? Enter your API Key for DeepSeek:
  ********************************************
  
✓ API key validated successfully
```

### Configuration Saved

```
✓ Configuration saved to ~/.config/ghostcli/config.yaml

To use with Claude Code, run:
  export ANTHROPIC_BASE_URL=http://localhost:3200
  claude

Starting server on http://127.0.0.1:3200...
```

## Implementation

### Main Entry Point

```go
package main

import (
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
    Use:   "ghostcli",
    Short: "GhostCLI - Use Claude Code with any LLM provider",
    Long:  `A high-performance proxy that connects Claude Code to various LLM providers.`,
    Run:   runServer,
}

func init() {
    rootCmd.Flags().StringP("port", "p", "3200", "Port to listen on")
    rootCmd.Flags().StringP("provider", "m", "", "Target provider (deepseek, kimi, kiro)")
    rootCmd.Flags().StringP("api-key", "k", "", "API key for the provider")
    rootCmd.Flags().StringP("config", "c", "", "Path to config file")
    rootCmd.Flags().BoolP("verbose", "v", false, "Enable debug logging")
    
    viper.BindPFlags(rootCmd.Flags())
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        log.Fatal(err)
    }
}
```

### Server Launch

```go
func runServer(cmd *cobra.Command, args []string) {
    // Load configuration
    config := loadConfig()
    
    // Check if first run
    if config.Provider == "" {
        config = runOnboarding()
    }
    
    // Initialize logger
    initLogger(config.Verbose)
    
    // Load API key
    apiKey, err := loadAPIKey(config.Provider)
    if err != nil {
        log.Fatal("Failed to load API key:", err)
    }
    
    // Initialize engine
    engine := engine.New(config.Provider, apiKey)
    
    // Create HTTP server
    server := api.NewServer(config.Port, engine)
    
    // Handle shutdown
    setupShutdownHandler(server)
    
    // Start server
    log.Printf("Starting GhostCLI on http://127.0.0.1:%s", config.Port)
    log.Printf("Provider: %s", config.Provider)
    log.Printf("\nTo use with Claude Code:")
    log.Printf("  export ANTHROPIC_BASE_URL=http://localhost:%s", config.Port)
    log.Printf("  claude\n")
    
    if err := server.Start(); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}
```

### Configuration Management

```go
type Config struct {
    Provider string            `yaml:"provider"`
    Port     string            `yaml:"port"`
    Verbose  bool              `yaml:"verbose"`
    Providers map[string]ProviderConfig `yaml:"providers"`
}

type ProviderConfig struct {
    APIKey      string `yaml:"api_key,omitempty"`
    BaseURL     string `yaml:"base_url,omitempty"`
    DefaultModel string `yaml:"default_model,omitempty"`
}

func loadConfig() *Config {
    configPath := viper.GetString("config")
    if configPath == "" {
        configPath = filepath.Join(os.Getenv("HOME"), ".config", "ghostcli", "config.yaml")
    }
    
    viper.SetConfigFile(configPath)
    viper.SetConfigType("yaml")
    
    // Set defaults
    viper.SetDefault("port", "3200")
    
    // Read config file
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            log.Printf("Error reading config: %v", err)
        }
    }
    
    var config Config
    viper.Unmarshal(&config)
    
    // Override with flags
    if viper.GetString("provider") != "" {
        config.Provider = viper.GetString("provider")
    }
    if viper.GetString("port") != "" {
        config.Port = viper.GetString("port")
    }
    
    return &config
}

func saveConfig(config *Config) error {
    configPath := filepath.Join(os.Getenv("HOME"), ".config", "ghostcli", "config.yaml")
    
    // Create directory if not exists
    os.MkdirAll(filepath.Dir(configPath), 0755)
    
    // Marshal to YAML
    data, err := yaml.Marshal(config)
    if err != nil {
        return err
    }
    
    // Write to file
    return ioutil.WriteFile(configPath, data, 0600)
}
```

### Interactive Onboarding

```go
import (
    "github.com/AlecAivazis/survey/v2"
)

func runOnboarding() *Config {
    fmt.Println(welcomeBanner)
    
    // Provider selection
    var provider string
    providerPrompt := &survey.Select{
        Message: "Select your preferred provider:",
        Options: []string{
            "DeepSeek (OpenAI-Compatible)",
            "Kimi (OpenAI-Compatible)",
            "Kiro (Anthropic-Native)",
            "Anthropic (Anthropic-Native)",
        },
    }
    survey.AskOne(providerPrompt, &provider)
    
    // Extract provider name
    providerName := strings.Split(provider, " ")[0]
    providerName = strings.ToLower(providerName)
    
    // API key input
    var apiKey string
    apiKeyPrompt := &survey.Password{
        Message: fmt.Sprintf("Enter your API Key for %s:", providerName),
    }
    survey.AskOne(apiKeyPrompt, &apiKey)
    
    // Validate API key
    if err := validateAPIKey(providerName, apiKey); err != nil {
        log.Fatal("Invalid API key:", err)
    }
    
    fmt.Println("✓ API key validated successfully")
    
    // Save to keyring
    if err := security.SaveAPIKey(providerName, apiKey); err != nil {
        log.Printf("Warning: Could not save to keyring: %v", err)
    }
    
    // Create config
    config := &Config{
        Provider: providerName,
        Port:     "3200",
    }
    
    // Save config
    if err := saveConfig(config); err != nil {
        log.Printf("Warning: Could not save config: %v", err)
    }
    
    fmt.Printf("\n✓ Configuration saved to ~/.config/ghostcli/config.yaml\n\n")
    
    return config
}
```

### API Key Loading

```go
func loadAPIKey(provider string) (string, error) {
    // 1. Try command-line flag
    if key := viper.GetString("api-key"); key != "" {
        return key, nil
    }
    
    // 2. Try environment variable
    envVar := strings.ToUpper(provider) + "_API_KEY"
    if key := os.Getenv(envVar); key != "" {
        return key, nil
    }
    
    // 3. Try OS keyring
    key, err := security.LoadAPIKey(provider)
    if err == nil {
        return key, nil
    }
    
    // 4. Try config file
    if config.Providers != nil {
        if providerConfig, ok := config.Providers[provider]; ok {
            if providerConfig.APIKey != "" {
                return providerConfig.APIKey, nil
            }
        }
    }
    
    return "", fmt.Errorf("no API key found for provider: %s", provider)
}
```

### Shutdown Handling

```go
func setupShutdownHandler(server *api.Server) {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        log.Println("\nReceived shutdown signal...")
        
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        
        if err := server.Shutdown(ctx); err != nil {
            log.Printf("Error during shutdown: %v", err)
        }
        
        os.Exit(0)
    }()
}
```

## Configuration File Format

### Example: ~/.config/ghostcli/config.yaml

```yaml
provider: deepseek
port: 3200
verbose: false

providers:
  deepseek:
    base_url: https://api.deepseek.com
    default_model: deepseek-v4-pro
  
  kimi:
    base_url: https://api.moonshot.cn
    default_model: moonshot-v1-8k
  
  kiro:
    base_url: https://api.kiro.dev
    default_model: claude-sonnet-4.6
```

## Environment Variables

```bash
# Provider selection
GHOSTCLI_PROVIDER=deepseek

# API keys (provider-specific)
DEEPSEEK_API_KEY=sk-...
KIMI_API_KEY=sk-...
KIRO_API_KEY=ksk_...
ANTHROPIC_API_KEY=sk-ant-...

# Server configuration
GHOSTCLI_PORT=3200
GHOSTCLI_VERBOSE=true

# Claude Code integration
ANTHROPIC_BASE_URL=http://localhost:3200
```

## Logging

```go
func initLogger(verbose bool) {
    level := slog.LevelInfo
    if verbose {
        level = slog.LevelDebug
    }
    
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: level,
    }))
    
    slog.SetDefault(logger)
}
```

## Related Documentation

- [Security](./security.md) - API key storage
- [HTTP Server](./http-server.md) - Server implementation
- [Architecture Overview](../architecture/overview.md) - System architecture
