# GhostCLI: Secure Local Storage Strategy

Storing API keys in plain-text files (like `.env` or `config.json`) is a security risk. For a production-grade tool like **GhostCLI**, we implement a tiered storage strategy that prioritizes security and follows OS-native best practices.

## 1. The Tiered Storage Strategy

GhostCLI attempts to store and retrieve keys in the following order of preference:

| Tier | Method | Security Level | OS Support |
| :--- | :--- | :--- | :--- |
| **Tier 1** | **OS Keyring** | High | macOS (Keychain), Windows (Credential Manager), Linux (Secret Service/DBus) |
| **Tier 2** | **Encrypted Local File** | Medium | All (Fallback) |
| **Tier 3** | **Environment Variables** | Low | All (Session-only / Dev use) |

---

## 2. Tier 1: OS-Native Keyring (Recommended)

Instead of saving the key to a file on disk, the engine "hands off" the key to the Operating System's secure storage.

### How it works:
- **Service Name:** `ghostcli`
- **Account:** The provider name (e.g., `deepseek`, `kiro`).
- **Secret:** The API Key.

### Benefits:
- **Biometric Protection:** On macOS, the system can require TouchID to unlock the keychain.
- **Isolation:** Other apps cannot easily read these keys without permission.
- **Disk Security:** The keys are stored in an encrypted database managed by the OS.

---

## 3. Tier 2: Encrypted Local Config (Fallback)

If the OS Keyring is unavailable (e.g., in a headless server environment), GhostCLI falls back to a local configuration file, but with **AES encryption**.

- **Location:** `~/.config/ghost/secrets.db`
- **Key:** A machine-specific salt (derived from the Hardware UUID) is used to encrypt the keys before writing them to disk.
- **Warning:** A warning is shown to the user if the tool is forced to fall back to Tier 2.

---

## 4. Implementation in Go

We use the [**zalando/go-keyring**](https://github.com/zalando/go-keyring) library, which is the industry standard for cross-platform secret management in Go.

### Usage Example:
```go
import "github.com/zalando/go-keyring"

const service = "ghostcli"

// Saving a key
func SaveKey(provider, key string) error {
    return keyring.Set(service, provider, key)
}

// Retrieving a key
func GetKey(provider string) (string, error) {
    return keyring.Get(service, provider)
}
```

---

## 5. User Security Best Practices

### The "Permission" Prompt
The first time GhostCLI tries to access the Keyring, the OS will show a native prompt (e.g., "ghostcli wants to access your keychain"). We should warn the user during the onboarding flow that they will see this prompt.

### "Clear" Command
Always provide a way for the user to wipe their keys:
```bash
ghost --clear-keys
```
This command should iterate through the Keyring and delete all entries associated with the `ghostcli` service.

### Support for "Session-Only" Mode
For users who do not want anything stored locally, support a `--no-store` flag that keeps the API key in memory only for that specific session.
