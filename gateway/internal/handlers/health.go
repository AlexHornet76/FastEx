package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler returns service health status
func HealthHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		// Check database connection
		dbStatus := "connected"
		if err := db.Ping(ctx); err != nil {
			dbStatus = "disconnected"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		response := map[string]interface{}{
			"status":    "healthy",
			"database":  dbStatus,
			"timestamp": time.Now().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
