// Package config provides configuration management for GhostCLI, including
// API key validation for different LLM providers.
//
// The validation module implements provider-specific API key format validation
// using regex patterns and test request functionality to validate keys against
// provider APIs. This provides immediate feedback on configuration errors before
// the proxy starts.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// APIKeyValidator validates API keys for different providers.
type APIKeyValidator struct {
	client *http.Client
}

// NewAPIKeyValidator creates a new API key validator.
func NewAPIKeyValidator(client *http.Client) *APIKeyValidator {
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	return &APIKeyValidator{
		client: client,
	}
}

// ProviderValidationConfig holds validation configuration for a provider.
type ProviderValidationConfig struct {
	Name           string
	Pattern        string // Provider pattern: openai, anthropic, aws
	KeyRegex       *regexp.Regexp
	TestEndpoint   string
	TestMethod     string
	AuthHeader     string
	AuthPrefix     string
	MinimalPayload interface{}
}

// Common API key regex patterns for different providers.
var (
	// DeepSeek uses sk- prefix like OpenAI
	DeepSeekKeyRegex = regexp.MustCompile(`^sk-[a-zA-Z0-9]{32,}$`)

	// Kimi (Moonshot) uses sk- prefix
	KimiKeyRegex = regexp.MustCompile(`^sk-[a-zA-Z0-9]{32,}$`)

	// OpenAI uses sk- prefix
	OpenAIKeyRegex = regexp.MustCompile(`^sk-[a-zA-Z0-9\-_]{20,}$`)

	// Kiro uses various formats depending on deployment
	KiroKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_]{20,}$`)

	// Anthropic uses sk-ant- prefix
	AnthropicKeyRegex = regexp.MustCompile(`^sk-ant-[a-zA-Z0-9\-_]{20,}$`)
)

// GetValidationConfig returns the validation configuration for a provider.
func GetValidationConfig(providerName, pattern, baseURL string) (*ProviderValidationConfig, error) {
	switch strings.ToLower(providerName) {
	case "deepseek":
		return &ProviderValidationConfig{
			Name:         "deepseek",
			Pattern:      "openai",
			KeyRegex:     DeepSeekKeyRegex,
			TestEndpoint: baseURL + "/models",
			TestMethod:   "GET",
			AuthHeader:   "Authorization",
			AuthPrefix:   "Bearer ",
		}, nil

	case "kimi", "moonshot":
		return &ProviderValidationConfig{
			Name:         "kimi",
			Pattern:      "openai",
			KeyRegex:     KimiKeyRegex,
			TestEndpoint: baseURL + "/models",
			TestMethod:   "GET",
			AuthHeader:   "Authorization",
			AuthPrefix:   "Bearer ",
		}, nil

	case "openai":
		return &ProviderValidationConfig{
			Name:         "openai",
			Pattern:      "openai",
			KeyRegex:     OpenAIKeyRegex,
			TestEndpoint: baseURL + "/models",
			TestMethod:   "GET",
			AuthHeader:   "Authorization",
			AuthPrefix:   "Bearer ",
		}, nil

	case "kiro":
		return &ProviderValidationConfig{
			Name:         "kiro",
			Pattern:      "aws",
			KeyRegex:     KiroKeyRegex,
			TestEndpoint: baseURL + "/health",
			TestMethod:   "GET",
			AuthHeader:   "X-API-Key",
			AuthPrefix:   "",
		}, nil

	case "anthropic":
		return &ProviderValidationConfig{
			Name:         "anthropic",
			Pattern:      "anthropic",
			KeyRegex:     AnthropicKeyRegex,
			TestEndpoint: baseURL + "/v1/messages",
			TestMethod:   "POST",
			AuthHeader:   "x-api-key",
			AuthPrefix:   "",
			MinimalPayload: map[string]interface{}{
				"model":      "claude-3-5-sonnet-20241022",
				"max_tokens": 1,
				"messages": []map[string]interface{}{
					{
						"role":    "user",
						"content": "Hi",
					},
				},
			},
		}, nil

	default:
		// Generic validation for unknown providers
		return &ProviderValidationConfig{
			Name:         providerName,
			Pattern:      pattern,
			KeyRegex:     regexp.MustCompile(`^[a-zA-Z0-9\-_]{10,}$`),
			TestEndpoint: baseURL + "/health",
			TestMethod:   "GET",
			AuthHeader:   "Authorization",
			AuthPrefix:   "Bearer ",
		}, nil
	}
}

// ValidationResult holds the result of an API key validation.
type ValidationResult struct {
	Valid        bool
	FormatValid  bool
	APIReachable bool
	ErrorMessage string
	StatusCode   int
}

// ValidateFormat checks if the API key matches the expected format for the provider.
func (v *APIKeyValidator) ValidateFormat(apiKey string, config *ProviderValidationConfig) bool {
	if config.KeyRegex == nil {
		// No regex pattern defined, skip format validation
		return true
	}
	return config.KeyRegex.MatchString(apiKey)
}

// ValidateAPIKey performs both format and API validation for a provider API key.
func (v *APIKeyValidator) ValidateAPIKey(ctx context.Context, apiKey string, config *ProviderValidationConfig) *ValidationResult {
	result := &ValidationResult{
		Valid:       false,
		FormatValid: false,
		APIReachable: false,
	}

	// Step 1: Format validation
	result.FormatValid = v.ValidateFormat(apiKey, config)
	if !result.FormatValid {
		result.ErrorMessage = fmt.Sprintf("API key format is invalid for %s provider. Expected pattern: %s", config.Name, config.KeyRegex.String())
		return result
	}

	// Step 2: API validation with test request
	apiResult := v.testAPIKey(ctx, apiKey, config)
	result.APIReachable = apiResult.APIReachable
	result.StatusCode = apiResult.StatusCode
	result.ErrorMessage = apiResult.ErrorMessage
	result.Valid = apiResult.Valid

	return result
}

// testAPIKey sends a minimal test request to the provider API to validate the key.
func (v *APIKeyValidator) testAPIKey(ctx context.Context, apiKey string, config *ProviderValidationConfig) *ValidationResult {
	result := &ValidationResult{
		Valid:        false,
		FormatValid:  true,
		APIReachable: false,
	}

	// Create HTTP request
	var req *http.Request
	var err error

	if config.TestMethod == "POST" && config.MinimalPayload != nil {
		// Create POST request with minimal payload
		payloadBytes, err := json.Marshal(config.MinimalPayload)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("Failed to create test payload: %v", err)
			return result
		}

		req, err = http.NewRequestWithContext(ctx, "POST", config.TestEndpoint, strings.NewReader(string(payloadBytes)))
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("Failed to create test request: %v", err)
			return result
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		// Create GET request
		req, err = http.NewRequestWithContext(ctx, config.TestMethod, config.TestEndpoint, nil)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("Failed to create test request: %v", err)
			return result
		}
	}

	// Add authentication header
	authValue := config.AuthPrefix + apiKey
	req.Header.Set(config.AuthHeader, authValue)

	// Add provider-specific headers
	if config.Pattern == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	// Send request
	resp, err := v.client.Do(req)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to reach %s API: %v", config.Name, err)
		return result
	}
	defer resp.Body.Close()

	result.APIReachable = true
	result.StatusCode = resp.StatusCode

	// Check response status
	switch {
	case resp.StatusCode == http.StatusOK:
		// Success
		result.Valid = true
		return result

	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		// Authentication failed
		body, _ := io.ReadAll(resp.Body)
		result.ErrorMessage = fmt.Sprintf("Authentication failed (HTTP %d): Invalid API key for %s. Response: %s", resp.StatusCode, config.Name, string(body))
		return result

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// Client error (but not auth-related)
		body, _ := io.ReadAll(resp.Body)
		// For some providers, a 400 with specific error might still indicate the key is valid
		// but the test payload was rejected. We'll consider this as potentially valid.
		if config.TestMethod == "POST" && resp.StatusCode == 400 {
			// If it's a POST request and we get a 400, the key might be valid but payload invalid
			result.Valid = true
			result.ErrorMessage = fmt.Sprintf("API key appears valid, but test request returned HTTP %d (this is expected for minimal test payloads)", resp.StatusCode)
		} else {
			result.ErrorMessage = fmt.Sprintf("API request failed (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return result

	case resp.StatusCode >= 500:
		// Server error
		result.ErrorMessage = fmt.Sprintf("Provider API returned server error (HTTP %d). The API key format is correct, but the service may be unavailable.", resp.StatusCode)
		// We'll consider this as potentially valid since it's a server issue, not an auth issue
		result.Valid = true
		return result

	default:
		body, _ := io.ReadAll(resp.Body)
		result.ErrorMessage = fmt.Sprintf("Unexpected response (HTTP %d): %s", resp.StatusCode, string(body))
		return result
	}
}

// ValidateProviderConfig validates a complete provider configuration.
func (v *APIKeyValidator) ValidateProviderConfig(ctx context.Context, providerName, pattern, baseURL, apiKey string) error {
	// Get validation config
	config, err := GetValidationConfig(providerName, pattern, baseURL)
	if err != nil {
		return fmt.Errorf("failed to get validation config: %w", err)
	}

	// Validate API key
	result := v.ValidateAPIKey(ctx, apiKey, config)

	if !result.Valid {
		return fmt.Errorf("validation failed: %s", result.ErrorMessage)
	}

	return nil
}
