# Module 05: Security

## Overview

The Security module provides secure storage for API keys and sensitive credentials using OS-native keyrings with encrypted file fallback.

## Responsibilities

- Store API keys securely using OS-native keyring
- Provide encrypted file fallback when keyring unavailable
- Derive encryption keys from machine hardware UUID
- Support multiple providers with separate credentials
- Enable credential clearing for security/troubleshooting
- Prevent plain-text key exposure

## Architecture

```
Security
├── Secure Storage Interface
├── Tier 1: OS Keyring (Preferred)
│   ├── macOS: Keychain
│   ├── Windows: Credential Manager
│   └── Linux: Secret Service (DBus)
├── Tier 2: Encrypted File (Fallback)
│   ├── Location: ~/.config/ghost/secrets.db
│   ├── Encryption: AES-256-GCM
│   └── Key Derivation: Machine UUID
└── Tier 3: Environment Variables (Session-only)
```

## Related Requirements

- **Requirement 10**: Secure API Key Storage

## Tiered Storage Strategy

### Tier 1: OS-Native Keyring (High Security)

**macOS - Keychain**:
- Service: `ghostcli`
- Account: Provider name (e.g., `deepseek`)
- Secret: API key
- Protection: TouchID/password required

**Windows - Credential Manager**:
- Target: `ghostcli/{provider}`
- Username: Provider name
- Password: API key
- Protection: Windows user account

**Linux - Secret Service**:
- Collection: `ghostcli`
- Label: Provider name
- Secret: API key
- Protection: User session keyring

**Library**: [zalando/go-keyring](https://github.com/zalando/go-keyring)

### Tier 2: Encrypted File (Medium Security)

**When Used**:
- OS keyring unavailable (headless servers)
- Keyring access denied
- Keyring library not supported

**Location**: `~/.config/ghost/secrets.db`

**Encryption**:
- Algorithm: AES-256-GCM
- Key Derivation: PBKDF2 from machine UUID
- Salt: Random 32-byte salt per file
- Iterations: 100,000

**Format**:
```json
{
  "deepseek": "sk-...",
  "kimi": "sk-...",
  "openai": "sk-..."
}
```

### Tier 3: Environment Variables (Low Security)

**When Used**:
- `--no-store` flag provided
- Development/testing
- CI/CD pipelines

**Format**:
```bash
export DEEPSEEK_API_KEY=sk-...
export KIMI_API_KEY=sk-...
```

## Security Best Practices

### Key Storage
- Never store keys in plain text
- Use OS keyring when available
- Encrypt fallback storage
- Restrict file permissions (0600)

### Key Retrieval
- Try keyring first
- Fall back to encrypted file
- Log security tier used
- Warn on fallback usage

### Key Deletion
- Delete from all tiers
- Prompt for confirmation
- Support --force flag
- Log deletion success/failure

## Implementation Details

See [design.md](./design.md) for detailed implementation specifications.
