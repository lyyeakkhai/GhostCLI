// Package api provides the HTTP transport layer for the GhostCLI proxy.
// It handles routing, middleware, and SSE streaming of Anthropic API requests.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Engine is the minimal interface the server requires from the translation engine.
// It decouples the api package from the concrete engine implementation.
type Engine interface {
	ParseRequest(r *http.Request) (interface{}, error)
	StreamRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error
	ProviderName() string
	Version() string
}

// Server is the HTTP server for the GhostCLI proxy.
type Server struct {
	httpServer *http.Server
	engine     Engine
	logger     *slog.Logger
}

// NewServer creates and configures a new Server instance.
// It wires up all routes and middleware before returning.
func NewServer(port string, engine Engine, logger *slog.Logger) *Server {
	s := &Server{
		engine: engine,
		logger: logger,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:              ":" + port,
		Handler:           applyMiddleware(mux, logger),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return s
}

// registerRoutes wires all HTTP routes to their handlers.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/messages", s.handleMessages)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/", s.handleNotFound)
}

// Start begins listening for incoming connections.
// It returns http.ErrServerClosed on graceful shutdown, or another error
// if the server fails to start.
func (s *Server) Start() error {
	s.logger.Info("server starting", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Addr returns the server's listen address (e.g., ":3200").
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// Shutdown gracefully stops the server, waiting up to the context deadline
// for active connections to finish.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down")
	return s.httpServer.Shutdown(ctx)
}
