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

	"github.com/AlexHornet76/FastEx/gateway/internal/auth"
	"github.com/AlexHornet76/FastEx/gateway/internal/config"
	"github.com/AlexHornet76/FastEx/gateway/internal/database"
	"github.com/AlexHornet76/FastEx/gateway/internal/handlers"
	"github.com/AlexHornet76/FastEx/gateway/internal/logger"
	"github.com/AlexHornet76/FastEx/gateway/internal/matching"
	"github.com/gorilla/websocket"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	logger.Init(cfg.LogLevel)
	slog.Info("starting gateway service", "version", "sprint-1")

	// Connect to PostgreSQL
	ctx := context.Background()
	db, err := database.Connect(ctx, cfg)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database connected", "host", cfg.PostgresHost, "database", cfg.PostgresDB)

	// Run migrations
	if err := database.RunMigrations(ctx, db); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations completed")

	// Start background challenge cleanup
	go database.CleanupExpiredChallenges(ctx, db)

	// Initialize matching engine client
	matchingClient := matching.NewClient(cfg.MatchingEngineURL)
	slog.Info("matching engine client initialized", "url", cfg.MatchingEngineURL)

	// Check matching engine health
	if err := matchingClient.HealthCheck(); err != nil {
		slog.Warn("matching engine health check failed (will retry on requests)", "error", err)
	} else {
		slog.Info("matching engine health check passed")
	}

	// Initialize WebSocket upgrader
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     checkOrigin(cfg.CORSAllowedOrigins),
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, cfg)
	wsHandler := handlers.NewWebSocketHandler(upgrader, cfg.JWTSecret)
	orderHandler := handlers.NewOrderHandler(matchingClient)

	// Setup routes
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/health", handlers.HealthHandler(db))
	mux.HandleFunc("POST /auth/register", authHandler.Register)
	mux.HandleFunc("POST /auth/challenge", authHandler.Challenge)
	mux.HandleFunc("POST /auth/verify", authHandler.Verify)

	// WebSocket
	mux.HandleFunc("/ws", wsHandler.HandleConnection)

	// Protected routes (example)
	mux.Handle("GET /api/user/profile", auth.JWTMiddleware(cfg.JWTSecret)(
		http.HandlerFunc(authHandler.GetProfile),
	))

	// Order routes
	mux.Handle("POST /api/orders", auth.JWTMiddleware(cfg.JWTSecret)(
		http.HandlerFunc(orderHandler.SubmitOrder),
	))
	mux.Handle("DELETE /api/orders/{id}", auth.JWTMiddleware(cfg.JWTSecret)(
		http.HandlerFunc(orderHandler.CancelOrder),
	))
	mux.Handle("GET /api/orderbook/{instrument}", auth.JWTMiddleware(cfg.JWTSecret)(
		http.HandlerFunc(orderHandler.GetOrderBook),
	))

	// Apply CORS middleware
	handler := corsMiddleware(cfg.CORSAllowedOrigins)(mux)

	// Apply recovery middleware
	handler = recoveryMiddleware(handler)

	// HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.GatewayPort),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("gateway listening", "port", cfg.GatewayPort)
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exited")
}

// corsMiddleware adds CORS headers for Next.js frontend
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range allowedOrigins {
				if origin == o {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// Handle preflight
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

// checkOrigin creates WebSocket origin checker
func checkOrigin(allowedOrigins []string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		for _, o := range allowedOrigins {
			if origin == o {
				return true
			}
		}
		// Allow same-origin for development
		return origin == ""
	}
}
