package api

import (
	"encoding/json"
	"net/http"
)

// handleMessages handles POST /v1/messages — the primary proxy endpoint.
// It delegates parsing, routing, and SSE streaming to the engine.
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "invalid_request_error", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.engine.StreamRequest(r.Context(), w, r); err != nil {
		s.logger.Error("stream request failed", "error", err, "path", r.URL.Path)
		// Only write an error response if headers have not been sent yet.
		// If streaming already started the client will see a truncated SSE stream.
		writeError(w, "api_error", err.Error(), http.StatusBadGateway)
	}
}

// handleNotFound handles all unmatched routes.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, "not_found", "endpoint not found", http.StatusNotFound)
}

// writeError writes a JSON error response in the Anthropic error envelope format.
func writeError(w http.ResponseWriter, errType, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]string{
			"type":    errType,
			"message": message,
		},
	})
}
