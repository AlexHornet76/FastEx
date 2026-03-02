package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GenerateChallenge creates a cryptographically secure random challenge
func GenerateChallenge() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// StoreChallenge saves challenge to database with expiration
func StoreChallenge(ctx context.Context, db *pgxpool.Pool, username, challenge string, ttlMinutes int) error {
	expiresAt := time.Now().Add(time.Duration(ttlMinutes) * time.Minute)

	query := `
		INSERT INTO auth_challenges (username, challenge, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err := db.Exec(ctx, query, username, challenge, expiresAt)
	if err != nil {
		return fmt.Errorf("store challenge: %w", err)
	}

	return nil
}

// VerifyChallenge checks challenge exists, not expired, and deletes it (one-time use)
func VerifyChallenge(ctx context.Context, db *pgxpool.Pool, username, challenge string) error {
	// Check and delete in transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var expiresAt time.Time
	query := `
		SELECT expires_at FROM auth_challenges
		WHERE username = $1 AND challenge = $2
		FOR UPDATE
	`
	err = tx.QueryRow(ctx, query, username, challenge).Scan(&expiresAt)
	if err != nil {
		return fmt.Errorf("challenge not found or already used")
	}

	// Check expiration
	if time.Now().After(expiresAt) {
		return fmt.Errorf("challenge expired")
	}

	// Delete challenge (one-time use)
	_, err = tx.Exec(ctx, "DELETE FROM auth_challenges WHERE username = $1 AND challenge = $2", username, challenge)
	if err != nil {
		return fmt.Errorf("delete challenge: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
