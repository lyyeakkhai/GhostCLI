package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/denisbrodbeck/machineid"
)

// EncryptedFile provides encrypted local file storage for API keys.
// The encryption key is derived from the machine hardware UUID.
type EncryptedFile struct {
	path string
	key  []byte
}

// NewEncryptedFile creates a new EncryptedFile instance.
// It derives the encryption key from the machine hardware UUID.
func NewEncryptedFile() (*EncryptedFile, error) {
	// Get user config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user config directory: %w", err)
	}
	
	// Create ghost config directory
	ghostDir := filepath.Join(configDir, "ghost")
	if err := os.MkdirAll(ghostDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Derive encryption key from machine UUID
	machineID, err := getMachineID()
	if err != nil {
		return nil, fmt.Errorf("failed to get machine ID: %w", err)
	}
	
	key := deriveKey(machineID)
	
	return &EncryptedFile{
		path: filepath.Join(ghostDir, "secrets.db"),
		key:  key,
	}, nil
}

// Set stores an API key for a provider in the encrypted file.
func (e *EncryptedFile) Set(provider, apiKey string) error {
	// Load existing data
	data, err := e.load()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load existing data: %w", err)
	}
	
	if data == nil {
		data = make(map[string]string)
	}
	
	// Update data
	data[provider] = apiKey
	
	// Save data
	if err := e.save(data); err != nil {
		return fmt.Errorf("failed to save data: %w", err)
	}
	
	return nil
}

// Get retrieves an API key for a provider from the encrypted file.
func (e *EncryptedFile) Get(provider string) (string, error) {
	// Load data
	data, err := e.load()
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no stored credentials found")
		}
		return "", fmt.Errorf("failed to load data: %w", err)
	}
	
	// Get API key
	apiKey, exists := data[provider]
	if !exists {
		return "", fmt.Errorf("no API key found for provider '%s'", provider)
	}
	
	return apiKey, nil
}

// Delete removes an API key for a provider from the encrypted file.
func (e *EncryptedFile) Delete(provider string) error {
	// Load existing data
	data, err := e.load()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to delete
		}
		return fmt.Errorf("failed to load existing data: %w", err)
	}
	
	// Delete provider
	delete(data, provider)
	
	// Save data
	if err := e.save(data); err != nil {
		return fmt.Errorf("failed to save data: %w", err)
	}
	
	return nil
}

// load reads and decrypts the data from the file.
func (e *EncryptedFile) load() (map[string]string, error) {
	// Read encrypted data
	ciphertext, err := os.ReadFile(e.path)
	if err != nil {
		return nil, err
	}
	
	// Decrypt data
	plaintext, err := e.decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}
	
	// Unmarshal JSON
	var data map[string]string
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}
	
	return data, nil
}

// save encrypts and writes the data to the file.
func (e *EncryptedFile) save(data map[string]string) error {
	// Marshal to JSON
	plaintext, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	
	// Encrypt data
	ciphertext, err := e.encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt data: %w", err)
	}
	
	// Write to file with restricted permissions
	if err := os.WriteFile(e.path, ciphertext, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	return nil
}

// encrypt encrypts plaintext using AES-GCM.
func (e *EncryptedFile) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	
	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt decrypts ciphertext using AES-GCM.
func (e *EncryptedFile) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	
	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	
	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	
	return plaintext, nil
}

// getMachineID retrieves the machine hardware UUID.
func getMachineID() (string, error) {
	// Use the machineid library which handles cross-platform machine ID retrieval
	id, err := machineid.ID()
	if err != nil {
		return "", fmt.Errorf("failed to get machine ID: %w", err)
	}
	
	// Add OS-specific salt for additional uniqueness
	salt := fmt.Sprintf("ghostcli-%s-%s", runtime.GOOS, runtime.GOARCH)
	return id + salt, nil
}

// deriveKey derives a 32-byte AES-256 key from the machine ID using SHA-256.
func deriveKey(machineID string) []byte {
	hash := sha256.Sum256([]byte(machineID))
	return hash[:]
}
