# GhostCLI: Interactive Onboarding & Startup Flow

To provide a "best-in-class" user experience, **GhostCLI** features an interactive first-run wizard. This ensures that users don't have to manually edit `.env` files or learn complex flags to get started.

## 1. The First-Run Experience

When a user runs the `ghost` command for the first time (or when no configuration is found), the tool enters **Setup Mode**.

### Step A: The Welcome Screen
The CLI clears the terminal and shows a high-impact welcome message:
```text
╔════════════════════════════════════════════════════════════╗
║                                                            ║
║    G H O S T C L I                                         ║
║    Use Claude Code with any provider, 10x cheaper.         ║
║                                                            ║
╚════════════════════════════════════════════════════════════╝

This tool acts as a bridge between Claude Code and models like 
DeepSeek, Kiro, and Kimi.
```

### Step B: Provider Selection
The user is presented with a searchable list of providers. Each provider is tagged with its **Pattern Family**.

**UI Mockup:**
```text
? Select your preferred provider:
  > DeepSeek (Pattern A: OpenAI-Compat)
    Kiro (Pattern C: AWS/EventStream)
    Kimi (Pattern A: OpenAI-Compat)
    Anthropic (Pattern B: Native)
    Custom...
```

### Step C: API Key Input
The input is masked for security. The engine performs a "Regex-light" check to ensure the key looks correct (e.g., starts with `sk-` or `ksk_`).

**UI Mockup:**
```text
? Enter your API Key for DeepSeek:
  ********************************************
```

---

## 2. The Persistence Layer

Once the user enters their info, GhostCLI saves it to a persistent config file (e.g., `~/.config/ghost/config.json` or a local `.env`).

**Saved Config Example:**
```json
{
  "active_provider": "deepseek",
  "providers": {
    "deepseek": {
        "api_key": "sk-...",
        "pattern": "openai"
    }
  }
}
```

---

## 3. The "Auto-Launch" Sequence

After setup is complete (or on every subsequent run of `ghost`), the tool performs the following **background orchestration**:

1.  **Start Ghost Engine:** The binary converter engine starts in a background goroutine, listening on a dynamic local port (e.g., `127.0.0.1:3200`).
2.  **Health Check:** The CLI waits ~100ms for the engine to signal it is "Ready."
3.  **Inject Environment:** The tool sets the `ANTHROPIC_BASE_URL` to point to the local engine.
4.  **Launch Claude Code:** The CLI uses `os.Exec` to start `claude`.

```bash
# This is what happens under the hood:
export ANTHROPIC_BASE_URL=http://localhost:3200
export ANTHROPIC_AUTH_TOKEN=your_provider_key
claude
```

---

## 4. Best Practices for the Onboarding UI

- **"Smart" Defaults:** If the user has a `DEEPSEEK_API_KEY` already in their shell environment, the tool should detect it and offer it as the default choice.
- **Immediate Validation:** Before starting Claude Code, the engine should send a "dummy" request to the provider to verify the API key is actually working.
- **Graceful Exit:** If the user exits Claude Code (`/exit`), GhostCLI should automatically shut down the background engine and clean up the environment.

---

## 5. Proposed Go Libraries for UI
- **[Bubble Tea / Lip Gloss](https://github.com/charmbracelet/bubbletea):** For a high-fidelity, modern terminal UI (TUIs).
- **[Survey](https://github.com/AlecAivazis/survey):** For simple, interactive prompts and selects.
