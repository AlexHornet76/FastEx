package handlers

import (
	"net/http"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/engine"
)

type HealthHandler struct {
	engines map[string]*engine.Engine
}

func NewHealthHandler(engines map[string]*engine.Engine) *HealthHandler {
	return &HealthHandler{
		engines: engines,
	}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	instruments := make(map[string]string)
	overallHealthy := true

	for instrument, eng := range h.engines {
		if eng == nil {
			instruments[instrument] = "unavailable"
			overallHealthy = false
			continue
		}

		ob := eng.GetOrderBook()
		if ob == nil {
			instruments[instrument] = "degraded"
			overallHealthy = false
		} else {
			instruments[instrument] = "healthy"
		}
	}

	status := "healthy"
	if !overallHealthy {
		status = "degraded"
	}

	response := HealthResponse{
		Status:      status,
		Instruments: instruments,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	// Returnează 503 dacă nu e healthy
	statusCode := http.StatusOK
	if !overallHealthy {
		statusCode = http.StatusServiceUnavailable
	}

	respondJSON(w, statusCode, response)
}
