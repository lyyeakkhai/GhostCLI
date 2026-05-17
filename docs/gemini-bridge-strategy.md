# [DRAFT] Pattern D: The Gemini Bridge Strategy (Free Tier Access)

> **⚠️ PROPOSAL ONLY:** This document represents a proposed architectural pattern for future implementation. It is currently in the research phase and has not been finalized or implemented.

The **Gemini Bridge** is a specialized connection pattern that allows GhostCLI to provide free access to Gemini models by "borrowing" the active session from the official Gemini CLI.

## 1. The Concept: "Session Hijacking" for Good
Unlike other providers that require a static API Key, the Gemini Bridge leverages the fact that the user is already authenticated with Google on their local machine. 

**The Workflow:**
1.  **Requirement:** The user must have the official Gemini CLI installed and run `gemini auth login`.
2.  **Detection:** When GhostCLI starts in `gemini-free` mode, it searches the local machine for Google's Application Default Credentials (ADC) or the Gemini CLI's internal token cache.
3.  **Bridge:** The engine extracts the active `OAuth2 Access Token`.
4.  **Injection:** The engine injects this token into the `Authorization` header for every request sent to the Gemini API.

---

## 2. Technical Architecture

```text
[ Claude Code ]
      ↓ (Anthropic JSON)
[ Ghost Engine ]
      ├── 1. Intercept Request
      ├── 2. READ local Gemini CLI Token (~/.config/gcloud/...)
      ├── 3. Wrap in Google Vertex/AI-Studio format
      └── 4. Forward to Google API with "Borrowed" Token
```

---

## 3. Implementation Details

### A. Locating the Token
The engine will look in standard locations used by Google's auth libraries:
- **Windows:** `%APPDATA%\gcloud\access_tokens.db`
- **macOS/Linux:** `~/.config/gcloud/access_tokens.db`
- **Environment:** Check `$GOOGLE_APPLICATION_CREDENTIALS`

### B. Token Refresh Loop
Since OAuth tokens expire (usually every 60 minutes), the Bridge Translator must be "Refresh-Aware":
1.  If a request returns a `401 Unauthorized`, the engine immediately re-scans the local Gemini CLI folder to see if a new token has been generated.
2.  If no new token is found, it prompts the user: *"Your Gemini session has expired. Please run 'gemini auth login' to refresh."*

---

## 4. User Experience (The "Zero-Key" Setup)

**Setup Flow:**
1.  User: `npm install -g @google/gemini-cli`
2.  User: `gemini auth login`
3.  User: `ghost --provider gemini`
4.  **GhostCLI:** *"Detected active Google session. Connecting to Gemini Flash 2.0... Ready!"*

---

## 5. Strategic Benefits
- **$0 Cost:** Uses the user's personal free tier quota from Google.
- **High Security:** GhostCLI never sees the user's password; it only uses a short-lived temporary token.
- **Competitive Edge:** This makes GhostCLI the only tool that can give you "Claude Code UX" for absolutely zero dollars.

## 6. Risks & Maintenance
- **Internal API Changes:** If Google changes the location of their token cache, the Bridge needs an update.
- **Rate Limits:** We are limited by the user's personal Google account quota (which is usually generous for individual use).
