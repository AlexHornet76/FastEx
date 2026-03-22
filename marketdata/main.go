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

	"github.com/AlexHornet76/FastEx/marketdata/internal/api"
	"github.com/AlexHornet76/FastEx/marketdata/internal/config"
	"github.com/AlexHornet76/FastEx/marketdata/internal/consumer"
	"github.com/AlexHornet76/FastEx/marketdata/internal/state"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store := state.NewStore()

	// ---- HTTP SERVER ----
	apiServer := api.NewServer(store)

	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      apiServer.Router(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		slog.Info("marketdata listening", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
			stop()
		}
	}()

	// ---- KAFKA CONSUMER ----
	c := consumer.NewTradeConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaTopic,
		cfg.KafkaGroupID,
		store,
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Run(rootCtx)
	}()

	// ---- WAIT FOR SHUTDOWN ----
	select {
	case <-rootCtx.Done():
		slog.Info("shutdown requested")
	case err := <-errCh:
		slog.Error("consumer stopped", "error", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("http shutdown error", "error", err)
	}

	slog.Info("marketdata stopped")
}
