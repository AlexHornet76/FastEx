package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/AlexHornet76/FastEx/gateway/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetInstruments returns list of all active instruments from database
func GetInstruments(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		instruments, err := getInstrumentsFromDB(r.Context(), db)
		if err != nil {
			slog.Error("failed to get instruments", "error", err)
			http.Error(w, `{"error": "failed to fetch instruments"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(instruments)
	}
}

// GetInstrument returns a specific instrument by symbol
func GetInstrument(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		symbol := r.PathValue("symbol")

		if symbol == "" {
			http.Error(w, `{"error": "symbol required"}`, http.StatusBadRequest)
			return
		}

		var instrument models.Instrument
		query := `
			SELECT id, symbol, name, base_asset, quote_asset,
			       min_order_size, price_precision, quantity_precision,
			       is_active, created_at, updated_at
			FROM instruments
			WHERE symbol = $1 AND is_active = true
		`
		err := db.QueryRow(r.Context(), query, symbol).Scan(
			&instrument.ID,
			&instrument.Symbol,
			&instrument.Name,
			&instrument.BaseAsset,
			&instrument.QuoteAsset,
			&instrument.MinOrderSize,
			&instrument.PricePrecision,
			&instrument.QuantityPrecision,
			&instrument.IsActive,
			&instrument.CreatedAt,
			&instrument.UpdatedAt,
		)

		if err != nil {
			if err == pgx.ErrNoRows {
				http.Error(w, `{"error": "instrument not found"}`, http.StatusNotFound)
				return
			}
			slog.Error("failed to get instrument", "symbol", symbol, "error", err)
			http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(instrument)
	}
}

// getInstrumentsFromDB queries database for active instruments
func getInstrumentsFromDB(ctx context.Context, db *pgxpool.Pool) ([]models.Instrument, error) {
	query := `
		SELECT id, symbol, name, base_asset, quote_asset,
		       min_order_size, price_precision, quantity_precision,
		       is_active, created_at, updated_at
		FROM instruments
		WHERE is_active = true
		ORDER BY symbol ASC
	`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instruments []models.Instrument
	for rows.Next() {
		var inst models.Instrument
		err := rows.Scan(
			&inst.ID,
			&inst.Symbol,
			&inst.Name,
			&inst.BaseAsset,
			&inst.QuoteAsset,
			&inst.MinOrderSize,
			&inst.PricePrecision,
			&inst.QuantityPrecision,
			&inst.IsActive,
			&inst.CreatedAt,
			&inst.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		instruments = append(instruments, inst)
	}

	return instruments, rows.Err()
}
