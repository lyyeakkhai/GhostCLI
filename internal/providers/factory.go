package providers

import (
	"fmt"
	"log/slog"

	"ghostcli/internal/config"
	"ghostcli/internal/providers/base"
)

// Factory creates provider instances with dependency injection.
// It uses pattern-based routing to instantiate the appropriate provider
// adapter based on the provider configuration.
type Factory struct {
	config *config.Config
	logger *slog.Logger
}

// NewFactory creates a new provider factory with the given configuration and logger.
func NewFactory(cfg *config.Config, logger *slog.Logger) *Factory {
	return &Factory{
		config: cfg,
		logger: logger,
	}
}

// CreateProvider creates a provider instance based on the provider name.
// It looks up the provider configuration and uses pattern-based routing
// to instantiate the appropriate adapter (OpenAI-compatible, Anthropic-native, or AWS EventStream).
//
// Returns an error if:
// - The provider is not found in the configuration
// - The provider pattern is unknown
// - The provider adapter cannot be created
func (f *Factory) CreateProvider(name string) (Provider, error) {
	// Get provider configuration
	providerCfg, err := f.config.GetProviderConfig(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider config: %w", err)
	}

	// Log provider creation
	f.logger.Debug("creating provider",
		"name", name,
		"pattern", providerCfg.Pattern,
		"base_url", providerCfg.BaseURL,
	)

	// Route to appropriate factory method based on pattern
	switch providerCfg.Pattern {
	case "openai":
		return f.createOpenAICompatProvider(name, providerCfg)
	case "anthropic":
		return f.createAnthropicNativeProvider(name, providerCfg)
	case "aws":
		return f.createAWSEventStreamProvider(name, providerCfg)
	default:
		return nil, fmt.Errorf("unknown provider pattern: %s (supported: openai, anthropic, aws)", providerCfg.Pattern)
	}
}

// CreateActiveProvider creates a provider instance for the currently active provider.
// This is a convenience method that uses the ActiveProvider field from the configuration.
func (f *Factory) CreateActiveProvider() (Provider, error) {
	if f.config.ActiveProvider == "" {
		return nil, fmt.Errorf("no active provider configured")
	}
	return f.CreateProvider(f.config.ActiveProvider)
}

// createOpenAICompatProvider creates a provider adapter for OpenAI-compatible APIs.
// This pattern is used by providers like DeepSeek, Kimi, OpenAI, Groq, etc.
// that implement the OpenAI Chat Completions API format.
func (f *Factory) createOpenAICompatProvider(name string, cfg *config.ProviderConfig) (Provider, error) {
	// TODO: Implement OpenAI-compatible provider creation
	// This will be implemented in task 6.1 and 7.x
	return nil, fmt.Errorf("OpenAI-compatible provider creation not yet implemented")
}

// createAnthropicNativeProvider creates a provider adapter for Anthropic-native APIs.
// This pattern is used by providers like Anthropic, OpenRouter, and Kiro Gateway
// that implement the Anthropic Messages API format natively.
func (f *Factory) createAnthropicNativeProvider(name string, cfg *config.ProviderConfig) (Provider, error) {
	adapter := base.NewAnthropicAdapter(base.AnthropicConfig{
		Name:     name,
		BaseURL:  cfg.BaseURL,
		APIKey:   cfg.APIKey,
		ModelMap: cfg.ModelMap,
		Logger:   f.logger,
	})
	return adapter, nil
}

// createAWSEventStreamProvider creates a provider adapter for AWS EventStream APIs.
// This pattern is used by providers like Kiro and Amazon Bedrock that use
// the AWS EventStream binary protocol.
func (f *Factory) createAWSEventStreamProvider(name string, cfg *config.ProviderConfig) (Provider, error) {
	// TODO: Implement AWS EventStream provider creation
	// This will be implemented in task 6.3
	return nil, fmt.Errorf("AWS EventStream provider creation not yet implemented")
}

// ListSupportedPatterns returns a list of supported provider patterns.
func (f *Factory) ListSupportedPatterns() []string {
	return []string{"openai", "anthropic", "aws"}
}
