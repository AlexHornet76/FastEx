package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/AlexHornet76/FastEx/marketdata/internal/state"
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
	mux.HandleFunc("GET /market/candles/{instrument}", s.getCandles)
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

func (s *Server) getCandles(w http.ResponseWriter, r *http.Request) {
	inst := r.PathValue("instrument")
	if inst == "" {
		http.Error(w, "instrument required", http.StatusBadRequest)
		return
	}

	limit := 10 // default
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 500 {
			http.Error(w, "invalid limit (1..500)", http.StatusBadRequest)
			return
		}
		limit = n
	}

	candles, ok := s.store.GetCandles(inst, limit)
	if !ok {
		http.Error(w, "candles not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"instrument": inst,
		"limit":      limit,
		"candles":    candles,
	})
}
