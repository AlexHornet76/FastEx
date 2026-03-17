package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlexHornet76/FastEx/engine/internal/config"
	"github.com/AlexHornet76/FastEx/engine/internal/engine"
	"github.com/AlexHornet76/FastEx/engine/internal/handlers"
	"github.com/AlexHornet76/FastEx/engine/internal/kafka"
	"github.com/AlexHornet76/FastEx/engine/internal/logger"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Init logger
	logger.Init(cfg.LogLevel)
	slog.Info("starting matching engine service", "version", "sprint-3.6")

	// Kafka producer (used ONLY by WAL publisher)
	var producer *kafka.Producer
	if cfg.KafkaEnabled {
		p, err := kafka.NewProducer(cfg.KafkaBrokers)
		if err != nil {
			slog.Error("failed to initialize Kafka producer", "error", err)
			os.Exit(1)
		}
		producer = p
		slog.Info("Kafka producer initialized", "brokers", cfg.KafkaBrokers)
		defer producer.Close()
	} else {
		slog.Info("Kafka producer disabled")
	}

	// Init engines (NO kafka producer passed)
	engines := make(map[string]*engine.Engine)
	for _, instrument := range cfg.Instruments {
		eng, err := engine.NewEngine(instrument, cfg.WALDir)
		if err != nil {
			slog.Error("failed to initialize engine", "instrument", instrument, "error", err)
			os.Exit(1)
		}
		engines[instrument] = eng
		slog.Info("engine initialized", "instrument", instrument)
	}

	// Start WAL publishers
	if producer != nil {
		pubCtx, pubCancel := context.WithCancel(context.Background())
		defer pubCancel()

		for _, instrument := range cfg.Instruments {
			wp := kafka.NewWALPublisher(producer, cfg.WALDir, instrument)
			go wp.Run(pubCtx)
		}
	}

	// Initialize handlers
	orderHandler := handlers.NewOrderHandler(engines)
	healthHandler := handlers.NewHealthHandler(engines)

	// Setup routes
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", healthHandler.Health)

	// Orders
	mux.HandleFunc("POST /orders", orderHandler.SubmitOrder)
	mux.HandleFunc("DELETE /orders/{id}", orderHandler.CancelOrder)

	// Order book
	mux.HandleFunc("GET /orderbook/{instrument}", orderHandler.GetOrderBook)

	// Apply CORS middleware (for browser clients)
	handler := corsMiddleware()(mux)

	// Apply recovery middleware
	handler = recoveryMiddleware(handler)

	// HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("matching engine listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// Close all engines (flush WAL)
	for instrument, eng := range engines {
		if err := eng.Close(); err != nil {
			slog.Error("failed to close engine", "instrument", instrument, "error", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exited")
}

// corsMiddleware adds CORS headers
func corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*") // TODO: Configure allowed origins
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// recoveryMiddleware recovers from panics
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
