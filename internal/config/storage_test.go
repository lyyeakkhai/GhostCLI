package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestSecureStorage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	// Create storage instance
	storage, err := NewSecureStorage(logger, func() bool { return true }, func() (EncryptedFileProvider, error) {
		return &EncryptedFile{
			path: filepath.Join(t.TempDir(), "test_secrets.db"),
			key:  []byte("test-key-32-bytes-long-123456789"),
		}, nil
	})
	if err != nil {
		t.Fatalf("Failed to create SecureStorage: %v", err)
	}
	
	// Test provider and API key
	testProvider := "test-provider"
	testAPIKey := "sk-test-1234567890"
	
	// Clean up before test
	_ = storage.DeleteAPIKey(testProvider)
	
	t.Run("SaveAndGetAPIKey", func(t *testing.T) {
		// Save API key
		err := storage.SaveAPIKey(testProvider, testAPIKey)
		if err != nil {
			t.Fatalf("Failed to save API key: %v", err)
		}
		
		// Retrieve API key
		retrievedKey, err := storage.GetAPIKey(testProvider)
		if err != nil {
			t.Fatalf("Failed to get API key: %v", err)
		}
		
		if retrievedKey != testAPIKey {
			t.Errorf("Expected API key %s, got %s", testAPIKey, retrievedKey)
		}
	})
	
	t.Run("GetNonExistentAPIKey", func(t *testing.T) {
		_, err := storage.GetAPIKey("non-existent-provider")
		if err == nil {
			t.Error("Expected error when getting non-existent API key, got nil")
		}
	})
	
	t.Run("DeleteAPIKey", func(t *testing.T) {
		// Save API key first
		err := storage.SaveAPIKey(testProvider, testAPIKey)
		if err != nil {
			t.Fatalf("Failed to save API key: %v", err)
		}
		
		// Delete API key
		err = storage.DeleteAPIKey(testProvider)
		if err != nil {
			t.Fatalf("Failed to delete API key: %v", err)
		}
		
		// Verify deletion
		_, err = storage.GetAPIKey(testProvider)
		if err == nil {
			t.Error("Expected error after deletion, got nil")
		}
	})
	
	t.Run("UpdateAPIKey", func(t *testing.T) {
		// Save initial API key
		err := storage.SaveAPIKey(testProvider, testAPIKey)
		if err != nil {
			t.Fatalf("Failed to save API key: %v", err)
		}
		
		// Update with new API key
		newAPIKey := "sk-test-updated-9876543210"
		err = storage.SaveAPIKey(testProvider, newAPIKey)
		if err != nil {
			t.Fatalf("Failed to update API key: %v", err)
		}
		
		// Retrieve and verify
		retrievedKey, err := storage.GetAPIKey(testProvider)
		if err != nil {
			t.Fatalf("Failed to get API key: %v", err)
		}
		
		if retrievedKey != newAPIKey {
			t.Errorf("Expected updated API key %s, got %s", newAPIKey, retrievedKey)
		}
		
		// Clean up
		_ = storage.DeleteAPIKey(testProvider)
	})
	
	t.Run("MultipleProviders", func(t *testing.T) {
		providers := map[string]string{
			"deepseek": "sk-deepseek-123",
			"openai":   "sk-openai-456",
			"kimi":     "sk-kimi-789",
		}
		
		// Save all providers
		for provider, apiKey := range providers {
			err := storage.SaveAPIKey(provider, apiKey)
			if err != nil {
				t.Fatalf("Failed to save API key for %s: %v", provider, err)
			}
		}
		
		// Retrieve and verify all providers
		for provider, expectedKey := range providers {
			retrievedKey, err := storage.GetAPIKey(provider)
			if err != nil {
				t.Fatalf("Failed to get API key for %s: %v", provider, err)
			}
			
			if retrievedKey != expectedKey {
				t.Errorf("For provider %s, expected %s, got %s", provider, expectedKey, retrievedKey)
			}
		}
		
		// Clean up
		for provider := range providers {
			_ = storage.DeleteAPIKey(provider)
		}
	})
}

func TestKeyringAvailability(t *testing.T) {
	available := testKeyringAvailability()
	t.Logf("OS keyring available: %v", available)
	
	// This test just logs the availability status
	// It doesn't fail because keyring availability depends on the OS environment
}

func TestSecureStorageFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	// Create storage instance
	storage, err := NewSecureStorage(logger, func() bool { return true }, func() (EncryptedFileProvider, error) {
		return &EncryptedFile{
			path: filepath.Join(t.TempDir(), "test_secrets.db"),
			key:  []byte("test-key-32-bytes-long-123456789"),
		}, nil
	})
	if err != nil {
		t.Fatalf("Failed to create SecureStorage: %v", err)
	}
	
	testProvider := "fallback-test-provider"
	testAPIKey := "sk-fallback-test-key"
	
	// Clean up before test
	_ = storage.DeleteAPIKey(testProvider)
	
	// If keyring is available, temporarily disable it to test fallback
	originalAvailability := storage.keyringAvailable
	storage.keyringAvailable = false
	
	t.Run("FallbackToEncryptedFile", func(t *testing.T) {
		// Save API key (should use encrypted file)
		err := storage.SaveAPIKey(testProvider, testAPIKey)
		if err != nil {
			t.Fatalf("Failed to save API key to encrypted file: %v", err)
		}
		
		// Retrieve API key (should use encrypted file)
		retrievedKey, err := storage.GetAPIKey(testProvider)
		if err != nil {
			t.Fatalf("Failed to get API key from encrypted file: %v", err)
		}
		
		if retrievedKey != testAPIKey {
			t.Errorf("Expected API key %s, got %s", testAPIKey, retrievedKey)
		}
		
		// Clean up
		_ = storage.DeleteAPIKey(testProvider)
	})
	
	// Restore original availability
	storage.keyringAvailable = originalAvailability
}

func TestServiceName(t *testing.T) {
	if ServiceName != "ghostcli" {
		t.Errorf("Expected ServiceName to be 'ghostcli', got '%s'", ServiceName)
	}
}

// TestKeyringIntegration tests direct keyring operations
func TestKeyringIntegration(t *testing.T) {
	// Skip if keyring is not available
	if !testKeyringAvailability() {
		t.Skip("OS keyring not available, skipping keyring integration test")
	}
	
	testProvider := "keyring-integration-test"
	testAPIKey := "sk-keyring-test-key"
	
	// Clean up before test
	_ = keyring.Delete(ServiceName, testProvider)
	
	t.Run("DirectKeyringOperations", func(t *testing.T) {
		// Set
		err := keyring.Set(ServiceName, testProvider, testAPIKey)
		if err != nil {
			t.Fatalf("Failed to set keyring value: %v", err)
		}
		
		// Get
		retrievedKey, err := keyring.Get(ServiceName, testProvider)
		if err != nil {
			t.Fatalf("Failed to get keyring value: %v", err)
		}
		
		if retrievedKey != testAPIKey {
			t.Errorf("Expected %s, got %s", testAPIKey, retrievedKey)
		}
		
		// Delete
		err = keyring.Delete(ServiceName, testProvider)
		if err != nil {
			t.Fatalf("Failed to delete keyring value: %v", err)
		}
		
		// Verify deletion
		_, err = keyring.Get(ServiceName, testProvider)
		if err != keyring.ErrNotFound {
			t.Errorf("Expected ErrNotFound after deletion, got %v", err)
		}
	})
}
