package database

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/AlexHornet76/FastEx/gateway/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect establishes PostgreSQL connection pool
func Connect(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	// Connection pool settings
	poolConfig.MaxConns = int32(runtime.NumCPU() * 5) // 20 on 4-core VPS
	poolConfig.MinConns = int32(runtime.NumCPU())     // 4 keep-alive connections
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// RunMigrations applies database schema
// RunMigrations applies database schema (embedded SQL)
func RunMigrations(ctx context.Context, db *pgxpool.Pool) error {
	// Inline migration SQL (no external files needed)
	migrationSQL := `
-- Sprint 1: Gateway + Authentication Schema

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    public_key TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Authentication challenges table
CREATE TABLE IF NOT EXISTS auth_challenges (
    challenge_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) NOT NULL,
    challenge TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_challenges_challenge ON auth_challenges(challenge);
CREATE INDEX IF NOT EXISTS idx_challenges_expires ON auth_challenges(expires_at);
	`

	_, err := db.Exec(ctx, migrationSQL)
	if err != nil {
		return fmt.Errorf("execute migrations: %w", err)
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
