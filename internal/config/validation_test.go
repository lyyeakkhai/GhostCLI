package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateFormat(t *testing.T) {
	validator := NewAPIKeyValidator(nil)

	tests := []struct {
		name      string
		apiKey    string
		provider  string
		pattern   string
		wantValid bool
	}{
		{
			name:      "valid deepseek key",
			apiKey:    "sk-1234567890abcdef1234567890abcdef",
			provider:  "deepseek",
			pattern:   "openai",
			wantValid: true,
		},
		{
			name:      "invalid deepseek key - no prefix",
			apiKey:    "1234567890abcdef1234567890abcdef",
			provider:  "deepseek",
			pattern:   "openai",
			wantValid: false,
		},
		{
			name:      "invalid deepseek key - too short",
			apiKey:    "sk-short",
			provider:  "deepseek",
			pattern:   "openai",
			wantValid: false,
		},
		{
			name:      "valid kimi key",
			apiKey:    "sk-abcdefghijklmnopqrstuvwxyz123456",
			provider:  "kimi",
			pattern:   "openai",
			wantValid: true,
		},
		{
			name:      "valid openai key",
			apiKey:    "sk-proj-abcdefghijklmnopqrstuvwxyz",
			provider:  "openai",
			pattern:   "openai",
			wantValid: true,
		},
		{
			name:      "valid openai key with underscores",
			apiKey:    "sk-proj_abcdefghijklmnopqrstuvwxyz_123",
			provider:  "openai",
			pattern:   "openai",
			wantValid: true,
		},
		{
			name:      "valid anthropic key",
			apiKey:    "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
			provider:  "anthropic",
			pattern:   "anthropic",
			wantValid: true,
		},
		{
			name:      "invalid anthropic key - wrong prefix",
			apiKey:    "sk-abcdefghijklmnopqrstuvwxyz",
			provider:  "anthropic",
			pattern:   "anthropic",
			wantValid: false,
		},
		{
			name:      "valid kiro key",
			apiKey:    "kiro-1234567890abcdef",
			provider:  "kiro",
			pattern:   "aws",
			wantValid: true,
		},
		{
			name:      "valid generic key",
			apiKey:    "generic_key_1234567890",
			provider:  "unknown",
			pattern:   "openai",
			wantValid: true,
		},
		{
			name:      "invalid generic key - too short",
			apiKey:    "short",
			provider:  "unknown",
			pattern:   "openai",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetValidationConfig(tt.provider, tt.pattern, "https://api.example.com")
			if err != nil {
				t.Fatalf("GetValidationConfig failed: %v", err)
			}

			got := validator.ValidateFormat(tt.apiKey, config)
			if got != tt.wantValid {
				t.Errorf("ValidateFormat() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestGetValidationConfig(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		pattern      string
		baseURL      string
		wantPattern  string
		wantEndpoint string
		wantMethod   string
		wantHeader   string
	}{
		{
			name:         "deepseek config",
			provider:     "deepseek",
			pattern:      "openai",
			baseURL:      "https://api.deepseek.com",
			wantPattern:  "openai",
			wantEndpoint: "https://api.deepseek.com/models",
			wantMethod:   "GET",
			wantHeader:   "Authorization",
		},
		{
			name:         "kimi config",
			provider:     "kimi",
			pattern:      "openai",
			baseURL:      "https://api.moonshot.cn",
			wantPattern:  "openai",
			wantEndpoint: "https://api.moonshot.cn/models",
			wantMethod:   "GET",
			wantHeader:   "Authorization",
		},
		{
			name:         "openai config",
			provider:     "openai",
			pattern:      "openai",
			baseURL:      "https://api.openai.com/v1",
			wantPattern:  "openai",
			wantEndpoint: "https://api.openai.com/v1/models",
			wantMethod:   "GET",
			wantHeader:   "Authorization",
		},
		{
			name:         "anthropic config",
			provider:     "anthropic",
			pattern:      "anthropic",
			baseURL:      "https://api.anthropic.com",
			wantPattern:  "anthropic",
			wantEndpoint: "https://api.anthropic.com/v1/messages",
			wantMethod:   "POST",
			wantHeader:   "x-api-key",
		},
		{
			name:         "kiro config",
			provider:     "kiro",
			pattern:      "aws",
			baseURL:      "https://api.kiro.ai",
			wantPattern:  "aws",
			wantEndpoint: "https://api.kiro.ai/health",
			wantMethod:   "GET",
			wantHeader:   "X-API-Key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetValidationConfig(tt.provider, tt.pattern, tt.baseURL)
			if err != nil {
				t.Fatalf("GetValidationConfig failed: %v", err)
			}

			if config.Pattern != tt.wantPattern {
				t.Errorf("Pattern = %v, want %v", config.Pattern, tt.wantPattern)
			}
			if config.TestEndpoint != tt.wantEndpoint {
				t.Errorf("TestEndpoint = %v, want %v", config.TestEndpoint, tt.wantEndpoint)
			}
			if config.TestMethod != tt.wantMethod {
				t.Errorf("TestMethod = %v, want %v", config.TestMethod, tt.wantMethod)
			}
			if config.AuthHeader != tt.wantHeader {
				t.Errorf("AuthHeader = %v, want %v", config.AuthHeader, tt.wantHeader)
			}
		})
	}
}

func TestValidateAPIKey_Success(t *testing.T) {
	// Create mock server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			authHeader = r.Header.Get("x-api-key")
		}
		if authHeader == "" {
			authHeader = r.Header.Get("X-API-Key")
		}

		if authHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{
				{"id": "model-1"},
			},
		})
	}))
	defer server.Close()

	validator := NewAPIKeyValidator(nil)
	config := &ProviderValidationConfig{
		Name:         "test",
		Pattern:      "openai",
		KeyRegex:     DeepSeekKeyRegex,
		TestEndpoint: server.URL + "/models",
		TestMethod:   "GET",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
	}

	ctx := context.Background()
	result := validator.ValidateAPIKey(ctx, "sk-1234567890abcdef1234567890abcdef", config)

	if !result.Valid {
		t.Errorf("ValidateAPIKey() failed: %s", result.ErrorMessage)
	}
	if !result.FormatValid {
		t.Error("ValidateAPIKey() format validation failed")
	}
	if !result.APIReachable {
		t.Error("ValidateAPIKey() API not reachable")
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("ValidateAPIKey() status code = %d, want %d", result.StatusCode, http.StatusOK)
	}
}

func TestValidateAPIKey_InvalidFormat(t *testing.T) {
	validator := NewAPIKeyValidator(nil)
	config := &ProviderValidationConfig{
		Name:         "test",
		Pattern:      "openai",
		KeyRegex:     DeepSeekKeyRegex,
		TestEndpoint: "https://api.example.com/models",
		TestMethod:   "GET",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
	}

	ctx := context.Background()
	result := validator.ValidateAPIKey(ctx, "invalid-key", config)

	if result.Valid {
		t.Error("ValidateAPIKey() should fail for invalid format")
	}
	if result.FormatValid {
		t.Error("ValidateAPIKey() format should be invalid")
	}
	if result.ErrorMessage == "" {
		t.Error("ValidateAPIKey() should return error message")
	}
}

func TestValidateAPIKey_Unauthorized(t *testing.T) {
	// Create mock server that returns 401 Unauthorized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Invalid API key",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	validator := NewAPIKeyValidator(nil)
	config := &ProviderValidationConfig{
		Name:         "test",
		Pattern:      "openai",
		KeyRegex:     DeepSeekKeyRegex,
		TestEndpoint: server.URL + "/models",
		TestMethod:   "GET",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
	}

	ctx := context.Background()
	result := validator.ValidateAPIKey(ctx, "sk-1234567890abcdef1234567890abcdef", config)

	if result.Valid {
		t.Error("ValidateAPIKey() should fail for unauthorized")
	}
	if !result.FormatValid {
		t.Error("ValidateAPIKey() format should be valid")
	}
	if !result.APIReachable {
		t.Error("ValidateAPIKey() API should be reachable")
	}
	if result.StatusCode != http.StatusUnauthorized {
		t.Errorf("ValidateAPIKey() status code = %d, want %d", result.StatusCode, http.StatusUnauthorized)
	}
}

func TestValidateAPIKey_ServerError(t *testing.T) {
	// Create mock server that returns 500 Server Error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Internal server error",
		})
	}))
	defer server.Close()

	validator := NewAPIKeyValidator(nil)
	config := &ProviderValidationConfig{
		Name:         "test",
		Pattern:      "openai",
		KeyRegex:     DeepSeekKeyRegex,
		TestEndpoint: server.URL + "/models",
		TestMethod:   "GET",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
	}

	ctx := context.Background()
	result := validator.ValidateAPIKey(ctx, "sk-1234567890abcdef1234567890abcdef", config)

	// Server errors should be considered as potentially valid (server issue, not auth issue)
	if !result.Valid {
		t.Error("ValidateAPIKey() should consider server errors as potentially valid")
	}
	if !result.FormatValid {
		t.Error("ValidateAPIKey() format should be valid")
	}
	if !result.APIReachable {
		t.Error("ValidateAPIKey() API should be reachable")
	}
	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("ValidateAPIKey() status code = %d, want %d", result.StatusCode, http.StatusInternalServerError)
	}
}

func TestValidateAPIKey_Timeout(t *testing.T) {
	// Create mock server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	validator := NewAPIKeyValidator(nil)
	validator.client.Timeout = 100 * time.Millisecond

	config := &ProviderValidationConfig{
		Name:         "test",
		Pattern:      "openai",
		KeyRegex:     DeepSeekKeyRegex,
		TestEndpoint: server.URL + "/models",
		TestMethod:   "GET",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
	}

	ctx := context.Background()
	result := validator.ValidateAPIKey(ctx, "sk-1234567890abcdef1234567890abcdef", config)

	if result.Valid {
		t.Error("ValidateAPIKey() should fail on timeout")
	}
	if !result.FormatValid {
		t.Error("ValidateAPIKey() format should be valid")
	}
	if result.APIReachable {
		t.Error("ValidateAPIKey() API should not be reachable on timeout")
	}
}

func TestValidateAPIKey_POSTRequest(t *testing.T) {
	// Create mock server for POST requests (like Anthropic)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Verify auth header
		authHeader := r.Header.Get("x-api-key")
		if authHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// For minimal test payloads, providers might return 400
		// but this still indicates the key is valid
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "max_tokens too small",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	validator := NewAPIKeyValidator(nil)
	config := &ProviderValidationConfig{
		Name:         "anthropic",
		Pattern:      "anthropic",
		KeyRegex:     AnthropicKeyRegex,
		TestEndpoint: server.URL + "/v1/messages",
		TestMethod:   "POST",
		AuthHeader:   "x-api-key",
		AuthPrefix:   "",
		MinimalPayload: map[string]interface{}{
			"model":      "claude-3-5-sonnet-20241022",
			"max_tokens": 1,
			"messages": []map[string]interface{}{
				{"role": "user", "content": "Hi"},
			},
		},
	}

	ctx := context.Background()
	result := validator.ValidateAPIKey(ctx, "sk-ant-api03-abcdefghijklmnopqrstuvwxyz", config)

	// For POST requests with 400, we consider the key valid
	if !result.Valid {
		t.Errorf("ValidateAPIKey() should consider 400 on POST as valid: %s", result.ErrorMessage)
	}
	if !result.FormatValid {
		t.Error("ValidateAPIKey() format should be valid")
	}
	if !result.APIReachable {
		t.Error("ValidateAPIKey() API should be reachable")
	}
}

func TestValidateAPIKey_POSTRequest_Non400(t *testing.T) {
	// Create mock server for POST requests returning 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	validator := NewAPIKeyValidator(nil)
	config := &ProviderValidationConfig{
		Name:         "anthropic",
		Pattern:      "anthropic",
		KeyRegex:     AnthropicKeyRegex,
		TestEndpoint: server.URL + "/v1/messages",
		TestMethod:   "POST",
		AuthHeader:   "x-api-key",
	}

	ctx := context.Background()
	result := validator.ValidateAPIKey(ctx, "sk-ant-api03-abcdefghijklmnopqrstuvwxyz", config)

	// For POST requests with 404, we consider the key invalid
	if result.Valid {
		t.Error("ValidateAPIKey() should consider 404 on POST as invalid")
	}
}

func TestValidateProviderConfig(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{{"id": "model-1"}},
		})
	}))
	defer server.Close()

	validator := NewAPIKeyValidator(nil)
	ctx := context.Background()

	err := validator.ValidateProviderConfig(ctx, "deepseek", "openai", server.URL, "sk-1234567890abcdef1234567890abcdef")
	if err != nil {
		t.Errorf("ValidateProviderConfig() failed: %v", err)
	}
}

func TestValidateProviderConfig_InvalidKey(t *testing.T) {
	// Create mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	validator := NewAPIKeyValidator(nil)
	ctx := context.Background()

	err := validator.ValidateProviderConfig(ctx, "deepseek", "openai", server.URL, "sk-1234567890abcdef1234567890abcdef")
	if err == nil {
		t.Error("ValidateProviderConfig() should fail for invalid key")
	}
}
