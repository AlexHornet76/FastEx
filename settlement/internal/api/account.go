package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
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

// Deposit handles POST /deposit?user_id=...
// Demo-only endpoint: seeds USD and instrument balances for a new user.
// Convention: 1 display-dollar = 10,000 raw units (price_cents * qty_100ths).
func (a *AccountAPI) Deposit(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}

	// USD: 1,000,000,000 raw = $100,000 display
	// Instruments: 10,000 raw = 100 units display (enough to sell)
	type deposit struct {
		asset  string
		amount int64
	}
	deposits := []deposit{{"USD", 1_000_000_000}}
	// ?seed=1 este folosit exclusiv de scriptul de seeding al datelor demo;
	// UI-ul Fund Demo nu trimite acest parametru si primeste doar USD.
	if r.URL.Query().Get("seed") == "1" {
		deposits = append(deposits,
			deposit{"BTC", 10_000},
			deposit{"ETH", 10_000},
			deposit{"AAPL", 10_000},
			deposit{"GOOGL", 10_000},
			deposit{"TSLA", 10_000},
		)
	}

	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("begin tx: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	for _, d := range deposits {
		tradeID := uuid.New()
		amountStr := fmt.Sprintf("%d", d.amount)

		if _, err := tx.Exec(ctx, `
			INSERT INTO ledger_entries (trade_id, user_id, asset, amount, created_at)
			VALUES ($1, $2::UUID, $3, $4::NUMERIC, NOW())
		`, tradeID, userID, d.asset, amountStr); err != nil {
			http.Error(w, fmt.Sprintf("ledger insert %s: %v", d.asset, err), http.StatusInternalServerError)
			return
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO balances (user_id, asset, available, locked, updated_at)
			VALUES ($1::UUID, $2, $3::NUMERIC, 0, NOW())
			ON CONFLICT (user_id, asset) DO UPDATE
			SET available = balances.available + $3::NUMERIC, updated_at = NOW()
		`, userID, d.asset, amountStr); err != nil {
			http.Error(w, fmt.Sprintf("balance upsert %s: %v", d.asset, err), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, fmt.Sprintf("commit: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// CostBasisItem represents the average buy cost for one asset
type CostBasisItem struct {
	Asset       string `json:"asset"`
	TotalQty    string `json:"total_qty"`
	AvgRawPrice string `json:"avg_raw_price"` // raw cents; divide by 100 for display
}

// CostBasisResponse is the list of cost-basis rows per asset
type CostBasisResponse struct {
	UserID    string          `json:"user_id"`
	CostBasis []CostBasisItem `json:"cost_basis"`
}

// GetCostBasis handles GET /cost-basis?user_id=...
// Returns average buy price per non-USD asset, computed from ledger JOIN on trade_id.
// Seed deposits are excluded automatically because they have no matching USD debit entry.
func (a *AccountAPI) GetCostBasis(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}

	const sql = `
		SELECT
			la.asset,
			SUM(la.amount)::text                                         AS total_qty,
			(ABS(SUM(lu.amount)) / SUM(la.amount))::numeric(20,4)::text  AS avg_raw_price
		FROM ledger_entries la
		JOIN ledger_entries lu
			ON  la.trade_id = lu.trade_id
			AND la.user_id  = lu.user_id
			AND lu.asset    = 'USD'
			AND lu.amount   < 0
		WHERE la.user_id  = $1
		  AND la.asset   <> 'USD'
		  AND la.amount  > 0
		GROUP BY la.asset
		ORDER BY la.asset
	`

	rows, err := a.db.Query(r.Context(), sql, userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("db query failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]CostBasisItem, 0)
	for rows.Next() {
		var item CostBasisItem
		if err := rows.Scan(&item.Asset, &item.TotalQty, &item.AvgRawPrice); err != nil {
			http.Error(w, fmt.Sprintf("db scan failed: %v", err), http.StatusInternalServerError)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("db rows error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CostBasisResponse{UserID: userID, CostBasis: items})
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
