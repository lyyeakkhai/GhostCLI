# Security

> **Component**: Cross-Cutting | **Layer**: 5 (Security) | **Related**: [cli.md](./cli.md)

The Security component handles secure storage and retrieval of API keys using OS-native keyrings and encrypted fallback storage.

## Overview

Storing API keys in plain-text files is a security risk. GhostCLI implements a **tiered storage strategy** that prioritizes security:

```
┌─────────────────────────────────────────────────┐
│         Tiered Storage Strategy                 │
│                                                 │
│  Tier 1: OS Keyring (Preferred)                │
│    ├─ macOS: Keychain (TouchID support)        │
│    ├─ Windows: Credential Manager              │
│    └─ Linux: Secret Service (DBus)             │
│                                                 │
│  Tier 2: Encrypted File (Fallback)             │
│    └─ AES encryption with machine salt         │
│                                                 │
│  Tier 3: Environment Variables (Dev/CI)        │
│    └─ Session-only, not persisted              │
│                                                 │
└─────────────────────────────────────────────────┘
```

## Tier 1: OS-Native Keyring (Recommended)

### Implementation

```go
package security

import (
    "github.com/zalando/go-keyring"
)

const serviceName = "ghostcli"

// SaveAPIKey stores an API key in the OS keyring
func SaveAPIKey(provider, apiKey string) error {
    return keyring.Set(serviceName, provider, apiKey)
}

// LoadAPIKey retrieves an API key from the OS keyring
func LoadAPIKey(provider string) (string, error) {
    return keyring.Get(serviceName, provider)
}

// DeleteAPIKey removes an API key from the OS keyring
func DeleteAPIKey(provider string) error {
    return keyring.Delete(serviceName, provider)
}

// ClearAllKeys removes all API keys for GhostCLI
func ClearAllKeys() error {
    providers := []string{"deepseek", "kimi", "kiro", "anthropic", "openai"}
    
    for _, provider := range providers {
        // Ignore errors (key might not exist)
        keyring.Delete(serviceName, provider)
    }
    
    return nil
}
```

### Platform-Specific Behavior

#### macOS (Keychain)

```
Service: ghostcli
Account: deepseek
Secret: sk-...

Features:
- TouchID/FaceID protection
- Encrypted database
- System-level isolation
```

**First Access Prompt**:
```
"ghostcli" wants to access key "deepseek" in your keychain.

[Deny] [Allow] [Always Allow]
```

#### Windows (Credential Manager)

```
Target: ghostcli:deepseek
Username: ghostcli
Password: sk-...

Features:
- Windows Credential Manager
- Encrypted storage
- User-level isolation
```

#### Linux (Secret Service)

```
Service: ghostcli
Attribute: provider=deepseek
Secret: sk-...

Features:
- DBus Secret Service API
- GNOME Keyring / KWallet
- Encrypted storage
```

## Tier 2: Encrypted Local File (Fallback)

When OS keyring is unavailable (headless servers, CI/CD), fall back to encrypted file storage.

### Implementation

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "io"
)

type EncryptedStorage struct {
    filePath string
    key      []byte
}

func NewEncryptedStorage() (*EncryptedStorage, error) {
    // Derive encryption key from machine UUID
    machineID, err := getMachineID()
    if err != nil {
        return nil, err
    }
    
    key := deriveKey(machineID)
    
    return &EncryptedStorage{
        filePath: filepath.Join(os.Getenv("HOME"), ".config", "ghostcli", "secrets.enc"),
        key:      key,
    }, nil
}

func (s *EncryptedStorage) Save(provider, apiKey string) error {
    // Load existing secrets
    secrets, _ := s.loadAll()
    
    // Update
    secrets[provider] = apiKey
    
    // Encrypt
    encrypted, err := s.encrypt(secrets)
    if err != nil {
        return err
    }
    
    // Write to file
    return ioutil.WriteFile(s.filePath, encrypted, 0600)
}

func (s *EncryptedStorage) Load(provider string) (string, error) {
    secrets, err := s.loadAll()
    if err != nil {
        return "", err
    }
    
    apiKey, ok := secrets[provider]
    if !ok {
        return "", fmt.Errorf("no API key found for provider: %s", provider)
    }
    
    return apiKey, nil
}

func (s *EncryptedStorage) encrypt(data map[string]string) ([]byte, error) {
    // Marshal to JSON
    plaintext, err := json.Marshal(data)
    if err != nil {
        return nil, err
    }
    
    // Create cipher
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, err
    }
    
    // Create GCM
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    // Generate nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    
    // Encrypt
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    
    return ciphertext, nil
}

func (s *EncryptedStorage) decrypt(ciphertext []byte) (map[string]string, error) {
    // Create cipher
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, err
    }
    
    // Create GCM
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    // Extract nonce
    nonceSize := gcm.NonceSize()
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    
    // Decrypt
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, err
    }
    
    // Unmarshal
    var secrets map[string]string
    if err := json.Unmarshal(plaintext, &secrets); err != nil {
        return nil, err
    }
    
    return secrets, nil
}

func getMachineID() (string, error) {
    // Platform-specific machine ID retrieval
    // macOS: IOPlatformUUID
    // Linux: /etc/machine-id
    // Windows: MachineGuid registry key
    
    // Simplified example
    return os.Hostname()
}

func deriveKey(machineID string) []byte {
    // Use PBKDF2 or similar to derive 32-byte key
    hash := sha256.Sum256([]byte(machineID + "ghostcli-salt"))
    return hash[:]
}
```

### Warning Display

```go
func SaveAPIKeyWithFallback(provider, apiKey string) error {
    // Try OS keyring first
    if err := SaveAPIKey(provider, apiKey); err == nil {
        return nil
    }
    
    // Fall back to encrypted file
    log.Println("⚠️  Warning: OS keyring unavailable, using encrypted file storage")
    log.Println("   For better security, ensure your system keyring is configured")
    
    storage, err := NewEncryptedStorage()
    if err != nil {
        return err
    }
    
    return storage.Save(provider, apiKey)
}
```

## Tier 3: Environment Variables (Development)

For development and CI/CD environments:

```bash
# Provider-specific
export DEEPSEEK_API_KEY=sk-...
export KIMI_API_KEY=sk-...
export KIRO_API_KEY=ksk_...

# Session-only mode (no persistence)
ghostcli --provider deepseek --no-store
```

## Unified API Key Loading

```go
func LoadAPIKey(provider string) (string, error) {
    // 1. Try OS keyring
    if key, err := keyring.Get(serviceName, provider); err == nil {
        return key, nil
    }
    
    // 2. Try encrypted file
    storage, err := NewEncryptedStorage()
    if err == nil {
        if key, err := storage.Load(provider); err == nil {
            return key, nil
        }
    }
    
    // 3. Try environment variable
    envVar := strings.ToUpper(provider) + "_API_KEY"
    if key := os.Getenv(envVar); key != "" {
        return key, nil
    }
    
    return "", fmt.Errorf("no API key found for provider: %s", provider)
}
```

## Security Best Practices

### API Key Validation

```go
func ValidateAPIKey(provider, apiKey string) error {
    // Basic format validation
    switch provider {
    case "deepseek", "kimi", "openai":
        if !strings.HasPrefix(apiKey, "sk-") {
            return errors.New("invalid API key format (expected sk-...)")
        }
    case "kiro":
        if !strings.HasPrefix(apiKey, "ksk_") {
            return errors.New("invalid API key format (expected ksk_...)")
        }
    case "anthropic":
        if !strings.HasPrefix(apiKey, "sk-ant-") {
            return errors.New("invalid API key format (expected sk-ant-...)")
        }
    }
    
    // Optional: Test API key with provider
    return testAPIKey(provider, apiKey)
}
```

### Clear Keys Command

```bash
# Clear all stored API keys
ghostcli --clear-keys

# Clear specific provider
ghostcli --clear-key deepseek
```

```go
func ClearKeys(provider string) error {
    if provider == "" {
        // Clear all
        return ClearAllKeys()
    }
    
    // Clear specific provider
    keyring.Delete(serviceName, provider)
    
    storage, _ := NewEncryptedStorage()
    storage.Delete(provider)
    
    log.Printf("✓ Cleared API key for %s", provider)
    return nil
}
```

### Session-Only Mode

```bash
# Don't persist API key
ghostcli --provider deepseek --api-key sk-... --no-store
```

```go
func runServer(cmd *cobra.Command, args []string) {
    apiKey := viper.GetString("api-key")
    noStore := viper.GetBool("no-store")
    
    if apiKey != "" && !noStore {
        // Save to keyring
        SaveAPIKey(config.Provider, apiKey)
    }
    
    // Continue with server startup...
}
```

## Network Security

### Local-Only Binding

```go
// ✅ Bind to localhost only (secure)
server := &http.Server{
    Addr: "127.0.0.1:3200",
}

// ❌ Bind to all interfaces (insecure)
server := &http.Server{
    Addr: ":3200",  // Don't do this!
}
```

### HTTPS for Upstream

```go
// All provider connections use HTTPS
var httpClient = &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    },
}
```

### Certificate Validation

```go
// Verify provider certificates
transport := &http.Transport{
    TLSClientConfig: &tls.Config{
        InsecureSkipVerify: false,  // Always verify
    },
}
```

## Threat Model

### In Scope

- **API key theft from disk**: Mitigated by OS keyring encryption
- **Man-in-the-middle attacks**: Mitigated by HTTPS upstream
- **Unauthorized proxy access**: Mitigated by localhost-only binding

### Out of Scope

- **Malicious Claude Code client**: Assumed trusted
- **Compromised OS keyring**: OS-level security issue
- **Physical machine access**: Physical security required

## Related Documentation

- [CLI](./cli.md) - Configuration and API key loading
- [Architecture Overview](../architecture/overview.md) - Security architecture
