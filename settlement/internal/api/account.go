package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountAPI struct {
	db *pgxpool.Pool
}

func NewAccountAPI(db *pgxpool.Pool) *AccountAPI {
	return &AccountAPI{db: db}
}

// BalanceResponse represents the user's USD balance
type BalanceResponse struct {
	UserID   string `json:"user_id"`
	Balance  string `json:"balance"`
	Currency string `json:"currency"`
}

// HoldingItem represents a single asset holding
type HoldingItem struct {
	Asset    string `json:"asset"`
	Quantity string `json:"quantity"`
}

// HoldingsResponse represents all non-USD holdings
type HoldingsResponse struct {
	UserID   string        `json:"user_id"`
	Holdings []HoldingItem `json:"holdings"`
}

// LedgerEntry represents a single ledger line
type LedgerEntry struct {
	EntryID   string `json:"entry_id"`
	TradeID   string `json:"trade_id"`
	Asset     string `json:"asset"`
	Amount    string `json:"amount"`
	CreatedAt string `json:"created_at"`
}

// LedgerResponse is a paginated list of ledger entries
type LedgerResponse struct {
	Items      []LedgerEntry `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

// GetBalance handles GET /balance?user_id=...
func (a *AccountAPI) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}

	const sql = `
		SELECT COALESCE(SUM(amount), 0)::text
		FROM ledger_entries
		WHERE user_id = $1 AND asset = 'USD'
	`

	var balance string
	if err := a.db.QueryRow(r.Context(), sql, userID).Scan(&balance); err != nil {
		http.Error(w, fmt.Sprintf("db query failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BalanceResponse{
		UserID:   userID,
		Balance:  balance,
		Currency: "USD",
	})
}

// GetHoldings handles GET /holdings?user_id=...
func (a *AccountAPI) GetHoldings(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}

	const sql = `
		SELECT asset, COALESCE(SUM(amount), 0)::text AS quantity
		FROM ledger_entries
		WHERE user_id = $1 AND asset <> 'USD'
		GROUP BY asset
		HAVING SUM(amount) <> 0
		ORDER BY asset
	`

	rows, err := a.db.Query(r.Context(), sql, userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("db query failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]HoldingItem, 0)
	for rows.Next() {
		var h HoldingItem
		if err := rows.Scan(&h.Asset, &h.Quantity); err != nil {
			http.Error(w, fmt.Sprintf("db scan failed: %v", err), http.StatusInternalServerError)
			return
		}
		items = append(items, h)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("db rows error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(HoldingsResponse{
		UserID:   userID,
		Holdings: items,
	})
}

// GetLedger handles GET /ledger?user_id=...&limit=...&cursor=...&type=...
func (a *AccountAPI) GetLedger(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	userID := strings.TrimSpace(q.Get("user_id"))
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}

	asset := strings.TrimSpace(q.Get("type")) // optional filter by asset type

	const sql = `
		SELECT
			id::text,
			trade_id::text,
			asset,
			amount::text,
			created_at::text
		FROM ledger_entries
		WHERE user_id = $1
		  AND ($2 = '' OR asset = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT 50
	`

	rows, err := a.db.Query(r.Context(), sql, userID, asset)
	if err != nil {
		http.Error(w, fmt.Sprintf("db query failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]LedgerEntry, 0)
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.EntryID, &e.TradeID, &e.Asset, &e.Amount, &e.CreatedAt); err != nil {
			http.Error(w, fmt.Sprintf("db scan failed: %v", err), http.StatusInternalServerError)
			return
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("db rows error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LedgerResponse{Items: items})
}
