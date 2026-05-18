package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration.
type Config struct {
	// Server configuration
	Port    int    `yaml:"port" json:"port"`
	Host    string `yaml:"host" json:"host"`
	Timeout int    `yaml:"timeout" json:"timeout"` // seconds

	// Active provider
	ActiveProvider string `yaml:"active_provider" json:"active_provider"`

	// Provider configurations
	Providers map[string]ProviderConfig `yaml:"providers" json:"providers"`

	// CORS configuration
	CORSOrigin string `yaml:"cors_origin" json:"cors_origin"`

	// Logging
	Verbose bool `yaml:"verbose" json:"verbose"`
}

// ProviderConfig represents configuration for a single provider.
type ProviderConfig struct {
	Name     string            `yaml:"name" json:"name"`
	Pattern  string            `yaml:"pattern" json:"pattern"` // openai, anthropic, aws
	BaseURL  string            `yaml:"base_url" json:"base_url"`
	APIKey   string            `yaml:"api_key" json:"api_key"`
	ModelMap map[string]string `yaml:"model_map" json:"model_map"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Port:       3200,
		Host:       "127.0.0.1",
		Timeout:    300,
		CORSOrigin: "*",
		Verbose:    false,
		Providers:  make(map[string]ProviderConfig),
	}
}

// Load loads configuration from a file, environment variables, and CLI flags.
// Priority: CLI flags > environment variables > config file > defaults
func Load(configPath string, cliFlags map[string]interface{}) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Load from config file if provided
	if configPath != "" {
		if err := cfg.loadFromFile(configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	} else {
		// Try to load from default location
		defaultPath, err := getDefaultConfigPath()
		if err == nil && fileExists(defaultPath) {
			if err := cfg.loadFromFile(defaultPath); err != nil {
				return nil, fmt.Errorf("failed to load default config file: %w", err)
			}
		}
	}

	// Merge environment variables
	cfg.mergeFromEnv()

	// Merge CLI flags (highest priority)
	if cliFlags != nil {
		cfg.mergeFromFlags(cliFlags)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// loadFromFile loads configuration from a YAML or JSON file.
func (c *Config) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Determine file format by extension
	ext := filepath.Ext(path)
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, c); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, c); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s (use .yaml, .yml, or .json)", ext)
	}

	return nil
}

// mergeFromEnv merges configuration from environment variables.
func (c *Config) mergeFromEnv() {
	// Server configuration
	if port := os.Getenv("GHOST_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Port = p
		}
	}
	if host := os.Getenv("GHOST_HOST"); host != "" {
		c.Host = host
	}
	if timeout := os.Getenv("GHOST_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			c.Timeout = t
		}
	}

	// Active provider
	if provider := os.Getenv("GHOST_PROVIDER"); provider != "" {
		c.ActiveProvider = provider
	}

	// CORS
	if corsOrigin := os.Getenv("GHOST_CORS_ORIGIN"); corsOrigin != "" {
		c.CORSOrigin = corsOrigin
	}

	// Logging
	if verbose, ok := os.LookupEnv("GHOST_VERBOSE"); ok {
		if verbose == "true" || verbose == "1" {
			c.Verbose = true
		} else if verbose == "false" || verbose == "0" {
			c.Verbose = false
		}
	}

	// Provider-specific API keys (e.g., DEEPSEEK_API_KEY, OPENAI_API_KEY)
	// These are merged into the Providers map if the provider exists
	for name := range c.Providers {
		envKey := fmt.Sprintf("%s_API_KEY", toEnvVarName(name))
		if apiKey := os.Getenv(envKey); apiKey != "" {
			provider := c.Providers[name]
			provider.APIKey = apiKey
			c.Providers[name] = provider
		}
	}
}

// mergeFromFlags merges configuration from CLI flags.
func (c *Config) mergeFromFlags(flags map[string]interface{}) {
	if port, ok := flags["port"].(int); ok && port != 0 {
		c.Port = port
	}
	if host, ok := flags["host"].(string); ok && host != "" {
		c.Host = host
	}
	if timeout, ok := flags["timeout"].(int); ok && timeout != 0 {
		c.Timeout = timeout
	}
	if provider, ok := flags["provider"].(string); ok && provider != "" {
		c.ActiveProvider = provider
	}
	if apiKey, ok := flags["api-key"].(string); ok && apiKey != "" {
		// Set API key for active provider
		if c.ActiveProvider != "" {
			if provider, exists := c.Providers[c.ActiveProvider]; exists {
				provider.APIKey = apiKey
				c.Providers[c.ActiveProvider] = provider
			}
		}
	}
	if corsOrigin, ok := flags["cors-origin"].(string); ok && corsOrigin != "" {
		c.CORSOrigin = corsOrigin
	}
	if verbose, ok := flags["verbose"].(bool); ok {
		c.Verbose = verbose
	}
}

// Validate checks that all required configuration fields are present and valid.
func (c *Config) Validate() error {
	// Validate port
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be between 1 and 65535)", c.Port)
	}

	// Validate host
	if c.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	// Validate timeout
	if c.Timeout < 1 {
		return fmt.Errorf("invalid timeout: %d (must be positive)", c.Timeout)
	}

	// Validate active provider
	if c.ActiveProvider == "" {
		return fmt.Errorf("active_provider must be specified")
	}

	// Validate that active provider exists in providers map
	if _, exists := c.Providers[c.ActiveProvider]; !exists {
		return fmt.Errorf("active provider '%s' not found in providers configuration", c.ActiveProvider)
	}

	// Validate provider configurations
	for name, provider := range c.Providers {
		if err := provider.Validate(); err != nil {
			return fmt.Errorf("invalid configuration for provider '%s': %w", name, err)
		}
	}

	return nil
}

// Validate checks that a ProviderConfig is valid.
func (pc *ProviderConfig) Validate() error {
	if pc.Name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if pc.Pattern == "" {
		return fmt.Errorf("provider pattern cannot be empty")
	}
	if pc.Pattern != "openai" && pc.Pattern != "anthropic" && pc.Pattern != "aws" {
		return fmt.Errorf("invalid provider pattern: %s (must be 'openai', 'anthropic', or 'aws')", pc.Pattern)
	}
	if pc.BaseURL == "" {
		return fmt.Errorf("provider base_url cannot be empty")
	}
	if pc.APIKey == "" {
		return fmt.Errorf("provider api_key cannot be empty")
	}
	return nil
}

// GetProviderConfig returns the configuration for a specific provider.
func (c *Config) GetProviderConfig(name string) (*ProviderConfig, error) {
	provider, exists := c.Providers[name]
	if !exists {
		return nil, fmt.Errorf("provider '%s' not found", name)
	}
	return &provider, nil
}

// GetActiveProviderConfig returns the configuration for the active provider.
func (c *Config) GetActiveProviderConfig() (*ProviderConfig, error) {
	return c.GetProviderConfig(c.ActiveProvider)
}

// getDefaultConfigPath returns the default configuration file path.
func getDefaultConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "ghost", "config.yaml"), nil
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// toEnvVarName converts a provider name to an environment variable name.
// Example: "deepseek" -> "DEEPSEEK"
func toEnvVarName(name string) string {
	result := ""
	for _, ch := range name {
		if ch >= 'a' && ch <= 'z' {
			result += string(ch - 'a' + 'A')
		} else if ch >= 'A' && ch <= 'Z' {
			result += string(ch)
		} else if ch >= '0' && ch <= '9' {
			result += string(ch)
		} else {
			result += "_"
		}
	}
	return result
}
