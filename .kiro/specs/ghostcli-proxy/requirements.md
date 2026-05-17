# Requirements Document

## Introduction

GhostCLI is a high-performance, Go-based proxy server that acts as a translation layer between Claude Code and various LLM providers (DeepSeek, Kimi, OpenAI, Kiro, etc.). The system intercepts Anthropic Messages API requests, translates them through a unified protocol architecture, routes them to provider-specific adapters, and streams responses back in Anthropic-compatible format. The goal is to enable Claude Code users to access alternative LLM providers with zero configuration changes, achieving 10x cost savings while maintaining full feature compatibility.

## Glossary

- **GhostCLI**: The complete proxy system including HTTP server, translation engine, and CLI tool
- **Claude_Code**: The Anthropic terminal-based AI coding assistant that sends requests to the proxy
- **HTTP_Server**: The Go HTTP server component listening on localhost that receives Anthropic API requests
- **Translation_Engine**: The core component that converts between Anthropic format and the Unified Protocol
- **Unified_Protocol**: The internal data structures (UnifiedChatRequest, UnifiedStreamEvent) used for provider-agnostic message representation
- **Provider_Adapter**: A module implementing the Provider interface for a specific LLM service
- **Provider_Router**: The component that selects the appropriate Provider_Adapter based on configuration
- **AnthropicIn_Parser**: The component that converts incoming Anthropic JSON to UnifiedChatRequest
- **AnthropicOut_Formatter**: The component that converts UnifiedStreamEvent to Anthropic SSE format
- **SSE**: Server-Sent Events, the streaming protocol used by Anthropic Messages API
- **CLI**: The command-line interface tool that users interact with to start and configure the proxy
- **Keyring**: OS-native secure storage system (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- **TTFT**: Time-to-first-token, the latency metric from request to first response chunk
- **Provider_Registry**: The map of provider identifiers to their adapter implementations

## Requirements

### Requirement 1: HTTP Server Initialization

**User Story:** As a developer, I want the proxy to start an HTTP server on a configurable port, so that Claude Code can connect to it via ANTHROPIC_BASE_URL.

#### Acceptance Criteria

1. WHEN the CLI is invoked with a valid port flag, THE HTTP_Server SHALL bind to 127.0.0.1 on the specified port
2. WHERE no port flag is provided, THE HTTP_Server SHALL bind to port 3200 by default
3. IF the specified port is already in use, THEN THE CLI SHALL return an error message indicating the port conflict and exit with status code 1
4. WHEN the HTTP_Server successfully binds, THE CLI SHALL log the listening address to stdout
5. THE HTTP_Server SHALL accept HTTP/1.1 and HTTP/2 connections
6. WHEN the HTTP_Server receives a request to /v1/messages, THE HTTP_Server SHALL route it to the Translation_Engine
7. WHEN the HTTP_Server receives a request to any path other than /v1/messages, THE HTTP_Server SHALL return HTTP 404

### Requirement 2: Anthropic Request Parsing

**User Story:** As a system, I want to parse incoming Anthropic Messages API requests into a unified format, so that provider adapters can work with a consistent data structure.

#### Acceptance Criteria

1. WHEN the HTTP_Server receives a POST request to /v1/messages, THE AnthropicIn_Parser SHALL decode the JSON body into a UnifiedChatRequest struct
2. THE AnthropicIn_Parser SHALL use streaming JSON decoding without buffering the entire request body
3. IF the request body contains invalid JSON, THEN THE AnthropicIn_Parser SHALL return HTTP 400 with error details
4. THE AnthropicIn_Parser SHALL extract the model field from the Anthropic request
5. THE AnthropicIn_Parser SHALL extract the messages array from the Anthropic request
6. THE AnthropicIn_Parser SHALL extract the system field from the Anthropic request when present
7. THE AnthropicIn_Parser SHALL extract the temperature field from the Anthropic request when present
8. THE AnthropicIn_Parser SHALL extract the max_tokens field from the Anthropic request
9. THE AnthropicIn_Parser SHALL extract the tools array from the Anthropic request when present
10. THE AnthropicIn_Parser SHALL extract the stream field from the Anthropic request

### Requirement 3: Provider Routing

**User Story:** As a user, I want the proxy to route my requests to the correct LLM provider, so that I can use different providers without changing Claude Code configuration.

#### Acceptance Criteria

1. WHEN the CLI starts, THE Provider_Router SHALL register all available Provider_Adapters in the Provider_Registry
2. WHEN the Translation_Engine receives a UnifiedChatRequest, THE Provider_Router SHALL select the Provider_Adapter based on the active provider configuration
3. IF the configured provider is not in the Provider_Registry, THEN THE Provider_Router SHALL return an error listing all supported providers
4. THE Provider_Router SHALL support dynamic provider selection without restarting the HTTP_Server
5. WHEN a Provider_Adapter is selected, THE Provider_Router SHALL invoke its StreamChat method with the UnifiedChatRequest

### Requirement 4: Provider Adapter Interface

**User Story:** As a developer, I want a standardized interface for provider adapters, so that adding new providers requires minimal code changes.

#### Acceptance Criteria

1. THE Provider_Adapter SHALL implement a StreamChat method that accepts a context and UnifiedChatRequest
2. THE Provider_Adapter StreamChat method SHALL return a channel of UnifiedStreamEvent and an error
3. WHEN the Provider_Adapter receives a UnifiedChatRequest, THE Provider_Adapter SHALL translate it to the provider-specific request format
4. WHEN the Provider_Adapter sends a request to the provider API, THE Provider_Adapter SHALL stream responses as UnifiedStreamEvent objects
5. THE Provider_Adapter SHALL map Anthropic model names to provider-specific model identifiers
6. IF the provider API returns an error, THEN THE Provider_Adapter SHALL emit a UnifiedStreamEvent with error type
7. WHEN the context is cancelled, THE Provider_Adapter SHALL immediately terminate the provider API connection

### Requirement 5: Streaming Response Translation

**User Story:** As Claude Code, I want to receive responses in Anthropic SSE format, so that I can display streaming output to the user.

#### Acceptance Criteria

1. WHEN the Translation_Engine receives UnifiedStreamEvent objects, THE AnthropicOut_Formatter SHALL convert them to Anthropic SSE format
2. THE AnthropicOut_Formatter SHALL write SSE events with proper event type and data fields
3. THE AnthropicOut_Formatter SHALL terminate each SSE event with double newline characters
4. THE AnthropicOut_Formatter SHALL flush the HTTP response writer after each event
5. THE AnthropicOut_Formatter SHALL track input_tokens and output_tokens across all events
6. WHEN a UnifiedStreamEvent lacks token usage data, THE AnthropicOut_Formatter SHALL inject the last known token counts
7. WHEN the UnifiedStreamEvent channel closes, THE AnthropicOut_Formatter SHALL send a message_stop event

### Requirement 6: DeepSeek Provider Adapter

**User Story:** As a user, I want to use DeepSeek models through Claude Code, so that I can reduce API costs.

#### Acceptance Criteria

1. THE DeepSeek_Adapter SHALL implement the Provider_Adapter interface
2. WHEN the DeepSeek_Adapter receives a UnifiedChatRequest, THE DeepSeek_Adapter SHALL convert it to OpenAI Chat Completions format
3. THE DeepSeek_Adapter SHALL map claude-3-5-sonnet to deepseek-chat
4. THE DeepSeek_Adapter SHALL send requests to api.deepseek.com/v1/chat/completions
5. WHEN the DeepSeek API returns SSE chunks, THE DeepSeek_Adapter SHALL parse them into UnifiedStreamEvent objects
6. THE DeepSeek_Adapter SHALL extract token usage from the final SSE chunk
7. THE DeepSeek_Adapter SHALL include the API key in the Authorization header as Bearer token

### Requirement 7: Kimi Provider Adapter

**User Story:** As a user, I want to use Kimi models through Claude Code, so that I can access alternative Chinese LLM providers.

#### Acceptance Criteria

1. THE Kimi_Adapter SHALL implement the Provider_Adapter interface
2. WHEN the Kimi_Adapter receives a UnifiedChatRequest, THE Kimi_Adapter SHALL convert it to OpenAI Chat Completions format
3. THE Kimi_Adapter SHALL map claude-3-5-sonnet to moonshot-v1-128k
4. THE Kimi_Adapter SHALL send requests to api.moonshot.cn/v1/chat/completions
5. WHEN the Kimi API returns SSE chunks, THE Kimi_Adapter SHALL parse them into UnifiedStreamEvent objects
6. THE Kimi_Adapter SHALL extract token usage from the final SSE chunk
7. THE Kimi_Adapter SHALL include the API key in the Authorization header as Bearer token

### Requirement 8: OpenAI Provider Adapter

**User Story:** As a user, I want to use OpenAI models through Claude Code, so that I can compare responses across providers.

#### Acceptance Criteria

1. THE OpenAI_Adapter SHALL implement the Provider_Adapter interface
2. WHEN the OpenAI_Adapter receives a UnifiedChatRequest, THE OpenAI_Adapter SHALL convert it to OpenAI Chat Completions format
3. THE OpenAI_Adapter SHALL map claude-3-5-sonnet to gpt-4o
4. THE OpenAI_Adapter SHALL send requests to api.openai.com/v1/chat/completions
5. WHEN the OpenAI API returns SSE chunks, THE OpenAI_Adapter SHALL parse them into UnifiedStreamEvent objects
6. THE OpenAI_Adapter SHALL extract token usage from the final SSE chunk
7. THE OpenAI_Adapter SHALL include the API key in the Authorization header as Bearer token

### Requirement 9: CLI Configuration Management

**User Story:** As a user, I want to configure the proxy through command-line flags, so that I can control its behavior without editing files.

#### Acceptance Criteria

1. THE CLI SHALL accept a --port flag to specify the listening port
2. THE CLI SHALL accept a --provider flag to specify the active provider
3. THE CLI SHALL accept a --api-key flag to specify the provider API key
4. THE CLI SHALL accept a --verbose flag to enable debug logging
5. THE CLI SHALL accept a --config flag to specify a configuration file path
6. WHEN the CLI starts, THE CLI SHALL validate that all required configuration is present
7. IF required configuration is missing, THEN THE CLI SHALL print usage information and exit with status code 1
8. THE CLI SHALL support reading configuration from environment variables
9. WHEN both flag and environment variable are provided, THE CLI SHALL prioritize the flag value

### Requirement 10: Secure API Key Storage

**User Story:** As a user, I want my API keys stored securely, so that they are not exposed in plain text files.

#### Acceptance Criteria

1. WHEN the CLI stores an API key, THE CLI SHALL attempt to save it to the OS Keyring
2. THE CLI SHALL use the service name ghostcli for all Keyring entries
3. THE CLI SHALL use the provider name as the account identifier in the Keyring
4. IF the OS Keyring is unavailable, THEN THE CLI SHALL fall back to encrypted local file storage
5. WHEN using encrypted local file storage, THE CLI SHALL derive the encryption key from the machine hardware UUID
6. THE CLI SHALL store encrypted keys in the user home directory under .config/ghost/secrets.db
7. WHEN the CLI retrieves an API key, THE CLI SHALL first attempt to read from the OS Keyring
8. IF the Keyring read fails, THEN THE CLI SHALL attempt to read from the encrypted local file
9. THE CLI SHALL support a --no-store flag that keeps API keys in memory only for the session

### Requirement 11: Interactive First-Run Setup

**User Story:** As a new user, I want an interactive setup wizard, so that I can configure the proxy without reading documentation.

#### Acceptance Criteria

1. WHEN the CLI is invoked without existing configuration, THE CLI SHALL enter interactive setup mode
2. THE CLI SHALL display a welcome message explaining the purpose of GhostCLI
3. THE CLI SHALL prompt the user to select a provider from a list
4. THE CLI SHALL display each provider with its pattern family label
5. WHEN the user selects a provider, THE CLI SHALL prompt for the API key with masked input
6. THE CLI SHALL validate the API key format using provider-specific regex patterns
7. WHEN the user completes setup, THE CLI SHALL save the configuration using the secure storage mechanism
8. THE CLI SHALL display a success message with instructions for using Claude Code
9. THE CLI SHALL support a --skip-setup flag to bypass interactive mode

### Requirement 12: Provider API Key Validation

**User Story:** As a user, I want the proxy to validate my API key before starting, so that I receive immediate feedback on configuration errors.

#### Acceptance Criteria

1. WHEN the CLI completes configuration, THE CLI SHALL send a test request to the provider API
2. THE test request SHALL use minimal tokens to reduce cost
3. IF the provider API returns HTTP 401 or 403, THEN THE CLI SHALL display an authentication error message
4. IF the provider API returns HTTP 200, THEN THE CLI SHALL display a success message
5. THE CLI SHALL support a --skip-validation flag to bypass API key testing
6. WHEN validation fails, THE CLI SHALL exit with status code 1 unless --skip-validation is set

### Requirement 13: Graceful Shutdown

**User Story:** As a user, I want the proxy to shut down cleanly when I stop it, so that no requests are lost or corrupted.

#### Acceptance Criteria

1. WHEN the CLI receives SIGINT or SIGTERM, THE HTTP_Server SHALL stop accepting new connections
2. THE HTTP_Server SHALL wait for all active requests to complete before shutting down
3. THE HTTP_Server SHALL enforce a shutdown timeout of 30 seconds
4. IF active requests exceed the shutdown timeout, THEN THE HTTP_Server SHALL forcefully close all connections
5. WHEN shutdown completes, THE CLI SHALL log a shutdown message and exit with status code 0
6. THE CLI SHALL close all provider API connections during shutdown

### Requirement 14: Request Context Propagation

**User Story:** As a system, I want to propagate request cancellation through all components, so that resources are freed immediately when Claude Code cancels a request.

#### Acceptance Criteria

1. WHEN the HTTP_Server receives a request, THE HTTP_Server SHALL create a context from the HTTP request
2. THE HTTP_Server SHALL pass the context to the Translation_Engine
3. THE Translation_Engine SHALL pass the context to the Provider_Adapter
4. WHEN the HTTP connection is closed by the client, THE context SHALL be cancelled
5. WHEN the context is cancelled, THE Provider_Adapter SHALL immediately close the provider API connection
6. THE Provider_Adapter SHALL stop emitting UnifiedStreamEvent objects when the context is cancelled

### Requirement 15: Token Usage Normalization

**User Story:** As Claude Code, I want accurate token usage information in every response, so that I can display usage statistics to the user.

#### Acceptance Criteria

1. THE AnthropicOut_Formatter SHALL maintain a running count of input_tokens and output_tokens
2. WHEN a UnifiedStreamEvent contains token usage data, THE AnthropicOut_Formatter SHALL update the running counts
3. WHEN a UnifiedStreamEvent lacks token usage data, THE AnthropicOut_Formatter SHALL include the last known counts in the SSE event
4. THE AnthropicOut_Formatter SHALL emit a message_delta event with usage data for every content chunk
5. THE AnthropicOut_Formatter SHALL emit a message_stop event with final usage data when the stream completes
6. IF no token usage data is available from the provider, THEN THE AnthropicOut_Formatter SHALL estimate tokens based on content length

### Requirement 16: Error Handling and Propagation

**User Story:** As Claude Code, I want to receive clear error messages when the proxy encounters problems, so that I can inform the user of the issue.

#### Acceptance Criteria

1. WHEN a Provider_Adapter encounters a provider API error, THE Provider_Adapter SHALL emit a UnifiedStreamEvent with error type
2. THE UnifiedStreamEvent error SHALL include the HTTP status code and error message from the provider
3. WHEN the AnthropicOut_Formatter receives an error event, THE AnthropicOut_Formatter SHALL convert it to an Anthropic error SSE event
4. THE AnthropicOut_Formatter SHALL include the error type and message in the SSE data field
5. IF the Translation_Engine encounters a parsing error, THEN THE HTTP_Server SHALL return HTTP 400 with error details
6. IF the Provider_Router cannot find a provider, THEN THE HTTP_Server SHALL return HTTP 500 with error details
7. THE CLI SHALL log all errors to stderr with timestamps and severity levels

### Requirement 17: Structured Logging

**User Story:** As a developer, I want structured logs with configurable verbosity, so that I can debug issues and monitor performance.

#### Acceptance Criteria

1. THE CLI SHALL use Go slog package for all logging
2. THE CLI SHALL support log levels: debug, info, warn, error
3. WHERE the --verbose flag is set, THE CLI SHALL log at debug level
4. WHERE the --verbose flag is not set, THE CLI SHALL log at info level
5. THE CLI SHALL log HTTP request method, path, and status code for every request
6. THE CLI SHALL log provider selection and model mapping for every request
7. THE CLI SHALL log TTFT for every streaming response
8. THE CLI SHALL log total request duration when the stream completes
9. THE CLI SHALL include timestamps in RFC3339 format for all log entries

### Requirement 18: Cross-Platform Binary Distribution

**User Story:** As a user, I want to download a single binary for my operating system, so that I can run GhostCLI without installing dependencies.

#### Acceptance Criteria

1. THE build system SHALL produce static binaries for Windows amd64
2. THE build system SHALL produce static binaries for macOS amd64
3. THE build system SHALL produce static binaries for macOS arm64
4. THE build system SHALL produce static binaries for Linux amd64
5. THE Windows binary SHALL have .exe extension
6. THE macOS and Linux binaries SHALL have executable permissions set
7. THE build system SHALL use GitHub Actions to automate binary builds
8. WHEN a version tag is pushed, THE build system SHALL create a GitHub Release with all platform binaries

### Requirement 19: Model Name Mapping

**User Story:** As a system, I want to map Anthropic model names to provider-specific names, so that requests use the correct model for each provider.

#### Acceptance Criteria

1. THE Provider_Adapter SHALL maintain a map of Anthropic model names to provider model names
2. WHEN the Provider_Adapter receives a UnifiedChatRequest, THE Provider_Adapter SHALL look up the model name in its map
3. IF the model name is found in the map, THEN THE Provider_Adapter SHALL use the mapped provider model name
4. IF the model name is not found in the map, THEN THE Provider_Adapter SHALL use the original model name
5. THE Provider_Adapter SHALL log the model mapping at debug level
6. THE model mapping SHALL be configurable through a JSON file in the user config directory

### Requirement 20: Tool Call Support

**User Story:** As Claude Code, I want to use tool calling features with alternative providers, so that I can execute functions during conversations.

#### Acceptance Criteria

1. WHEN the AnthropicIn_Parser receives a request with tools, THE AnthropicIn_Parser SHALL extract tool definitions into the UnifiedChatRequest
2. THE UnifiedChatRequest SHALL represent tools in a provider-agnostic format
3. WHEN a Provider_Adapter supports tool calling, THE Provider_Adapter SHALL convert UnifiedChatRequest tools to provider-specific format
4. WHEN a provider returns a tool call in the response, THE Provider_Adapter SHALL convert it to a UnifiedStreamEvent with tool_use type
5. THE AnthropicOut_Formatter SHALL convert tool_use events to Anthropic content_block_start events with tool_use type
6. IF a Provider_Adapter does not support tool calling, THEN THE Provider_Adapter SHALL return an error when tools are present in the request

### Requirement 21: Configuration File Support

**User Story:** As a user, I want to define multiple provider configurations in a file, so that I can switch between providers without re-entering credentials.

#### Acceptance Criteria

1. THE CLI SHALL support reading configuration from a YAML file
2. THE CLI SHALL support reading configuration from a JSON file
3. THE configuration file SHALL define multiple provider entries with name and api_key fields
4. THE configuration file SHALL define an active_provider field indicating the default provider
5. WHEN the CLI starts, THE CLI SHALL load the configuration file from the path specified by --config flag
6. WHERE no --config flag is provided, THE CLI SHALL look for config.yaml in the user config directory
7. THE CLI SHALL merge configuration file values with command-line flags, prioritizing flags
8. IF the configuration file contains invalid YAML or JSON, THEN THE CLI SHALL print a parse error and exit with status code 1

### Requirement 22: Health Check Endpoint

**User Story:** As a monitoring system, I want a health check endpoint, so that I can verify the proxy is running and responsive.

#### Acceptance Criteria

1. THE HTTP_Server SHALL respond to GET requests at /health
2. WHEN the HTTP_Server is ready to accept requests, THE HTTP_Server SHALL return HTTP 200 for /health requests
3. THE /health response body SHALL contain JSON with status field set to ok
4. THE /health response SHALL include the active provider name
5. THE /health response SHALL include the server version
6. THE /health endpoint SHALL not require authentication
7. THE /health endpoint SHALL respond within 100 milliseconds

### Requirement 23: NPM Wrapper Package

**User Story:** As a user, I want to install GhostCLI via npm, so that I can use familiar JavaScript tooling for installation.

#### Acceptance Criteria

1. THE npm package SHALL detect the user operating system and architecture
2. WHEN the npm package is installed, THE npm package SHALL download the appropriate GhostCLI binary from GitHub Releases
3. THE npm package SHALL place the binary in node_modules/.bin with the name ghost
4. THE npm package SHALL make the ghost command available globally when installed with -g flag
5. IF the binary download fails, THEN THE npm package installation SHALL fail with an error message
6. THE npm package SHALL verify the binary checksum after download
7. THE npm package SHALL support npx ghost for on-demand execution without installation

### Requirement 24: Streaming Performance Optimization

**User Story:** As a user, I want minimal latency between provider responses and Claude Code display, so that the experience feels responsive.

#### Acceptance Criteria

1. THE AnthropicIn_Parser SHALL use streaming JSON decoding to avoid buffering the entire request
2. THE Provider_Adapter SHALL emit UnifiedStreamEvent objects immediately upon receiving provider SSE chunks
3. THE AnthropicOut_Formatter SHALL flush the HTTP response writer after writing each SSE event
4. THE HTTP_Server SHALL use HTTP/2 when the client supports it
5. THE Translation_Engine SHALL add less than 5 milliseconds of latency to TTFT
6. THE Provider_Adapter SHALL use connection pooling for provider API requests
7. THE HTTP_Server SHALL reuse TCP connections for multiple requests from Claude Code

### Requirement 25: Thinking Block Handling

**User Story:** As Claude Code, I want to receive thinking blocks from providers that support extended thinking, so that I can display the model reasoning process.

#### Acceptance Criteria

1. WHEN a Provider_Adapter receives thinking content from the provider, THE Provider_Adapter SHALL emit a UnifiedStreamEvent with thinking type
2. THE AnthropicOut_Formatter SHALL convert thinking events to Anthropic content_block_start events with type thinking
3. THE AnthropicOut_Formatter SHALL stream thinking content as content_block_delta events
4. WHEN the thinking block completes, THE AnthropicOut_Formatter SHALL emit a content_block_stop event
5. IF a provider does not support thinking blocks, THEN THE Provider_Adapter SHALL omit thinking events from the stream

### Requirement 26: Version Information

**User Story:** As a user, I want to check the installed version of GhostCLI, so that I can verify I have the latest release.

#### Acceptance Criteria

1. THE CLI SHALL accept a --version flag
2. WHEN the --version flag is provided, THE CLI SHALL print the version number and exit with status code 0
3. THE version number SHALL follow semantic versioning format
4. THE CLI SHALL include the Git commit hash in the version output
5. THE CLI SHALL include the build date in the version output
6. THE version information SHALL be embedded at compile time using Go build flags

### Requirement 27: Provider Adapter Registry Initialization

**User Story:** As a developer, I want provider adapters to self-register at startup, so that adding new providers requires minimal boilerplate.

#### Acceptance Criteria

1. WHEN the CLI starts, THE Provider_Router SHALL call an initialization function for each provider package
2. THE provider package initialization function SHALL register the Provider_Adapter in the Provider_Registry
3. THE Provider_Registry SHALL store adapters in a thread-safe map
4. THE Provider_Router SHALL support querying the Provider_Registry for available provider names
5. THE CLI SHALL log all registered providers at startup when --verbose is set

### Requirement 28: Request Timeout Configuration

**User Story:** As a user, I want to configure request timeouts, so that I can prevent hanging requests from blocking the proxy.

#### Acceptance Criteria

1. THE CLI SHALL accept a --timeout flag to specify the maximum request duration in seconds
2. WHERE no --timeout flag is provided, THE HTTP_Server SHALL use a default timeout of 300 seconds
3. WHEN a request exceeds the timeout, THE HTTP_Server SHALL cancel the context
4. WHEN the context is cancelled due to timeout, THE Provider_Adapter SHALL close the provider connection
5. THE HTTP_Server SHALL return HTTP 504 when a request times out
6. THE timeout SHALL apply to the entire request lifecycle including streaming

### Requirement 29: CORS Support

**User Story:** As a web application developer, I want CORS headers on proxy responses, so that I can use GhostCLI from browser-based applications.

#### Acceptance Criteria

1. THE HTTP_Server SHALL respond to OPTIONS requests with CORS preflight headers
2. THE HTTP_Server SHALL include Access-Control-Allow-Origin header in all responses
3. THE HTTP_Server SHALL include Access-Control-Allow-Methods header with POST, GET, OPTIONS
4. THE HTTP_Server SHALL include Access-Control-Allow-Headers header with Content-Type, Authorization
5. THE CLI SHALL accept a --cors-origin flag to specify allowed origins
6. WHERE no --cors-origin flag is provided, THE HTTP_Server SHALL use * as the default origin
7. THE HTTP_Server SHALL include Access-Control-Max-Age header with value 86400

### Requirement 30: Configuration Clear Command

**User Story:** As a user, I want to clear stored credentials, so that I can remove sensitive data when switching machines or troubleshooting.

#### Acceptance Criteria

1. THE CLI SHALL accept a --clear-keys command
2. WHEN the --clear-keys command is invoked, THE CLI SHALL iterate through all providers in the Keyring
3. THE CLI SHALL delete each provider API key from the Keyring
4. THE CLI SHALL delete the encrypted local configuration file if it exists
5. WHEN key clearing completes, THE CLI SHALL print a success message
6. IF key clearing fails, THEN THE CLI SHALL print an error message and exit with status code 1
7. THE CLI SHALL prompt for confirmation before clearing keys unless --force flag is provided
