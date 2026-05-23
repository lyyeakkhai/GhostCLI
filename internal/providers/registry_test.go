package providers

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if registry.providers == nil {
		t.Error("Registry providers map is nil")
	}

	if len(registry.providers) != 0 {
		t.Errorf("Expected empty registry, got %d providers", len(registry.providers))
	}
}

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name        string
		providerName string
		provider    Provider
		wantErr     bool
	}{
		{
			name:        "register new provider",
			providerName: "deepseek",
			provider:    &mockProvider{name: "deepseek"},
			wantErr:     false,
		},
		{
			name:        "register another provider",
			providerName: "openai",
			provider:    &mockProvider{name: "openai"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			err := registry.Register(tt.providerName, tt.provider)

			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify provider was registered
				provider, err := registry.Get(tt.providerName)
				if err != nil {
					t.Errorf("Failed to get registered provider: %v", err)
				}
				if provider.Name() != tt.providerName {
					t.Errorf("Expected provider name %s, got %s", tt.providerName, provider.Name())
				}
			}
		})
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewRegistry()
	provider := &mockProvider{name: "deepseek"}

	// First registration should succeed
	err := registry.Register("deepseek", provider)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	// Second registration with same name should fail
	err = registry.Register("deepseek", provider)
	if err == nil {
		t.Error("Expected error when registering duplicate provider, got nil")
	}

	expectedErr := "provider deepseek already registered"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message %q, got %q", expectedErr, err.Error())
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	deepseek := &mockProvider{name: "deepseek"}
	openai := &mockProvider{name: "openai"}

	registry.Register("deepseek", deepseek)
	registry.Register("openai", openai)

	tests := []struct {
		name         string
		providerName string
		wantErr      bool
		wantProvider Provider
	}{
		{
			name:         "get existing provider deepseek",
			providerName: "deepseek",
			wantErr:      false,
			wantProvider: deepseek,
		},
		{
			name:         "get existing provider openai",
			providerName: "openai",
			wantErr:      false,
			wantProvider: openai,
		},
		{
			name:         "get non-existent provider",
			providerName: "kimi",
			wantErr:      true,
			wantProvider: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := registry.Get(tt.providerName)

			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && provider != tt.wantProvider {
				t.Errorf("Get() returned wrong provider")
			}

			if tt.wantErr && provider != nil {
				t.Error("Get() should return nil provider on error")
			}
		})
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	registry := NewRegistry()

	provider, err := registry.Get("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent provider, got nil")
	}

	if provider != nil {
		t.Error("Expected nil provider when not found")
	}

	expectedErr := "provider nonexistent not found"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message %q, got %q", expectedErr, err.Error())
	}
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	// Test empty registry
	names := registry.List()
	if len(names) != 0 {
		t.Errorf("Expected empty list, got %d items", len(names))
	}

	// Register providers in non-alphabetical order
	registry.Register("kimi", &mockProvider{name: "kimi"})
	registry.Register("deepseek", &mockProvider{name: "deepseek"})
	registry.Register("openai", &mockProvider{name: "openai"})
	registry.Register("anthropic", &mockProvider{name: "anthropic"})

	// Get list
	names = registry.List()

	// Verify count
	if len(names) != 4 {
		t.Errorf("Expected 4 providers, got %d", len(names))
	}

	// Verify alphabetical order
	expected := []string{"anthropic", "deepseek", "kimi", "openai"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected name at index %d to be %s, got %s", i, expected[i], name)
		}
	}
}

func TestRegistry_ConcurrentRegister(t *testing.T) {
	registry := NewRegistry()
	var wg sync.WaitGroup
	numGoroutines := 10

	// Attempt to register different providers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			providerName := fmt.Sprintf("provider%d", id)
			provider := &mockProvider{name: providerName}
			err := registry.Register(providerName, provider)
			if err != nil {
				t.Errorf("Concurrent registration failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all providers were registered
	names := registry.List()
	if len(names) != numGoroutines {
		t.Errorf("Expected %d providers, got %d", numGoroutines, len(names))
	}
}

func TestRegistry_ConcurrentGet(t *testing.T) {
	registry := NewRegistry()

	// Register some providers
	for i := 0; i < 5; i++ {
		providerName := fmt.Sprintf("provider%d", i)
		registry.Register(providerName, &mockProvider{name: providerName})
	}

	var wg sync.WaitGroup
	numGoroutines := 20

	// Attempt to get providers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			providerName := fmt.Sprintf("provider%d", id%5)
			provider, err := registry.Get(providerName)
			if err != nil {
				t.Errorf("Concurrent get failed: %v", err)
			}
			if provider.Name() != providerName {
				t.Errorf("Expected provider name %s, got %s", providerName, provider.Name())
			}
		}(i)
	}

	wg.Wait()
}

func TestRegistry_ConcurrentList(t *testing.T) {
	registry := NewRegistry()

	// Register some providers
	for i := 0; i < 5; i++ {
		providerName := fmt.Sprintf("provider%d", i)
		registry.Register(providerName, &mockProvider{name: providerName})
	}

	var wg sync.WaitGroup
	numGoroutines := 20

	// Attempt to list providers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			names := registry.List()
			if len(names) != 5 {
				t.Errorf("Expected 5 providers, got %d", len(names))
			}
		}()
	}

	wg.Wait()
}

func TestRegistry_ConcurrentMixed(t *testing.T) {
	registry := NewRegistry()
	var wg sync.WaitGroup
	numOperations := 50

	// Mix of register, get, and list operations
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			switch id % 3 {
			case 0: // Register
				providerName := fmt.Sprintf("provider%d", id)
				provider := &mockProvider{name: providerName}
				registry.Register(providerName, provider)
			case 1: // Get
				providerName := fmt.Sprintf("provider%d", id-1)
				registry.Get(providerName)
			case 2: // List
				registry.List()
			}
		}(i)
	}

	wg.Wait()

	// Verify registry is still functional
	names := registry.List()
	if len(names) == 0 {
		t.Error("Expected some providers to be registered")
	}
}

func TestRegistry_ListDoesNotModifyRegistry(t *testing.T) {
	registry := NewRegistry()

	registry.Register("provider1", &mockProvider{name: "provider1"})
	registry.Register("provider2", &mockProvider{name: "provider2"})

	// Get list and modify it
	names := registry.List()
	names[0] = "modified"
	names = append(names, "extra")

	// Verify registry is unchanged
	newNames := registry.List()
	if len(newNames) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(newNames))
	}

	if newNames[0] == "modified" {
		t.Error("Registry was modified by external list modification")
	}
}
