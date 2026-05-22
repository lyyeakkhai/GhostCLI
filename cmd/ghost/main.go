// GhostCLI — a high-performance proxy connecting Claude Code to alternative LLM providers.
//
// Usage:
//
//	ghost                          # start proxy with default/active provider
//	ghost --provider deepseek      # start with specific provider
//	ghost --setup                  # run interactive setup wizard
//	ghost --version                # print version information
//	ghost --clear-keys             # delete all stored API keys
//
// Environment variables:
//
//	GHOST_PROVIDER   active provider name
//	GHOST_PORT       HTTP listen port (default: 3200)
//	GHOST_HOST       HTTP listen host (default: 127.0.0.1)
//	GHOST_VERBOSE    enable debug logging (true/false)
//	<PROVIDER>_API_KEY  provider-specific API key
//
// After starting, configure Claude Code:
//
//	export ANTHROPIC_BASE_URL=http://localhost:3200
//	export DEEPSEEK_API_KEY=sk-...
//	claude
package main

import (
	"flag"
	"fmt"
	"os"

	"ghostcli/internal/app"
	"ghostcli/internal/config"
	"ghostcli/internal/telemetry"
)

// Build-time variables (injected via -ldflags).
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Define flags
	var (
		port       = flag.Int("port", 0, "HTTP listen port (default: 3200 or from config)")
		provider   = flag.String("provider", "", "Active provider name (deepseek, kimi, openai, kiro)")
		apiKey     = flag.String("api-key", "", "API key for the active provider")
		verbose    = flag.Bool("verbose", false, "Enable verbose (debug) logging")
		configPath = flag.String("config", "", "Path to configuration file")
		timeout    = flag.Int("timeout", 0, "Request timeout in seconds")
		corsOrigin = flag.String("cors-origin", "", "CORS allowed origin")
		setupMode  = flag.Bool("setup", false, "Run interactive setup wizard")
		versionMode= flag.Bool("version", false, "Print version information")
		clearKeys  = flag.Bool("clear-keys", false, "Delete all stored API keys")
		force      = flag.Bool("force", false, "Skip confirmation prompts")
	)
	flag.Parse()

	// Handle version flag
	if *versionMode {
		bi := newBuildInfo(version, commit, buildDate)
		fmt.Println(bi.String())
		return nil
	}

	// Initialise logger early so setup/clear operations can log.
	logLevel := telemetry.LevelInfo
	if *verbose {
		logLevel = telemetry.LevelDebug
	}
	logger := telemetry.NewLogger(telemetry.LoggerOptions{
		Level:      logLevel,
		Output:     os.Stderr,
		JSONFormat: false,
	})

	// Handle setup wizard
	if *setupMode {
		return runSetup(logger)
	}

	// Handle clear-keys command
	if *clearKeys {
		return runClearKeys(logger, *force)
	}

	// Build CLI flags map for config merging
	cliFlags := make(map[string]interface{})
	if *port != 0 {
		cliFlags["port"] = *port
	}
	if *provider != "" {
		cliFlags["provider"] = *provider
	}
	if *apiKey != "" {
		cliFlags["api-key"] = *apiKey
	}
	if *timeout != 0 {
		cliFlags["timeout"] = *timeout
	}
	if *corsOrigin != "" {
		cliFlags["cors-origin"] = *corsOrigin
	}
	cliFlags["verbose"] = *verbose

	// Load configuration
	cfg, err := config.Load(*configPath, cliFlags)
	if err != nil {
		// If no config exists and no provider was given, suggest setup
		if *configPath == "" && *provider == "" {
			logger.Warn("no configuration found; run 'ghost --setup' to configure")
		}
		return fmt.Errorf("load configuration: %w", err)
	}

	// Re-initialise logger with config-derived level
	if cfg.Verbose {
		logger = telemetry.NewLogger(telemetry.LoggerOptions{
			Level:      telemetry.LevelDebug,
			Output:     os.Stderr,
			JSONFormat: false,
		})
	}

	// Create application
	application, err := app.NewApp(cfg, logger)
	if err != nil {
		return fmt.Errorf("initialise app: %w", err)
	}

	// Override version from build info
	application.Version = version

	// Run application (blocks until shutdown)
	return application.Run()
}


