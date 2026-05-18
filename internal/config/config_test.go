package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Port != 3200 {
		t.Errorf("expected default port 3200, got %d", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected default host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.Timeout != 300 {
		t.Errorf("expected default timeout 300, got %d", cfg.Timeout)
	}
	if cfg.CORSOrigin != "*" {
		t.Errorf("expected default CORS origin *, got %s", cfg.CORSOrigin)
	}
	if cfg.Verbose {
		t.Error("expected verbose to be false by default")
	}
	if cfg.Providers == nil {
		t.Error("expected providers map to be initialized")
	}
}

func TestLoadFromYAMLFile(t *testing.T) {
	// Create temporary YAML config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
port: 8080
host: "0.0.0.0"
timeout: 600
active_provider: "deepseek"
cors_origin: "https://example.com"
verbose: true
providers:
  deepseek:
    name: "deepseek"
    pattern: "openai"
    base_url: "https://api.deepseek.com"
    api_key: "sk-test123"
    model_map:
      claude-3-5-sonnet: "deepseek-chat"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath, nil)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Host)
	}
	if cfg.Timeout != 600 {
		t.Errorf("expected timeout 600, got %d", cfg.Timeout)
	}
	if cfg.ActiveProvider != "deepseek" {
		t.Errorf("expected active provider deepseek, got %s", cfg.ActiveProvider)
	}
	if cfg.CORSOrigin != "https://example.com" {
		t.Errorf("expected CORS origin https://example.com, got %s", cfg.CORSOrigin)
	}
	if !cfg.Verbose {
		t.Error("expected verbose to be true")
	}

	provider, exists := cfg.Providers["deepseek"]
	if !exists {
		t.Fatal("expected deepseek provider to exist")
	}
	if provider.Name != "deepseek" {
		t.Errorf("expected provider name deepseek, got %s", provider.Name)
	}
	if provider.Pattern != "openai" {
		t.Errorf("expected provider pattern openai, got %s", provider.Pattern)
	}
	if provider.BaseURL != "https://api.deepseek.com" {
		t.Errorf("expected base URL https://api.deepseek.com, got %s", provider.BaseURL)
	}
	if provider.APIKey != "sk-test123" {
		t.Errorf("expected API key sk-test123, got %s", provider.APIKey)
	}
	if provider.ModelMap["claude-3-5-sonnet"] != "deepseek-chat" {
		t.Errorf("expected model mapping, got %v", provider.ModelMap)
	}
}

func TestLoadFromJSONFile(t *testing.T) {
	// Create temporary JSON config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	jsonContent := `{
  "port": 9000,
  "host": "localhost",
  "timeout": 120,
  "active_provider": "openai",
  "cors_origin": "http://localhost:3000",
  "verbose": false,
  "providers": {
    "openai": {
      "name": "openai",
      "pattern": "openai",
      "base_url": "https://api.openai.com",
      "api_key": "sk-openai123",
      "model_map": {
        "claude-3-5-sonnet": "gpt-4o"
      }
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath, nil)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Port)
	}
	if cfg.ActiveProvider != "openai" {
		t.Errorf("expected active provider openai, got %s", cfg.ActiveProvider)
	}

	provider, exists := cfg.Providers["openai"]
	if !exists {
		t.Fatal("expected openai provider to exist")
	}
	if provider.APIKey != "sk-openai123" {
		t.Errorf("expected API key sk-openai123, got %s", provider.APIKey)
	}
}

func TestMergeFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("GHOST_PORT", "7777")
	os.Setenv("GHOST_HOST", "192.168.1.1")
	os.Setenv("GHOST_TIMEOUT", "999")
	os.Setenv("GHOST_PROVIDER", "kimi")
	os.Setenv("GHOST_CORS_ORIGIN", "https://env.example.com")
	os.Setenv("GHOST_VERBOSE", "true")
	defer func() {
		os.Unsetenv("GHOST_PORT")
		os.Unsetenv("GHOST_HOST")
		os.Unsetenv("GHOST_TIMEOUT")
		os.Unsetenv("GHOST_PROVIDER")
		os.Unsetenv("GHOST_CORS_ORIGIN")
		os.Unsetenv("GHOST_VERBOSE")
	}()

	cfg := DefaultConfig()
	cfg.Providers["kimi"] = ProviderConfig{
		Name:    "kimi",
		Pattern: "openai",
		BaseURL: "https://api.moonshot.cn",
		APIKey:  "test",
	}
	cfg.mergeFromEnv()

	if cfg.Port != 7777 {
		t.Errorf("expected port 7777 from env, got %d", cfg.Port)
	}
	if cfg.Host != "192.168.1.1" {
		t.Errorf("expected host 192.168.1.1 from env, got %s", cfg.Host)
	}
	if cfg.Timeout != 999 {
		t.Errorf("expected timeout 999 from env, got %d", cfg.Timeout)
	}
	if cfg.ActiveProvider != "kimi" {
		t.Errorf("expected active provider kimi from env, got %s", cfg.ActiveProvider)
	}
	if cfg.CORSOrigin != "https://env.example.com" {
		t.Errorf("expected CORS origin from env, got %s", cfg.CORSOrigin)
	}
	if !cfg.Verbose {
		t.Error("expected verbose to be true from env")
	}
}

func TestMergeFromFlags(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActiveProvider = "deepseek"
	cfg.Providers["deepseek"] = ProviderConfig{
		Name:    "deepseek",
		Pattern: "openai",
		BaseURL: "https://api.deepseek.com",
		APIKey:  "old-key",
	}

	flags := map[string]interface{}{
		"port":        5555,
		"host":        "10.0.0.1",
		"timeout":     180,
		"provider":    "deepseek",
		"api-key":     "new-key-from-flag",
		"cors-origin": "https://flag.example.com",
		"verbose":     true,
	}

	cfg.mergeFromFlags(flags)

	if cfg.Port != 5555 {
		t.Errorf("expected port 5555 from flags, got %d", cfg.Port)
	}
	if cfg.Host != "10.0.0.1" {
		t.Errorf("expected host 10.0.0.1 from flags, got %s", cfg.Host)
	}
	if cfg.Timeout != 180 {
		t.Errorf("expected timeout 180 from flags, got %d", cfg.Timeout)
	}
	if cfg.CORSOrigin != "https://flag.example.com" {
		t.Errorf("expected CORS origin from flags, got %s", cfg.CORSOrigin)
	}
	if !cfg.Verbose {
		t.Error("expected verbose to be true from flags")
	}

	provider := cfg.Providers["deepseek"]
	if provider.APIKey != "new-key-from-flag" {
		t.Errorf("expected API key to be updated from flags, got %s", provider.APIKey)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: &Config{
				Port:           3200,
				Host:           "127.0.0.1",
				Timeout:        300,
				ActiveProvider: "deepseek",
				Providers: map[string]ProviderConfig{
					"deepseek": {
						Name:    "deepseek",
						Pattern: "openai",
						BaseURL: "https://api.deepseek.com",
						APIKey:  "sk-test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid port - too low",
			cfg: &Config{
				Port:           0,
				Host:           "127.0.0.1",
				Timeout:        300,
				ActiveProvider: "deepseek",
				Providers: map[string]ProviderConfig{
					"deepseek": {
						Name:    "deepseek",
						Pattern: "openai",
						BaseURL: "https://api.deepseek.com",
						APIKey:  "sk-test",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name: "invalid port - too high",
			cfg: &Config{
				Port:           99999,
				Host:           "127.0.0.1",
				Timeout:        300,
				ActiveProvider: "deepseek",
				Providers: map[string]ProviderConfig{
					"deepseek": {
						Name:    "deepseek",
						Pattern: "openai",
						BaseURL: "https://api.deepseek.com",
						APIKey:  "sk-test",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name: "empty host",
			cfg: &Config{
				Port:           3200,
				Host:           "",
				Timeout:        300,
				ActiveProvider: "deepseek",
				Providers: map[string]ProviderConfig{
					"deepseek": {
						Name:    "deepseek",
						Pattern: "openai",
						BaseURL: "https://api.deepseek.com",
						APIKey:  "sk-test",
					},
				},
			},
			wantErr: true,
			errMsg:  "host cannot be empty",
		},
		{
			name: "invalid timeout",
			cfg: &Config{
				Port:           3200,
				Host:           "127.0.0.1",
				Timeout:        0,
				ActiveProvider: "deepseek",
				Providers: map[string]ProviderConfig{
					"deepseek": {
						Name:    "deepseek",
						Pattern: "openai",
						BaseURL: "https://api.deepseek.com",
						APIKey:  "sk-test",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid timeout",
		},
		{
			name: "missing active provider",
			cfg: &Config{
				Port:           3200,
				Host:           "127.0.0.1",
				Timeout:        300,
				ActiveProvider: "",
				Providers:      map[string]ProviderConfig{},
			},
			wantErr: true,
			errMsg:  "active_provider must be specified",
		},
		{
			name: "active provider not in providers map",
			cfg: &Config{
				Port:           3200,
				Host:           "127.0.0.1",
				Timeout:        300,
				ActiveProvider: "nonexistent",
				Providers:      map[string]ProviderConfig{},
			},
			wantErr: true,
			errMsg:  "active provider 'nonexistent' not found",
		},
		{
			name: "invalid provider pattern",
			cfg: &Config{
				Port:           3200,
				Host:           "127.0.0.1",
				Timeout:        300,
				ActiveProvider: "test",
				Providers: map[string]ProviderConfig{
					"test": {
						Name:    "test",
						Pattern: "invalid",
						BaseURL: "https://api.test.com",
						APIKey:  "sk-test",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid provider pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestProviderConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		pc      ProviderConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid provider config",
			pc: ProviderConfig{
				Name:    "deepseek",
				Pattern: "openai",
				BaseURL: "https://api.deepseek.com",
				APIKey:  "sk-test",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			pc: ProviderConfig{
				Name:    "",
				Pattern: "openai",
				BaseURL: "https://api.deepseek.com",
				APIKey:  "sk-test",
			},
			wantErr: true,
			errMsg:  "name cannot be empty",
		},
		{
			name: "empty pattern",
			pc: ProviderConfig{
				Name:    "deepseek",
				Pattern: "",
				BaseURL: "https://api.deepseek.com",
				APIKey:  "sk-test",
			},
			wantErr: true,
			errMsg:  "pattern cannot be empty",
		},
		{
			name: "invalid pattern",
			pc: ProviderConfig{
				Name:    "deepseek",
				Pattern: "unknown",
				BaseURL: "https://api.deepseek.com",
				APIKey:  "sk-test",
			},
			wantErr: true,
			errMsg:  "invalid provider pattern",
		},
		{
			name: "empty base URL",
			pc: ProviderConfig{
				Name:    "deepseek",
				Pattern: "openai",
				BaseURL: "",
				APIKey:  "sk-test",
			},
			wantErr: true,
			errMsg:  "base_url cannot be empty",
		},
		{
			name: "empty API key",
			pc: ProviderConfig{
				Name:    "deepseek",
				Pattern: "openai",
				BaseURL: "https://api.deepseek.com",
				APIKey:  "",
			},
			wantErr: true,
			errMsg:  "api_key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pc.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestGetProviderConfig(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"deepseek": {
				Name:    "deepseek",
				Pattern: "openai",
				BaseURL: "https://api.deepseek.com",
				APIKey:  "sk-test",
			},
		},
	}

	// Test existing provider
	provider, err := cfg.GetProviderConfig("deepseek")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if provider.Name != "deepseek" {
		t.Errorf("expected provider name deepseek, got %s", provider.Name)
	}

	// Test non-existent provider
	_, err = cfg.GetProviderConfig("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent provider, got nil")
	}
}

func TestGetActiveProviderConfig(t *testing.T) {
	cfg := &Config{
		ActiveProvider: "deepseek",
		Providers: map[string]ProviderConfig{
			"deepseek": {
				Name:    "deepseek",
				Pattern: "openai",
				BaseURL: "https://api.deepseek.com",
				APIKey:  "sk-test",
			},
		},
	}

	provider, err := cfg.GetActiveProviderConfig()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if provider.Name != "deepseek" {
		t.Errorf("expected provider name deepseek, got %s", provider.Name)
	}
}

func TestToEnvVarName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"deepseek", "DEEPSEEK"},
		{"openai", "OPENAI"},
		{"kimi-api", "KIMI_API"},
		{"Test123", "TEST123"},
		{"my-provider-2", "MY_PROVIDER_2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toEnvVarName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestUnsupportedFileFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.txt")

	if err := os.WriteFile(configPath, []byte("invalid"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(configPath, nil)
	if err == nil {
		t.Error("expected error for unsupported file format, got nil")
	}
	if !contains(err.Error(), "unsupported config file format") {
		t.Errorf("expected unsupported format error, got %v", err)
	}
}

func TestInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	invalidYAML := `
port: 8080
host: [invalid yaml structure
`
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(configPath, nil)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	invalidJSON := `{
  "port": 8080,
  "host": "invalid json
}`
	if err := os.WriteFile(configPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(configPath, nil)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
