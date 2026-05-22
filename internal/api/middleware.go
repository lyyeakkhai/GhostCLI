package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// applyMiddleware wraps handler with the full middleware chain.
// Execution order (outermost first): CORS → timeout → logging → context → handler.
func applyMiddleware(handler http.Handler, logger *slog.Logger) http.Handler {
	handler = contextMiddleware(handler)
	handler = loggingMiddleware(handler, logger)
	handler = timeoutMiddleware(handler)
	handler = corsMiddleware(handler)
	return handler
}

// corsMiddleware adds CORS headers and handles OPTIONS preflight requests.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, anthropic-version, x-api-key, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs each request's method, path, and elapsed time.
func loggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
		)

		next.ServeHTTP(w, r)

		logger.Info("response",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// timeoutMiddleware adds a request-scoped timeout to the context.
// If the handler exceeds the timeout, the client receives 504 Gateway Timeout.
func timeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()

		done := make(chan struct{})
		go func() {
			next.ServeHTTP(w, r.WithContext(ctx))
			close(done)
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			writeError(w, "timeout_error", "request timeout", http.StatusGatewayTimeout)
		}
	})
}

// contextMiddleware propagates the request context without modification.
// It serves as an extension point for injecting request-scoped values
// (e.g., request IDs, trace spans) in future iterations.
func contextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(r.Context()))
	})
}
