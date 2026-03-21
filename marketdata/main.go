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

	"github.com/AlexHornet76/FastEx/marketdata/api"
	"github.com/AlexHornet76/FastEx/marketdata/config"
	"github.com/AlexHornet76/FastEx/marketdata/consumer"
	"github.com/AlexHornet76/FastEx/marketdata/state"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store := state.NewStore()

	// HTTP
	api := api.NewServer(store)
	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      api.Router(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		slog.Info("marketdata listening", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			stop()
		}
	}()

	// Kafka consumer
	c := consumer.NewTradeConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroupID, store)
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Run(rootCtx)
	}()

	select {
	case <-rootCtx.Done():
		slog.Info("shutdown requested")
	case err := <-errCh:
		slog.Error("consumer stopped", "error", err)
		fmt.Fprintln(os.Stderr, err)
	}
}
