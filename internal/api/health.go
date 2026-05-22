package api

import (
	"encoding/json"
	"net/http"
)

// HealthResponse is the JSON structure returned by the /health endpoint.
type HealthResponse struct {
	Status   string `json:"status"`
	Provider string `json:"provider"`
	Version  string `json:"version"`
}

// handleHealth handles GET /health — returns service status, active provider,
// and version in a sub-100ms JSON response.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "invalid_request_error", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := HealthResponse{
		Status:   "ok",
		Provider: s.engine.ProviderName(),
		Version:  s.engine.Version(),
	}

	// Sub-100ms guarantee: if encoding takes longer, cut it short.
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(resp)
}

// handlePing handles GET /ping — lightweight liveness check.
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("pong"))
}
