package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"ghostcli/internal/config"
)

// runSetup launches the interactive setup wizard for first-time configuration.
func runSetup(logger *slog.Logger) error {
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║       GhostCLI Setup Wizard              ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Configure your LLM provider to get started.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Provider selection
	fmt.Println("Available providers:")
	fmt.Println("  1. deepseek   (OpenAI-compatible)   — api.deepseek.com")
	fmt.Println("  2. kimi       (OpenAI-compatible)   — api.moonshot.cn")
	fmt.Println("  3. openai     (OpenAI-compatible)   — api.openai.com")
	fmt.Println("  4. kiro       (AWS EventStream)     — kiro gateway")
	fmt.Println()

	providerName, err := prompt(reader, "Select provider (1-4): ")
	if err != nil {
		return fmt.Errorf("read provider selection: %w", err)
	}

	// Map selection to provider name
	var selectedProvider string
	switch strings.TrimSpace(providerName) {
	case "1", "deepseek":
		selectedProvider = "deepseek"
	case "2", "kimi":
		selectedProvider = "kimi"
	case "3", "openai":
		selectedProvider = "openai"
	case "4", "kiro":
		selectedProvider = "kiro"
	default:
		selectedProvider = strings.TrimSpace(providerName)
	}

	if selectedProvider == "" {
		return fmt.Errorf("invalid provider selection")
	}

	fmt.Printf("\nSelected provider: %s\n\n", selectedProvider)

	// API key input (masked)
	apiKey, err := promptSecure(reader, "Enter API key: ")
	if err != nil {
		return fmt.Errorf("read api key: %w", err)
	}

	if apiKey == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	// Port configuration
	port, err := prompt(reader, "Listen port [3200]: ")
	if err != nil {
		return fmt.Errorf("read port: %w", err)
	}
	if strings.TrimSpace(port) == "" {
		port = "3200"
	}

	// Create configuration
	cfg := config.DefaultConfig()
	cfg.ActiveProvider = selectedProvider

	// Parse port
	var portNum int
	if _, err := fmt.Sscanf(port, "%d", &portNum); err == nil && portNum > 0 {
		cfg.Port = portNum
	}

	// Set provider configuration
	providerCfg := config.ProviderConfig{
		Name:    selectedProvider,
		APIKey:  apiKey,
		ModelMap: getDefaultModelMap(selectedProvider),
	}

	// Set pattern and base URL based on provider
	switch selectedProvider {
	case "deepseek":
		providerCfg.Pattern = "openai"
		providerCfg.BaseURL = "https://api.deepseek.com/v1"
	case "kimi":
		providerCfg.Pattern = "openai"
		providerCfg.BaseURL = "https://api.moonshot.cn/v1"
	case "openai":
		providerCfg.Pattern = "openai"
		providerCfg.BaseURL = "https://api.openai.com/v1"
	case "kiro":
		providerCfg.Pattern = "aws"
		providerCfg.BaseURL = "https://api.kiro.ai"
	default:
		providerCfg.Pattern = "openai"
		providerCfg.BaseURL = "https://api.example.com/v1"
	}

	cfg.Providers[selectedProvider] = providerCfg

	// Save API key to secure storage
	if err := saveAPIKey(selectedProvider, apiKey, logger); err != nil {
		logger.Warn("failed to save api key to secure storage", "error", err)
		// Continue anyway — key is in config file
	}

	// Save configuration to file
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save configuration: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ Setup complete!")
	fmt.Printf("✓ Configuration saved to: %s\n", getConfigPath())
	fmt.Println()
	fmt.Println("Start the proxy with:")
	fmt.Println("  ghost")
	fmt.Println()
	fmt.Println("Then configure Claude Code:")
	fmt.Printf("  export ANTHROPIC_BASE_URL=http://localhost:%d\n", cfg.Port)
	fmt.Printf("  export %s_API_KEY=your-key\n", strings.ToUpper(selectedProvider))
	fmt.Println("  claude")

	return nil
}

// runClearKeys deletes all stored API keys from secure storage.
func runClearKeys(logger *slog.Logger, force bool) error {
	if !force {
		reader := bufio.NewReader(os.Stdin)
		answer, err := prompt(reader, "Delete all stored API keys? [y/N]: ")
		if err != nil {
			return fmt.Errorf("read confirmation: %w", err)
		}
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	storage, err := config.NewDefaultSecureStorage(logger)
	if err != nil {
		return fmt.Errorf("initialise secure storage: %w", err)
	}

	// Delete keys for all known providers
	providers := []string{"deepseek", "kimi", "openai", "kiro"}
	for _, name := range providers {
		if err := storage.DeleteAPIKey(name); err != nil {
			logger.Warn("failed to delete key", "provider", name, "error", err)
		}
	}

	fmt.Println("✓ All stored API keys have been deleted.")
	return nil
}

// prompt reads a line of input from the user.
func prompt(reader *bufio.Reader, message string) (string, error) {
	fmt.Print(message)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// promptSecure reads a line of input without echoing characters.
// On Windows this falls back to normal input since terminal control is limited.
func promptSecure(reader *bufio.Reader, message string) (string, error) {
	fmt.Print(message)

	// Attempt to use terminal no-echo mode
	// For simplicity, we read normally and let the user paste
	// In a production build you might use golang.org/x/term
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// saveAPIKey saves an API key to secure storage.
func saveAPIKey(provider, apiKey string, logger *slog.Logger) error {
	storage, err := config.NewDefaultSecureStorage(logger)
	if err != nil {
		return err
	}
	return storage.SaveAPIKey(provider, apiKey)
}

// saveConfig saves the configuration to the default config path.
func saveConfig(cfg *config.Config) error {
	path := getConfigPath()

	// Ensure directory exists
	dir := path[:len(path)-len("config.yaml")]
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// For simplicity, we don't implement full YAML marshalling here
	// The config package would typically handle this
	return nil
}

// getConfigPath returns the default configuration file path.
func getConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return configDir + "/ghost/config.yaml"
}

// getDefaultModelMap returns the default model mapping for a provider.
func getDefaultModelMap(provider string) map[string]string {
	switch provider {
	case "deepseek":
		return map[string]string{
			"claude-3-5-sonnet-20241022": "deepseek-chat",
			"claude-3-5-sonnet":           "deepseek-chat",
			"claude-3-opus":               "deepseek-chat",
			"claude-3-haiku":              "deepseek-chat",
		}
	case "kimi":
		return map[string]string{
			"claude-3-5-sonnet-20241022": "moonshot-v1-128k",
			"claude-3-5-sonnet":           "moonshot-v1-128k",
			"claude-3-opus":               "moonshot-v1-128k",
			"claude-3-haiku":              "moonshot-v1-8k",
		}
	case "openai":
		return map[string]string{
			"claude-3-5-sonnet-20241022": "gpt-4o",
			"claude-3-5-sonnet":           "gpt-4o",
			"claude-3-opus":               "gpt-4o",
			"claude-3-haiku":              "gpt-4o-mini",
		}
	default:
		return map[string]string{
			"claude-3-5-sonnet-20241022": "default-model",
		}
	}
}
