package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexHornet76/FastEx/gateway/internal/auth"
	"github.com/AlexHornet76/FastEx/gateway/internal/config"
	"github.com/AlexHornet76/FastEx/gateway/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthHandler struct {
	db  *pgxpool.Pool
	cfg *config.Config
}

func NewAuthHandler(db *pgxpool.Pool, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

// Register creates new user with public key
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username  string `json:"username"`
		PublicKey string `json:"public_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Validate inputs
	if req.Username == "" || req.PublicKey == "" {
		http.Error(w, `{"error": "username and public_key required"}`, http.StatusBadRequest)
		return
	}

	// Validate public key format (hex-encoded, 64 chars for Ed25519)
	if len(req.PublicKey) != 64 {
		http.Error(w, `{"error": "invalid public key format (expected 64 hex chars)"}`, http.StatusBadRequest)
		return
	}

	// Insert user
	var user models.User
	query := `
		INSERT INTO users (username, public_key)
		VALUES ($1, $2)
		RETURNING user_id, username, created_at
	`
	err := h.db.QueryRow(r.Context(), query, req.Username, req.PublicKey).
		Scan(&user.UserID, &user.Username, &user.CreatedAt)

	if err != nil {
		// Check for duplicate username (pgconn.PgError for pgx v5)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, `{"error": "username already exists"}`, http.StatusConflict)
			return
		}
		slog.Error("register user failed", "error", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("user registered", "user_id", user.UserID, "username", user.Username)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// Challenge generates and returns authentication challenge
func (h *AuthHandler) Challenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, `{"error": "username required"}`, http.StatusBadRequest)
		return
	}

	// Verify user exists
	var userID uuid.UUID
	err := h.db.QueryRow(r.Context(), "SELECT user_id FROM users WHERE username = $1", req.Username).
		Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error": "user not found"}`, http.StatusNotFound)
			return
		}
		slog.Error("query user failed", "error", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Generate challenge
	challenge, err := auth.GenerateChallenge()
	if err != nil {
		slog.Error("generate challenge failed", "error", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Store challenge
	if err := auth.StoreChallenge(r.Context(), h.db, req.Username, challenge, h.cfg.ChallengeTTLMinutes); err != nil {
		slog.Error("store challenge failed", "error", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	slog.Debug("challenge generated", "username", req.Username, "length", len(challenge))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"challenge": challenge,
	})
}

// Verify validates signed challenge and issues JWT
func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username  string `json:"username"`
		Challenge string `json:"challenge"`
		Signature string `json:"signature"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Validate timestamp skew (prevent replay with old signatures)
	now := time.Now().Unix()
	if abs(now-req.Timestamp) > 300 { // 5 minutes
		http.Error(w, `{"error": "timestamp skew too large"}`, http.StatusBadRequest)
		return
	}

	// Verify challenge exists and not expired
	if err := auth.VerifyChallenge(r.Context(), h.db, req.Username, req.Challenge); err != nil {
		slog.Warn("challenge verification failed", "username", req.Username, "error", err)
		http.Error(w, `{"error": "invalid or expired challenge"}`, http.StatusBadRequest)
		return
	}

	// Get user's public key
	var user models.User
	query := "SELECT user_id, username, public_key FROM users WHERE username = $1"
	err := h.db.QueryRow(r.Context(), query, req.Username).
		Scan(&user.UserID, &user.Username, &user.PublicKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error": "user not found"}`, http.StatusNotFound)
			return
		}
		slog.Error("query user failed", "error", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Verify signature
	if err := auth.VerifyEd25519Signature(user.PublicKey, req.Challenge, req.Signature); err != nil {
		slog.Warn("signature verification failed", "username", req.Username, "error", err)
		http.Error(w, `{"error": "invalid signature"}`, http.StatusBadRequest)
		return
	}

	// Generate JWT
	token, err := auth.GenerateJWT(user.UserID, user.Username, h.cfg.JWTSecret, h.cfg.JWTExpiryMinutes)
	if err != nil {
		slog.Error("jwt generation failed", "error", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("user authenticated", "user_id", user.UserID, "username", user.Username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      token,
		"expires_in": h.cfg.JWTExpiryMinutes * 60, // seconds
	})
}

// GetProfile returns current user profile (protected endpoint example)
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Get user details from database
	var user models.User
	query := "SELECT user_id, username, created_at FROM users WHERE user_id = $1"
	err := h.db.QueryRow(r.Context(), query, claims.UserID).
		Scan(&user.UserID, &user.Username, &user.CreatedAt)
	if err != nil {
		slog.Error("query user profile failed", "error", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
