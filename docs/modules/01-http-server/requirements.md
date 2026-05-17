# HTTP Server - Requirements

## Requirement 1: HTTP Server Initialization

**User Story:** As a developer, I want the proxy to start an HTTP server on a configurable port, so that Claude Code can connect to it via ANTHROPIC_BASE_URL.

### Acceptance Criteria

1. WHEN the CLI is invoked with a valid port flag, THE HTTP_Server SHALL bind to 127.0.0.1 on the specified port
2. WHERE no port flag is provided, THE HTTP_Server SHALL bind to port 3200 by default
3. IF the specified port is already in use, THEN THE CLI SHALL return an error message indicating the port conflict and exit with status code 1
4. WHEN the HTTP_Server successfully binds, THE CLI SHALL log the listening address to stdout
5. THE HTTP_Server SHALL accept HTTP/1.1 and HTTP/2 connections
6. WHEN the HTTP_Server receives a request to /v1/messages, THE HTTP_Server SHALL route it to the Translation_Engine
7. WHEN the HTTP_Server receives a request to any path other than /v1/messages, THE HTTP_Server SHALL return HTTP 404

---

## Requirement 13: Graceful Shutdown

**User Story:** As a user, I want the proxy to shut down cleanly when I stop it, so that no requests are lost or corrupted.

### Acceptance Criteria

1. WHEN the CLI receives SIGINT or SIGTERM, THE HTTP_Server SHALL stop accepting new connections
2. THE HTTP_Server SHALL wait for all active requests to complete before shutting down
3. THE HTTP_Server SHALL enforce a shutdown timeout of 30 seconds
4. IF active requests exceed the shutdown timeout, THEN THE HTTP_Server SHALL forcefully close all connections
5. WHEN shutdown completes, THE CLI SHALL log a shutdown message and exit with status code 0
6. THE CLI SHALL close all provider API connections during shutdown

---

## Requirement 22: Health Check Endpoint

**User Story:** As a monitoring system, I want a health check endpoint, so that I can verify the proxy is running and responsive.

### Acceptance Criteria

1. THE HTTP_Server SHALL respond to GET requests at /health
2. WHEN the HTTP_Server is ready to accept requests, THE HTTP_Server SHALL return HTTP 200 for /health requests
3. THE /health response body SHALL contain JSON with status field set to ok
4. THE /health response SHALL include the active provider name
5. THE /health response SHALL include the server version
6. THE /health endpoint SHALL not require authentication
7. THE /health endpoint SHALL respond within 100 milliseconds

---

## Requirement 28: Request Timeout Configuration

**User Story:** As a user, I want to configure request timeouts, so that I can prevent hanging requests from blocking the proxy.

### Acceptance Criteria

1. THE CLI SHALL accept a --timeout flag to specify the maximum request duration in seconds
2. WHERE no --timeout flag is provided, THE HTTP_Server SHALL use a default timeout of 300 seconds
3. WHEN a request exceeds the timeout, THE HTTP_Server SHALL cancel the context
4. WHEN the context is cancelled due to timeout, THE Provider_Adapter SHALL close the provider connection
5. THE HTTP_Server SHALL return HTTP 504 when a request times out
6. THE timeout SHALL apply to the entire request lifecycle including streaming

---

## Requirement 29: CORS Support

**User Story:** As a web application developer, I want CORS headers on proxy responses, so that I can use GhostCLI from browser-based applications.

### Acceptance Criteria

1. THE HTTP_Server SHALL respond to OPTIONS requests with CORS preflight headers
2. THE HTTP_Server SHALL include Access-Control-Allow-Origin header in all responses
3. THE HTTP_Server SHALL include Access-Control-Allow-Methods header with POST, GET, OPTIONS
4. THE HTTP_Server SHALL include Access-Control-Allow-Headers header with Content-Type, Authorization
5. THE CLI SHALL accept a --cors-origin flag to specify allowed origins
6. WHERE no --cors-origin flag is provided, THE HTTP_Server SHALL use * as the default origin
7. THE HTTP_Server SHALL include Access-Control-Max-Age header with value 86400
