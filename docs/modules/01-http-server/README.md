# Module 01: HTTP Server

## Overview

The HTTP Server module provides the transport layer for GhostCLI, handling incoming requests from Claude Code and routing them to the translation engine.

## Responsibilities

- Initialize HTTP server on configurable port
- Route requests to appropriate handlers
- Apply middleware (CORS, logging, context)
- Provide health check endpoint
- Handle graceful shutdown
- Manage request timeouts

## Architecture

```
HTTP Server
├── Server Initialization
├── Request Routing
│   ├── /v1/messages → Translation Engine
│   └── /health → Health Check Handler
├── Middleware Stack
│   ├── CORS Headers
│   ├── Request Logging
│   └── Context Propagation
└── Lifecycle Management
    ├── Startup
    └── Graceful Shutdown
```

## Related Requirements

- **Requirement 1**: HTTP Server Initialization
- **Requirement 13**: Graceful Shutdown
- **Requirement 22**: Health Check Endpoint
- **Requirement 28**: Request Timeout Configuration
- **Requirement 29**: CORS Support

## Key Components

### Server Initialization
- Bind to 127.0.0.1 on configurable port (default: 3200)
- Support HTTP/1.1 and HTTP/2
- Detect port conflicts and fail fast
- Log listening address on successful bind

### Request Routing
- `/v1/messages` (POST) → Translation Engine
- `/health` (GET) → Health Check Handler
- All other paths → HTTP 404

### Middleware
- **CORS**: Add Access-Control headers for browser compatibility
- **Logging**: Log request method, path, status code
- **Context**: Create request context for cancellation propagation

### Health Check
- Return HTTP 200 with JSON status
- Include active provider name
- Include server version
- Respond within 100ms

### Graceful Shutdown
- Stop accepting new connections on SIGINT/SIGTERM
- Wait for active requests to complete (30s timeout)
- Force close connections after timeout
- Log shutdown message

## Implementation Details

See [design.md](./design.md) for detailed implementation specifications.
