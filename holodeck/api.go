package holodeck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

// Server is the Holodeck HTTP server.
type Server struct {
	manager *WorldManager
	mux     *http.ServeMux
}

func NewServer(manager *WorldManager) *Server {
	s := &Server{manager: manager, mux: http.NewServeMux()}

	s.mux.HandleFunc("GET /v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /v1/worlds", s.handleListWorlds)

	// World-scoped routes.
	s.mux.HandleFunc("GET /v1/worlds/{id}", s.handleGetWorld)
	s.mux.HandleFunc("PUT /v1/worlds/{id}", s.handleSetWorld)
	s.mux.HandleFunc("POST /v1/worlds/{id}/reset", s.handleResetWorld)
	s.mux.HandleFunc("GET /v1/worlds/{id}/metrics", s.handleQueryMetric)

	// Shorthand routes — default world.
	s.mux.HandleFunc("GET /v1/world", s.defaultWorld(s.handleGetWorld))
	s.mux.HandleFunc("PUT /v1/world", s.defaultWorld(s.handleSetWorld))
	s.mux.HandleFunc("POST /v1/world/reset", s.defaultWorld(s.handleResetWorld))
	s.mux.HandleFunc("GET /v1/metrics", s.defaultWorld(s.handleQueryMetric))

	s.mux.HandleFunc("GET /v1/sampled-metrics", s.handleGetSampledMetrics)

	// UI.
	s.mux.HandleFunc("GET /", s.handleUI)

	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

// defaultWorld wraps a handler so that it operates on the default world by
// injecting the default world ID as the "id" path value.
func (s *Server) defaultWorld(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", DefaultWorldID)
		h(w, r)
	}
}

// --- handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListWorlds(w http.ResponseWriter, _ *http.Request) {
	ids := s.manager.List()
	sort.Strings(ids)
	writeJSON(w, http.StatusOK, map[string][]string{"worlds": ids})
}

func (s *Server) handleGetWorld(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	world := s.manager.Get(id)
	if world == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("world not found: %s", id))
		return
	}
	writeJSON(w, http.StatusOK, worldState(world))
}

func (s *Server) handleSetWorld(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Metrics map[string]MetricRule `json:"metrics"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Metrics == nil {
		req.Metrics = map[string]MetricRule{}
	}

	world := s.manager.Set(id, req.Metrics)
	writeJSON(w, http.StatusOK, worldState(world))
}

func (s *Server) handleResetWorld(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.manager.Reset(id)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "reset": true})
}

func (s *Server) handleQueryMetric(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		writeError(w, http.StatusBadRequest, "metric parameter required")
		return
	}

	world := s.manager.Get(id)
	if world == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("world not found: %s", id))
		return
	}

	value, queriedAt, ok := world.Query(metric, s.manager.AllocCount(), s.manager.NodeCount())
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("metric not found: %s", metric))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"metric":     metric,
		"value":      value,
		"queried_at": queriedAt,
	})
}

func (s *Server) handleGetSampledMetrics(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"metrics": s.manager.GetSampledMetrics(),
	})
}

// --- helpers ---

type worldStateResponse struct {
	ID      string                `json:"id"`
	Metrics map[string]MetricRule `json:"metrics"`
}

func worldState(w *World) worldStateResponse {
	return worldStateResponse{ID: w.ID(), Metrics: w.State()}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
