package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEncryptedFile(t *testing.T) {
	// Create a temporary encrypted file for testing
	tempDir := t.TempDir()
	
	encFile := &EncryptedFile{
		path: filepath.Join(tempDir, "test_secrets.db"),
		key:  []byte("test-key-32-bytes-long-123456789"), // 32 bytes for AES-256
	}
	
	testProvider := "test-provider"
	testAPIKey := "sk-test-encrypted-key"
	
	t.Run("SetAndGet", func(t *testing.T) {
		// Set API key
		err := encFile.Set(testProvider, testAPIKey)
		if err != nil {
			t.Fatalf("Failed to set API key: %v", err)
		}
		
		// Get API key
		retrievedKey, err := encFile.Get(testProvider)
		if err != nil {
			t.Fatalf("Failed to get API key: %v", err)
		}
		
		if retrievedKey != testAPIKey {
			t.Errorf("Expected %s, got %s", testAPIKey, retrievedKey)
		}
	})
	
	t.Run("GetNonExistent", func(t *testing.T) {
		_, err := encFile.Get("non-existent-provider")
		if err == nil {
			t.Error("Expected error when getting non-existent provider, got nil")
		}
	})
	
	t.Run("Update", func(t *testing.T) {
		// Set initial value
		err := encFile.Set(testProvider, testAPIKey)
		if err != nil {
			t.Fatalf("Failed to set initial API key: %v", err)
		}
		
		// Update value
		newAPIKey := "sk-test-updated-key"
		err = encFile.Set(testProvider, newAPIKey)
		if err != nil {
			t.Fatalf("Failed to update API key: %v", err)
		}
		
		// Verify update
		retrievedKey, err := encFile.Get(testProvider)
		if err != nil {
			t.Fatalf("Failed to get updated API key: %v", err)
		}
		
		if retrievedKey != newAPIKey {
			t.Errorf("Expected %s, got %s", newAPIKey, retrievedKey)
		}
	})
	
	t.Run("Delete", func(t *testing.T) {
		// Set API key
		err := encFile.Set(testProvider, testAPIKey)
		if err != nil {
			t.Fatalf("Failed to set API key: %v", err)
		}
		
		// Delete API key
		err = encFile.Delete(testProvider)
		if err != nil {
			t.Fatalf("Failed to delete API key: %v", err)
		}
		
		// Verify deletion
		_, err = encFile.Get(testProvider)
		if err == nil {
			t.Error("Expected error after deletion, got nil")
		}
	})
	
	t.Run("MultipleProviders", func(t *testing.T) {
		providers := map[string]string{
			"provider1": "key1",
			"provider2": "key2",
			"provider3": "key3",
		}
		
		// Set all providers
		for provider, apiKey := range providers {
			err := encFile.Set(provider, apiKey)
			if err != nil {
				t.Fatalf("Failed to set API key for %s: %v", provider, err)
			}
		}
		
		// Verify all providers
		for provider, expectedKey := range providers {
			retrievedKey, err := encFile.Get(provider)
			if err != nil {
				t.Fatalf("Failed to get API key for %s: %v", provider, err)
			}
			
			if retrievedKey != expectedKey {
				t.Errorf("For provider %s, expected %s, got %s", provider, expectedKey, retrievedKey)
			}
		}
		
		// Delete one provider
		err := encFile.Delete("provider2")
		if err != nil {
			t.Fatalf("Failed to delete provider2: %v", err)
		}
		
		// Verify provider2 is deleted
		_, err = encFile.Get("provider2")
		if err == nil {
			t.Error("Expected error for deleted provider2, got nil")
		}
		
		// Verify other providers still exist
		for _, provider := range []string{"provider1", "provider3"} {
			_, err := encFile.Get(provider)
			if err != nil {
				t.Errorf("Provider %s should still exist: %v", provider, err)
			}
		}
	})
	
	t.Run("EmptyFile", func(t *testing.T) {
		// Create a new encrypted file with different path
		emptyFile := &EncryptedFile{
			path: filepath.Join(tempDir, "empty_secrets.db"),
			key:  encFile.key,
		}
		
		// Try to get from non-existent file
		_, err := emptyFile.Get("any-provider")
		if err == nil {
			t.Error("Expected error when reading from non-existent file, got nil")
		}
	})
}

func TestEncryption(t *testing.T) {
	tempDir := t.TempDir()
	
	encFile := &EncryptedFile{
		path: filepath.Join(tempDir, "encryption_test.db"),
		key:  []byte("test-key-32-bytes-long-123456789"),
	}
	
	t.Run("EncryptDecrypt", func(t *testing.T) {
		plaintext := []byte("secret data to encrypt")
		
		// Encrypt
		ciphertext, err := encFile.encrypt(plaintext)
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}
		
		// Verify ciphertext is different from plaintext
		if string(ciphertext) == string(plaintext) {
			t.Error("Ciphertext should be different from plaintext")
		}
		
		// Decrypt
		decrypted, err := encFile.decrypt(ciphertext)
		if err != nil {
			t.Fatalf("Failed to decrypt: %v", err)
		}
		
		// Verify decrypted matches original
		if string(decrypted) != string(plaintext) {
			t.Errorf("Expected %s, got %s", plaintext, decrypted)
		}
	})
	
	t.Run("DecryptWithWrongKey", func(t *testing.T) {
		plaintext := []byte("secret data")
		
		// Encrypt with first key
		ciphertext, err := encFile.encrypt(plaintext)
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}
		
		// Try to decrypt with different key
		wrongKeyFile := &EncryptedFile{
			path: encFile.path,
			key:  []byte("wrong-key-32-bytes-long-876543219"),
		}
		
		_, err = wrongKeyFile.decrypt(ciphertext)
		if err == nil {
			t.Error("Expected error when decrypting with wrong key, got nil")
		}
	})
	
	t.Run("DecryptInvalidData", func(t *testing.T) {
		invalidData := []byte("not encrypted data")
		
		_, err := encFile.decrypt(invalidData)
		if err == nil {
			t.Error("Expected error when decrypting invalid data, got nil")
		}
	})
}

func TestNewEncryptedFile(t *testing.T) {
	encFile, err := NewEncryptedFile()
	if err != nil {
		t.Fatalf("Failed to create EncryptedFile: %v", err)
	}
	
	// Verify path is set
	if encFile.path == "" {
		t.Error("Expected non-empty path")
	}
	
	// Verify key is set and has correct length (32 bytes for AES-256)
	if len(encFile.key) != 32 {
		t.Errorf("Expected key length 32, got %d", len(encFile.key))
	}
	
	// Verify directory exists
	dir := filepath.Dir(encFile.path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Expected directory %s to exist", dir)
	}
}

func TestGetMachineID(t *testing.T) {
	machineID, err := getMachineID()
	if err != nil {
		t.Fatalf("Failed to get machine ID: %v", err)
	}
	
	if machineID == "" {
		t.Error("Expected non-empty machine ID")
	}
	
	// Verify consistency - calling twice should return same ID
	machineID2, err := getMachineID()
	if err != nil {
		t.Fatalf("Failed to get machine ID second time: %v", err)
	}
	
	if machineID != machineID2 {
		t.Error("Machine ID should be consistent across calls")
	}
	
	t.Logf("Machine ID: %s", machineID)
}

func TestDeriveKey(t *testing.T) {
	machineID := "test-machine-id"
	
	key := deriveKey(machineID)
	
	// Verify key length (32 bytes for AES-256)
	if len(key) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key))
	}
	
	// Verify consistency - same input should produce same key
	key2 := deriveKey(machineID)
	if string(key) != string(key2) {
		t.Error("Key derivation should be deterministic")
	}
	
	// Verify different inputs produce different keys
	key3 := deriveKey("different-machine-id")
	if string(key) == string(key3) {
		t.Error("Different machine IDs should produce different keys")
	}
}

func TestFilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	
	encFile := &EncryptedFile{
		path: filepath.Join(tempDir, "permissions_test.db"),
		key:  []byte("test-key-32-bytes-long-123456789"),
	}
	
	// Save some data
	err := encFile.Set("test", "value")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}
	
	// Check file permissions
	info, err := os.Stat(encFile.path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	
	mode := info.Mode()
	
	// On Unix-like systems, verify file is only readable/writable by owner (0600)
	// On Windows, this check may not be as strict
	if mode.Perm()&0077 != 0 {
		if runtime.GOOS != "windows" {
			t.Fatalf("Warning: File permissions are %o, expected 0600 (owner read/write only)", mode.Perm())
		}
		t.Logf("Warning: File permissions are %o, expected 0600 (owner read/write only)", mode.Perm())
		// Don't fail on Windows as permission model is different
	}
}

func TestLoadSave(t *testing.T) {
	tempDir := t.TempDir()
	
	encFile := &EncryptedFile{
		path: filepath.Join(tempDir, "load_save_test.db"),
		key:  []byte("test-key-32-bytes-long-123456789"),
	}
	
	testData := map[string]string{
		"provider1": "key1",
		"provider2": "key2",
		"provider3": "key3",
	}
	
	t.Run("SaveAndLoad", func(t *testing.T) {
		// Save data
		err := encFile.save(testData)
		if err != nil {
			t.Fatalf("Failed to save data: %v", err)
		}
		
		// Load data
		loadedData, err := encFile.load()
		if err != nil {
			t.Fatalf("Failed to load data: %v", err)
		}
		
		// Verify data matches
		if len(loadedData) != len(testData) {
			t.Errorf("Expected %d entries, got %d", len(testData), len(loadedData))
		}
		
		for key, expectedValue := range testData {
			if loadedData[key] != expectedValue {
				t.Errorf("For key %s, expected %s, got %s", key, expectedValue, loadedData[key])
			}
		}
	})
	
	t.Run("LoadNonExistentFile", func(t *testing.T) {
		nonExistentFile := &EncryptedFile{
			path: filepath.Join(tempDir, "non_existent.db"),
			key:  encFile.key,
		}
		
		_, err := nonExistentFile.load()
		if err == nil {
			t.Error("Expected error when loading non-existent file, got nil")
		}
		
		if !os.IsNotExist(err) {
			t.Errorf("Expected IsNotExist error, got %v", err)
		}
	})
}
