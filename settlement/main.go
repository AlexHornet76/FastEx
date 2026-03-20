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

	"github.com/AlexHornet76/FastEx/settlement/internal/api"
	"github.com/AlexHornet76/FastEx/settlement/internal/config"
	"github.com/AlexHornet76/FastEx/settlement/internal/consumer"
	"github.com/AlexHornet76/FastEx/settlement/internal/db"
	"github.com/AlexHornet76/FastEx/settlement/internal/settle"
)

func main() {
	cfg := config.Load()

	// basic slog setup (keep simple, consistent with your project)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)

	slog.Info("starting settlement service", "topic", cfg.KafkaTopic, "group", cfg.KafkaGroupID)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Connect DB
	dbCtx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
	defer cancel()

	database, err := db.Connect(dbCtx,
		cfg.PostgresHost, cfg.PostgresPort,
		cfg.PostgresUser, cfg.PostgresPassword,
		cfg.PostgresDB, cfg.PostgresSSLMode,
	)
	if err != nil {
		slog.Error("failed to connect postgres", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	slog.Info("connected to postgres")

	// HTTP health
	api := api.NewServer(database.Pool)
	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      api.Router(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		slog.Info("settlement http listening", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
			stop()
		}
	}()

	settler := settle.NewSettler(database.Pool)

	// Kafka consumer
	c := consumer.NewTradeConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroupID, settler)

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
		os.Exit(1)
	}

	slog.Info("settlement exited")
}
