package config

import (
	"fmt"
	"log/slog"

	"github.com/zalando/go-keyring"
)

const (
	// ServiceName is the identifier used for OS keyring entries
	ServiceName = "ghostcli"
)

// EncryptedFileProvider defines the interface for interacting with an encrypted file.
type EncryptedFileProvider interface {
	Get(provider string) (string, error)
	Set(provider, apiKey string) error
	Delete(provider string) error
}

// KeyringProbe is a function that checks if the OS keyring is available.
type KeyringProbe func() bool

// EncryptedFileFactory is a function that returns an EncryptedFileProvider.
type EncryptedFileFactory func() (EncryptedFileProvider, error)

// SecureStorage provides secure API key storage using OS keyring with encrypted file fallback.
type SecureStorage struct {
	keyringAvailable bool
	encFile          EncryptedFileProvider
	logger           *slog.Logger
}

// NewSecureStorage creates a new SecureStorage instance.
// It attempts to use the OS keyring first, falling back to encrypted file storage if unavailable.
func NewSecureStorage(logger *slog.Logger, probe KeyringProbe, encFactory EncryptedFileFactory) (*SecureStorage, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Test if OS keyring is available by attempting a test operation
	keyringAvailable := probe()
	
	if !keyringAvailable {
		logger.Warn("OS keyring unavailable, using encrypted file fallback")
	}
	
	// Initialize encrypted file fallback
	encFile, err := encFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encrypted file storage: %w", err)
	}
	
	return &SecureStorage{
		keyringAvailable: keyringAvailable,
		encFile:          encFile,
		logger:           logger,
	}, nil
}

// SaveAPIKey stores an API key for a provider.
// It attempts to save to the OS keyring first, falling back to encrypted file if unavailable.
func (s *SecureStorage) SaveAPIKey(provider, apiKey string) error {
	// Try keyring first if available
	if s.keyringAvailable {
		err := keyring.Set(ServiceName, provider, apiKey)
		if err == nil {
			s.logger.Debug("API key saved to OS keyring", "provider", provider)
			return nil
		}
		s.logger.Warn("keyring save failed, using encrypted file", "provider", provider, "error", err)
	}
	
	// Fallback to encrypted file
	if err := s.encFile.Set(provider, apiKey); err != nil {
		return fmt.Errorf("failed to save API key to encrypted file: %w", err)
	}
	
	s.logger.Debug("API key saved to encrypted file", "provider", provider)
	return nil
}

// GetAPIKey retrieves an API key for a provider.
// It attempts to read from the OS keyring first, falling back to encrypted file if unavailable.
func (s *SecureStorage) GetAPIKey(provider string) (string, error) {
	// Try keyring first if available
	if s.keyringAvailable {
		apiKey, err := keyring.Get(ServiceName, provider)
		if err == nil {
			s.logger.Debug("API key retrieved from OS keyring", "provider", provider)
			return apiKey, nil
		}
		
		// If not found in keyring, try encrypted file
		if err == keyring.ErrNotFound {
			s.logger.Debug("API key not found in keyring, trying encrypted file", "provider", provider)
		} else {
			s.logger.Error("Error retrieving API key from keyring", "provider", provider, "error", err)
			return "", fmt.Errorf("keyring access failed for provider '%s': %w", provider, err)
		}
	}
	
	// Fallback to encrypted file
	apiKey, err := s.encFile.Get(provider)
	if err != nil {
		return "", fmt.Errorf("API key not found for provider '%s': %w", provider, err)
	}
	
	s.logger.Debug("API key retrieved from encrypted file", "provider", provider)
	return apiKey, nil
}

// DeleteAPIKey removes an API key for a provider from both keyring and encrypted file.
func (s *SecureStorage) DeleteAPIKey(provider string) error {
	var errs []error
	
	// Delete from keyring if available
	if s.keyringAvailable {
		if err := keyring.Delete(ServiceName, provider); err != nil {
			// Ignore "not found" errors
			if err != keyring.ErrNotFound {
				errs = append(errs, fmt.Errorf("keyring delete failed: %w", err))
			}
		} else {
			s.logger.Debug("API key deleted from OS keyring", "provider", provider)
		}
	}
	
	// Delete from encrypted file
	if err := s.encFile.Delete(provider); err != nil {
		errs = append(errs, fmt.Errorf("encrypted file delete failed: %w", err))
	} else {
		s.logger.Debug("API key deleted from encrypted file", "provider", provider)
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("delete errors: %v", errs)
	}
	
	return nil
}

// NewDefaultSecureStorage creates a SecureStorage with the built-in keyring probe
// and encrypted-file factory. It is a convenience wrapper for callers that don't
// need to inject custom implementations.
func NewDefaultSecureStorage(logger *slog.Logger) (*SecureStorage, error) {
	probe := func() bool {
		return testKeyringAvailability()
	}
	factory := func() (EncryptedFileProvider, error) {
		return NewEncryptedFile()
	}
	return NewSecureStorage(logger, probe, factory)
}

// testKeyringAvailability tests if the OS keyring is available and functional.
func testKeyringAvailability() bool {
	testKey := "__ghostcli_test__"
	testValue := "test"
	
	// Try to set a test value
	if err := keyring.Set(ServiceName, testKey, testValue); err != nil {
		return false
	}
	
	// Try to get the test value
	if _, err := keyring.Get(ServiceName, testKey); err != nil {
		return false
	}
	
	// Clean up test value
	_ = keyring.Delete(ServiceName, testKey)
	
	return true
}
