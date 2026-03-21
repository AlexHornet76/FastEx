package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexHornet76/FastEx/gateway/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DepositHandler struct {
	db *pgxpool.Pool
}

func NewDepositHandler(db *pgxpool.Pool) *DepositHandler {
	return &DepositHandler{db: db}
}

type DepositRequest struct {
	Asset  string  `json:"asset"`
	Amount float64 `json:"amount"`
}

func (r *DepositRequest) Validate() error {
	if r.Asset == "" {
		return &ValidationError{"asset is required"}
	}
	if r.Amount <= 0 {
		return &ValidationError{"amount must be positive"}
	}
	return nil
}

type DepositResponse struct {
	UserID    string `json:"user_id"`
	Asset     string `json:"asset"`
	Available int64  `json:"available"`
	Locked    int64  `json:"locked"`
}

func (h *DepositHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var available int64
	var locked int64
	err := h.db.QueryRow(ctx, `
		INSERT INTO balances (user_id, asset, available, locked, updated_at)
		VALUES ($1::uuid, $2, $3::numeric, 0, NOW())
		ON CONFLICT (user_id, asset)
		DO UPDATE SET
			available = balances.available + EXCLUDED.available,
			updated_at = NOW()
		RETURNING available::bigint, locked::bigint
	`, claims.UserID, req.Asset, req.Amount).Scan(&available, &locked)
	if err != nil {
		slog.Error("deposit failed", "user_id", claims.UserID, "asset", req.Asset, "amount", req.Amount, "error", err)
		respondError(w, http.StatusInternalServerError, "DEPOSIT_FAILED", "Deposit failed")
		return
	}

	slog.Info("deposit applied", "user_id", claims.UserID, "asset", req.Asset, "amount", req.Amount, "available", available)
	respondJSON(w, http.StatusOK, DepositResponse{
		UserID:    claims.UserID,
		Asset:     req.Asset,
		Available: available,
		Locked:    locked,
	})
}
