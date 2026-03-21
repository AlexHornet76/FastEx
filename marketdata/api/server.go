package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/AlexHornet76/FastEx/marketdata/state"
)

type Server struct {
	store *state.Store
}

func NewServer(store *state.Store) *Server {
	return &Server{store: store}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /market/ticker/{instrument}", s.getTicker)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (s *Server) getTicker(w http.ResponseWriter, r *http.Request) {
	inst := r.PathValue("instrument")
	if inst == "" {
		http.Error(w, "instrument required", http.StatusBadRequest)
		return
	}

	t, ok := s.store.GetTicker(inst)
	if !ok {
		http.Error(w, "ticker not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(t)
}
