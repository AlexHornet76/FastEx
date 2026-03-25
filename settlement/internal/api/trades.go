package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TradeAPI struct {
	db *pgxpool.Pool
}

func NewTradeAPI(db *pgxpool.Pool) *TradeAPI {
	return &TradeAPI{db: db}
}

type TradeHistory struct {
	TradeID     string    `json:"trade_id"`
	Instrument  string    `json:"instrument"`
	Side        string    `json:"side"` // BUY or SELL (relative to user)
	Price       string    `json:"price"`
	Quantity    string    `json:"quantity"`
	Status      string    `json:"status"`
	Reason      string    `json:"reason,omitempty"`
	ProcessedAt time.Time `json:"processed_at"`
}

type TradeHistoryResponse struct {
	Items      []TradeHistory `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

func (a *TradeAPI) GetTrades(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	userID := strings.TrimSpace(q.Get("user_id"))
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}
	limit := 50
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 200 {
			http.Error(w, "invalid limit (1..200)", http.StatusBadRequest)
			return
		}
		limit = n
	}

	status := strings.TrimSpace(q.Get("status"))
	instrument := strings.TrimSpace(q.Get("instrument"))

	cursor := strings.TrimSpace(q.Get("cursor")) // optional
	var cursorTime *time.Time
	var cursorTradeID *string
	if cursor != "" {
		ct, tid, err := decodeCursor(cursor)
		if err != nil {
			http.Error(w, "invalid cursor", http.StatusBadRequest)
			return
		}
		cursorTime = &ct
		cursorTradeID = &tid
	}

	const sql = `
	SELECT
		pt.trade_id::text AS trade_id,
		lei.asset AS instrument,
		CASE WHEN lei.amount > 0 THEN 'BUY' ELSE 'SELL' END AS side,
		abs(lei.amount)::text AS quantity,
		(abs(leusd.amount) / NULLIF(abs(lei.amount), 0))::text AS price,
		pt.status::text AS status,
		COALESCE(pt.reason, '') AS reason,
		pt.processed_at AS processed_at
	FROM processed_trades pt
	JOIN ledger_entries lei ON pt.trade_id = lei.trade_id AND lei.user_id = $1 AND lei.asset <> 'USD'
	JOIN ledger_entries leusd ON pt.trade_id = leusd.trade_id AND leusd.user_id = $1 AND leusd.asset = 'USD'
	WHERE
		($2 = '' OR pt.status = $2)
		AND ($3 = '' OR lei.asset = $3)
		AND (
			$4::timestamptz IS NULL
			OR (pt.processed_at, pt.trade_id) < ($4::timestamptz, $5::uuid)
		)
	ORDER BY pt.processed_at DESC, pt.trade_id DESC
	LIMIT $6;
		`

	args := []any{
		userID,
		status,
		instrument,
		nil,
		nil,
		limit + 1,
	}
	if cursorTime != nil && cursorTradeID != nil {
		args[3] = *cursorTime
		args[4] = *cursorTradeID
	}
	rows, err := a.db.Query(r.Context(), sql, args...)
	if err != nil {
		http.Error(w, fmt.Sprintf("db query failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]TradeHistory, 0, limit)
	var nextCursor string

	for rows.Next() {
		var th TradeHistory
		var processedAt time.Time

		if err := rows.Scan(&th.TradeID, &th.Instrument, &th.Side, &th.Quantity, &th.Price, &th.Status, &th.Reason, &processedAt); err != nil {
			http.Error(w, fmt.Sprintf("db scan failed: %v", err), http.StatusInternalServerError)
			return
		}

		if len(items) == limit {
			nextCursor = encodeCursor(processedAt, th.TradeID)
			break
		}
		th.ProcessedAt = processedAt.UTC()
		items = append(items, th)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("db rows error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(TradeHistoryResponse{
		Items:      items,
		NextCursor: nextCursor,
	})
}

func decodeCursor(cursor string) (time.Time, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("bad cursor format")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return t, parts[1], nil
}

// Cursor format: base64url("RFC3339Nano|trade_id")
func encodeCursor(t time.Time, tradeID string) string {
	raw := t.UTC().Format(time.RFC3339Nano) + "|" + tradeID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}
