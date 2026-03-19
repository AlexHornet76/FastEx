package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/AlexHornet76/FastEx/gateway/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.up.sql
var migrationFiles embed.FS

// Connect establishes PostgreSQL connection pool
func Connect(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	poolConfig.MaxConns = int32(runtime.NumCPU() * 5)
	poolConfig.MinConns = int32(runtime.NumCPU())
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// RunMigrations applies all embedded *.up.sql files in lexicographic order.
// Each file is executed in its own transaction so a failure rolls back only
// that migration and surfaces a clear error.
func RunMigrations(ctx context.Context, db *pgxpool.Pool) error {
	entries, err := fs.Glob(migrationFiles, "migrations/*.up.sql")
	if err != nil {
		return fmt.Errorf("glob migration files: %w", err)
	}

	// Guarantee deterministic order (001 → 002 → …)
	sort.Strings(entries)

	for _, path := range entries {
		name := path[len("migrations/"):]

		sqlBytes, err := migrationFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		sql := strings.TrimSpace(string(sqlBytes))
		if sql == "" {
			continue
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, sql); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute migration %s: %w", name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}

		slog.Info("migration applied", "file", name)
	}

	return nil
}

// CleanupExpiredChallenges runs periodic cleanup job
func CleanupExpiredChallenges(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			result, err := db.Exec(ctx, "DELETE FROM auth_challenges WHERE expires_at < NOW()")
			if err != nil {
				slog.Error("challenge cleanup failed", "error", err)
				continue
			}
			if result.RowsAffected() > 0 {
				slog.Debug("expired challenges deleted", "count", result.RowsAffected())
			}
		case <-ctx.Done():
			slog.Info("challenge cleanup stopped")
			return
		}
	}
}
