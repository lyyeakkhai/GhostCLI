# Implementation Plan: GhostCLI Proxy

## Overview

This implementation plan breaks down the GhostCLI proxy server into discrete coding tasks. The system is a Go-based HTTP proxy that translates Anthropic Messages API requests to multiple LLM providers (DeepSeek, Kimi, OpenAI, Kiro) through a unified protocol architecture. The implementation follows SOLID principles with dependency injection, interface-based design, and pattern-based provider adapters.

## Tasks

- [x] 1. Set up project structure and core protocol types
  - Create Go module with `go mod init`
  - Set up folder structure following the modular design (cmd/, internal/, pkg/, docs/)
  - Define UnifiedChatRequest, UnifiedStreamEvent, and related types in `internal/engine/protocol/types.go`
  - Define event type constants and shared protocol utilities in `internal/engine/protocol/constants.go`
  - _Requirements: 1, 2, 4, 5_

- [x] 2. Implement configuration management layer
  - [x] 2.1 Create configuration structures and file loading
    - Implement Config and ProviderConfig structs in `internal/config/config.go`
    - Add YAML and JSON configuration file parsing
    - Implement configuration merging (file → env vars → CLI flags)
    - Add validation for required configuration fields
    - _Requirements: 9, 21_
  
  - [x] 2.2 Implement secure API key storage
    - Create SecureStorage struct with OS keyring integration in `internal/config/storage.go`
    - Implement encrypted file fallback using machine UUID-derived key
    - Add SaveAPIKey, GetAPIKey, and DeleteAPIKey methods
    - Create EncryptedFile implementation in `internal/config/encrypted_file.go`
    - _Requirements: 10, 30_
  
  - [x] 2.3 Add API key validation
    - Implement provider-specific API key format validation in `internal/config/validation.go`
    - Add test request functionality to validate keys against provider APIs
    - Implement validation error handling and reporting
    - _Requirements: 12_

- [x] 3. Implement translation engine core
  - [x] 3.1 Create AnthropicIn parser
    - Implement AnthropicInParser struct in `internal/engine/translator/anthropic_in.go`
    - Add streaming JSON decoder for incoming Anthropic requests
    - Implement conversion from Anthropic format to UnifiedChatRequest
    - Add system prompt normalization (string and content block array formats)
    - Add message and tool conversion logic
    - _Requirements: 2_
  
  - [x] 3.2 Create AnthropicOut formatter
    - Implement AnthropicOutFormatter struct in `internal/engine/translator/anthropic_out.go`
    - Add SSE event writing with immediate flushing
    - Implement conversion from UnifiedStreamEvent to Anthropic SSE format
    - Add token usage tracking and injection logic
    - Implement message_start, content_block_delta, and message_stop event generation
    - _Requirements: 5, 15_
  
  - [x] 3.3 Implement streaming pipeline orchestration
    - Create stream orchestration logic in `internal/engine/pipeline/stream.go`
    - Add context propagation and cancellation handling
    - Implement token usage normalization in `internal/engine/pipeline/usage.go`
    - _Requirements: 14, 15, 24_

- [x] 4. Checkpoint - Ensure translation engine tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 5. Implement provider interface and registry
  - [x] 5.1 Define provider interface
    - Create Provider interface in `internal/providers/interface.go`
    - Define StreamChat, Name, SupportsTools, SupportsThinking, and MapModel methods
    - _Requirements: 4_
  
  - [ ] 5.2 Create provider registry
    - Implement thread-safe Registry struct in `internal/providers/registry.go`
    - Add Register, Get, and List methods with mutex protection
    - _Requirements: 3, 27_
  
  - [ ] 5.3 Implement provider factory
    - Create Factory struct with dependency injection in `internal/providers/factory.go`
    - Add CreateProvider method with pattern-based routing
    - Implement factory methods for each provider pattern (OpenAI, Anthropic, AWS)
    - _Requirements: 3, 27_

- [ ] 6. Implement base provider adapters (patterns)
  - [ ] 6.1 Create OpenAI-compatible base adapter
    - Implement OpenAIAdapter in `internal/providers/base/openai_base.go`
    - Add StreamChat method with OpenAI Chat Completions format conversion
    - Implement SSE streaming response parser
    - Add model mapping and configuration support
    - _Requirements: 4, 6, 7, 8, 19_
  
  - [ ] 6.2 Create Anthropic-native base adapter
    - Implement AnthropicAdapter in `internal/providers/base/anthropic_base.go`
    - Add passthrough logic with minimal translation
    - Implement Anthropic-specific header handling
    - _Requirements: 4_
  
  - [ ] 6.3 Create AWS EventStream base adapter
    - Implement AWSAdapter in `internal/providers/base/aws_base.go`
    - Add binary EventStream protocol decoder
    - Implement AWS-specific request format conversion
    - _Requirements: 4_

- [ ] 7. Implement specific provider adapters
  - [ ] 7.1 Create DeepSeek adapter
    - Implement DeepSeek adapter in `internal/providers/deepseek/adapter.go`
    - Configure OpenAI base with DeepSeek-specific settings (api.deepseek.com, model mapping)
    - Add DeepSeek configuration in `internal/providers/deepseek/config.go`
    - _Requirements: 6_
  
  - [ ] 7.2 Create Kimi adapter
    - Implement Kimi adapter in `internal/providers/kimi/adapter.go`
    - Configure OpenAI base with Kimi-specific settings (api.moonshot.cn, model mapping)
    - Add Kimi configuration
    - _Requirements: 7_
  
  - [ ] 7.3 Create OpenAI adapter
    - Implement OpenAI adapter in `internal/providers/openai/adapter.go`
    - Configure OpenAI base with OpenAI-specific settings (api.openai.com, model mapping)
    - Add OpenAI configuration
    - _Requirements: 8_
  
  - [ ] 7.4 Create Kiro adapter
    - Implement Kiro adapter in `internal/providers/kiro/adapter.go`
    - Configure AWS base with Kiro-specific settings
    - Add Kiro configuration
    - _Requirements: 4_

- [ ] 8. Implement HTTP transport layer
  - [ ] 8.1 Create HTTP server setup
    - Implement server initialization in `internal/api/server.go`
    - Add HTTP/1.1 and HTTP/2 support
    - Implement graceful shutdown with timeout
    - Add port binding and error handling
    - _Requirements: 1, 13_
  
  - [ ] 8.2 Create request handlers
    - Implement /v1/messages POST handler in `internal/api/handlers.go`
    - Add 404 handler for unsupported paths
    - Implement request routing logic
    - _Requirements: 1, 2_
  
  - [ ] 8.3 Implement middleware
    - Create CORS middleware in `internal/api/middleware.go`
    - Add structured logging middleware
    - Implement context propagation middleware
    - Add request timeout middleware
    - _Requirements: 14, 17, 28, 29_
  
  - [ ] 8.4 Create health check endpoint
    - Implement /health GET handler in `internal/api/health.go`
    - Add JSON response with status, provider, and version
    - Ensure sub-100ms response time
    - _Requirements: 22_

- [ ] 9. Checkpoint - Ensure HTTP layer tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 10. Implement telemetry layer
  - [ ] 10.1 Create structured logging
    - Implement logger setup with slog in `internal/telemetry/logger.go`
    - Add log level configuration (debug, info, warn, error)
    - Implement request/response logging with timestamps
    - Add TTFT and duration metrics logging
    - _Requirements: 17_
  
  - [ ] 10.2 Add performance metrics
    - Implement metrics collection in `internal/telemetry/metrics.go`
    - Add TTFT tracking
    - Add request duration tracking
    - _Requirements: 17, 24_

- [ ] 11. Implement application orchestration layer
  - [ ] 11.1 Create application struct and DI container
    - Implement App struct in `internal/app/app.go`
    - Add dependency injection for all components
    - Create component initialization logic
    - _Requirements: 1, 3, 27_
  
  - [ ] 11.2 Implement lifecycle management
    - Create startup logic in `internal/app/lifecycle.go`
    - Implement graceful shutdown with signal handling
    - Add component cleanup on shutdown
    - _Requirements: 13_

- [ ] 12. Implement CLI layer
  - [ ] 12.1 Create main CLI entry point
    - Implement main.go in `cmd/ghost/main.go`
    - Add command-line flag parsing (port, provider, api-key, verbose, config, timeout, cors-origin)
    - Add environment variable support
    - Implement bootstrap logic with App initialization
    - _Requirements: 9, 28, 29_
  
  - [ ] 12.2 Create interactive setup wizard
    - Implement setup wizard in `cmd/ghost/setup.go`
    - Add provider selection prompt with pattern family labels
    - Add masked API key input
    - Implement first-run detection and automatic setup trigger
    - Add configuration save after setup completion
    - _Requirements: 11_
  
  - [ ] 12.3 Add version command
    - Implement version command in `cmd/ghost/version.go`
    - Add semantic version display
    - Include Git commit hash and build date
    - Use Go build flags for version embedding
    - _Requirements: 26_
  
  - [ ] 12.4 Add configuration clear command
    - Implement --clear-keys command handling
    - Add confirmation prompt (unless --force flag)
    - Implement keyring and encrypted file deletion
    - _Requirements: 30_

- [ ] 13. Implement advanced features
  - [ ] 13.1 Add tool calling support
    - Extend UnifiedChatRequest and UnifiedStreamEvent for tool definitions
    - Implement tool conversion in AnthropicIn parser
    - Add tool_use event handling in AnthropicOut formatter
    - Update provider adapters to support tool calling
    - _Requirements: 20_
  
  - [ ] 13.2 Add thinking block support
    - Add thinking event type to UnifiedStreamEvent
    - Implement thinking block conversion in provider adapters
    - Add thinking event formatting in AnthropicOut formatter
    - _Requirements: 25_
  
  - [ ] 13.3 Implement error handling and propagation
    - Add error event type to UnifiedStreamEvent
    - Implement error conversion in provider adapters
    - Add error SSE event formatting in AnthropicOut formatter
    - Implement HTTP error responses for parsing and routing errors
    - _Requirements: 16_

- [ ] 14. Checkpoint - Ensure advanced features tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 15. Implement build and distribution
  - [ ] 15.1 Create cross-platform build system
    - Create GitHub Actions workflow for automated builds
    - Add build targets for Windows amd64, macOS amd64/arm64, Linux amd64
    - Implement static binary compilation with CGO_ENABLED=0
    - Add version embedding via ldflags
    - _Requirements: 18_
  
  - [ ] 15.2 Create NPM wrapper package
    - Create package.json with platform detection logic
    - Implement binary download from GitHub Releases
    - Add checksum verification
    - Create install script for node_modules/.bin placement
    - Add npx support
    - _Requirements: 23_

- [ ] 16. Integration and final wiring
  - [ ] 16.1 Wire all components together
    - Connect HTTP server to translation engine
    - Connect translation engine to provider router
    - Connect provider router to provider adapters
    - Verify end-to-end request flow
    - _Requirements: 1, 2, 3, 4, 5_
  
  - [ ] 16.2 Add connection pooling and performance optimizations
    - Implement HTTP client connection pooling in provider adapters
    - Add TCP connection reuse in HTTP server
    - Verify sub-5ms translation latency
    - _Requirements: 24_
  
  - [ ] 16.3 Add comprehensive error handling
    - Verify error propagation through all layers
    - Add error logging at appropriate levels
    - Test error scenarios (invalid JSON, provider errors, timeouts)
    - _Requirements: 16, 17_

- [ ] 17. Final checkpoint - End-to-end testing
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks involve writing, modifying, or testing Go code
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation at major milestones
- The implementation follows the modular folder structure defined in the design
- Provider adapters use pattern-based inheritance to minimize code duplication
- Security is prioritized with OS keyring storage and encrypted fallback
- Performance optimization focuses on zero-buffer streaming with immediate SSE flushing
- The build system produces static binaries for cross-platform distribution

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1"] },
    { "id": 1, "tasks": ["2.1", "2.2", "2.3"] },
    { "id": 2, "tasks": ["3.1", "3.2", "5.1"] },
    { "id": 3, "tasks": ["3.3", "5.2", "5.3"] },
    { "id": 4, "tasks": ["6.1", "6.2", "6.3"] },
    { "id": 5, "tasks": ["7.1", "7.2", "7.3", "7.4", "8.1", "10.1"] },
    { "id": 6, "tasks": ["8.2", "8.3", "8.4", "10.2", "11.1"] },
    { "id": 7, "tasks": ["11.2", "12.1"] },
    { "id": 8, "tasks": ["12.2", "12.3", "12.4"] },
    { "id": 9, "tasks": ["13.1", "13.2", "13.3"] },
    { "id": 10, "tasks": ["15.1", "15.2"] },
    { "id": 11, "tasks": ["16.1"] },
    { "id": 12, "tasks": ["16.2", "16.3"] }
  ]
}
```
