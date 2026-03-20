package settle

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexHornet76/FastEx/settlement/internal/events"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const USDAsset = "USD"

type Settler struct {
	db *pgxpool.Pool
}

func NewSettler(db *pgxpool.Pool) *Settler {
	return &Settler{db: db}
}

func upsertAdd(ctx context.Context, tx pgx.Tx, userID string, asset string, delta string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO balances (user_id, asset, available, locked, updated_at)
		VALUES ($1::UUID, $2, $3::NUMERIC, 0, NOW()) 
		ON CONFLICT (user_id, asset) DO UPDATE 
		SET available = balances.available + EXCLUDED.available, updated_at = NOW()`,
		userID, asset, delta)
	return err
}

// ApplyTrade applies a trade event idempotently using processed_trades.
// It writes ledger_entries + updates balances + marks processed, all in one transaction.
//
// Returns:
// - applied=true  => APPLIED
// - applied=false => SKIPPED (already processed) OR REJECTED (but then err=nil)
func (s *Settler) ApplyTrade(ctx context.Context, ev *events.TradeExecutedEvent) (applied bool, err error) {
	// int64 -> NUMERIC safely by passing as string
	// We'll store instrument quantity and USD value.
	instrumentQty := ev.Quantity
	usdValue, okOF := mulCheckOverflow(ev.Price, ev.Quantity)
	if !okOF {
		return s.reject(ctx, ev, fmt.Sprintf("INVALID_EVENT: notional overflow price=%d qty=%d", ev.Price, ev.Quantity))
	}

	// Sanity checks
	if ev.TradeID == [16]byte{} || ev.Instrument == "" || ev.BuyerUserID == [16]byte{} || ev.SellerUserID == [16]byte{} {
		return s.reject(ctx, ev, "INVALID_EVENT: missing required ids/fields")
	}
	if instrumentQty <= 0 {
		return s.reject(ctx, ev, fmt.Sprintf("INVALID_EVENT: quantity=%d", instrumentQty))
	}
	if ev.Price <= 0 {
		return s.reject(ctx, ev, fmt.Sprintf("INVALID_EVENT: price=%d", ev.Price))
	}
	if usdValue <= 0 {
		return s.reject(ctx, ev, fmt.Sprintf("INVALID_EVENT: notional=%d", usdValue))
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted, // sees only committed modifications
		AccessMode: pgx.ReadWrite,     // permits writes
	})

	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}

	// rollback if anything goes wrong
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Idempotency check: if already processed, do nothing
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM processed_trades WHERE trade_id = $1)`,
		ev.TradeID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check processed_trades: %w", err)
	}
	if exists {
		// Already applied
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit (noop): %w", err)
		}
		return false, nil
	}

	// Insufficient funds check
	buyerUSD, err := lockAndGetAvailable(ctx, tx, ev.BuyerUserID.String(), USDAsset)
	if err != nil {
		return false, fmt.Errorf("read buyer USD: %w", err)
	}
	sellerInstr, err := lockAndGetAvailable(ctx, tx, ev.SellerUserID.String(), ev.Instrument)
	if err != nil {
		return false, fmt.Errorf("read seller instrument: %w", err)
	}

	if buyerUSD < usdValue {
		reason := fmt.Sprintf("INSUFFICIENT_FUNDS: buyer USD available=%d needed=%d", buyerUSD, usdValue)
		if err := markProcessed(ctx, tx, ev.TradeID, "REJECTED", reason); err != nil {
			return false, fmt.Errorf("mark rejected: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit rejected: %w", err)
		}
		return false, nil
	}

	if sellerInstr < instrumentQty {
		reason := fmt.Sprintf("INSUFFICIENT_FUNDS: seller %s available=%d needed=%d", ev.Instrument, sellerInstr, instrumentQty)
		if err := markProcessed(ctx, tx, ev.TradeID, "REJECTED", reason); err != nil {
			return false, fmt.Errorf("mark rejected: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit rejected: %w", err)
		}
		return false, nil
	}

	// Insert ledger entries
	// Convention: amount signed (NUMERIC)
	// buyer: +instrument, -USD
	// seller: -instrument, +USD

	now := time.Now().UTC()

	if _, err := tx.Exec(ctx, `
	INSERT INTO ledger_entries (trade_id, user_id, asset, amount, created_at)
	VALUES
		($1, $2, $3, $4, $5),  -- buyer +instrument
		($1, $6, $3, $7, $5),  -- seller -instrument
		($1, $2, $8, $9, $5),  -- buyer -USD
		($1, $6, $8, $10, $5)  -- seller +USD
	`,
		ev.TradeID,
		ev.BuyerUserID, ev.Instrument, fmt.Sprintf("%d", instrumentQty), now,
		ev.SellerUserID, fmt.Sprintf("-%d", instrumentQty),
		USDAsset, fmt.Sprintf("-%d", usdValue), fmt.Sprintf("%d", usdValue),
	); err != nil {
		return false, fmt.Errorf("insert ledger: %w", err)
	}

	// Double-entry bookkeeping
	ok, err := validateZeroSum(ctx, tx, ev.TradeID, ev.Instrument)
	if err != nil {
		return false, fmt.Errorf("validate ledger: %w", err)
	}
	if !ok {
		// This indicates a bug in our logic (should never happen). Fail hard.
		return false, fmt.Errorf("double-entry validation failed for trade_id=%s", ev.TradeID)
	}

	// Update balances
	// buyer +instrument
	if err := upsertAdd(ctx, tx, ev.BuyerUserID.String(), ev.Instrument, fmt.Sprintf("%d", instrumentQty)); err != nil {
		return false, fmt.Errorf("balance buyer +instrument: %w", err)
	}
	// seller -instrument
	if err := upsertAdd(ctx, tx, ev.SellerUserID.String(), ev.Instrument, fmt.Sprintf("-%d", instrumentQty)); err != nil {
		return false, fmt.Errorf("balance seller -instrument: %w", err)
	}
	// buyer -USD
	if err := upsertAdd(ctx, tx, ev.BuyerUserID.String(), USDAsset, fmt.Sprintf("-%d", usdValue)); err != nil {
		return false, fmt.Errorf("balance buyer -USD: %w", err)
	}
	// seller +USD
	if err := upsertAdd(ctx, tx, ev.SellerUserID.String(), USDAsset, fmt.Sprintf("%d", usdValue)); err != nil {
		return false, fmt.Errorf("balance seller +USD: %w", err)
	}

	// Mark trade as processed
	if _, err := tx.Exec(ctx,
		`INSERT INTO processed_trades (trade_id, processed_at) VALUES ($1, NOW())`,
		ev.TradeID,
	); err != nil {
		return false, fmt.Errorf("insert processed_trades: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit tx: %w", err)
	}

	return true, nil
}

func lockAndGetAvailable(ctx context.Context, tx pgx.Tx, userID string, asset string) (int64, error) {
	// Ensure row exists so FOR UPDATE actually locks something
	if _, err := tx.Exec(ctx, `
		INSERT INTO balances (user_id, asset, available, locked, updated_at)
		VALUES ($1::uuid, $2, 0, 0, NOW())
		ON CONFLICT (user_id, asset) DO NOTHING
	`, userID, asset); err != nil {
		return 0, fmt.Errorf("ensure balance row: %w", err)
	}

	var v int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(available, 0)::bigint
		FROM balances
		WHERE user_id = $1::uuid AND asset = $2
		FOR UPDATE
	`, userID, asset).Scan(&v); err != nil {
		return 0, fmt.Errorf("select balance for update: %w", err)
	}

	return v, nil
}

func markProcessed(ctx context.Context, tx pgx.Tx, tradeID [16]byte, status string, reason string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO processed_trades (trade_id, status, reason, processed_at)
		 VALUES ($1, $2, $3, NOW())`,
		tradeID, status, reason,
	)
	return err
}

func validateZeroSum(ctx context.Context, tx pgx.Tx, tradeID [16]byte, instrument string) (bool, error) {
	// Check instrument sum
	var instrSumTxt string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)::text
		FROM ledger_entries
		WHERE trade_id = $1 AND asset = $2
	`, tradeID, instrument).Scan(&instrSumTxt); err != nil {
		return false, err
	}
	var instrSum int64
	if _, err := fmt.Sscanf(instrSumTxt, "%d", &instrSum); err != nil {
		return false, fmt.Errorf("parse instrument sum %q: %w", instrSumTxt, err)
	}
	if instrSum != 0 {
		return false, nil
	}

	// Check USD sum
	var usdSumTxt string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)::text
		FROM ledger_entries
		WHERE trade_id = $1 AND asset = $2
	`, tradeID, USDAsset).Scan(&usdSumTxt); err != nil {
		return false, err
	}
	var usdSum int64
	if _, err := fmt.Sscanf(usdSumTxt, "%d", &usdSum); err != nil {
		return false, fmt.Errorf("parse USD sum %q: %w", usdSumTxt, err)
	}
	if usdSum != 0 {
		return false, nil
	}

	return true, nil
}

func (s *Settler) reject(ctx context.Context, ev *events.TradeExecutedEvent, reason string) (bool, error) {
	// best-effort reject: we only can reject if trade_id exists
	if ev.TradeID == [16]byte{} {
		return false, fmt.Errorf("%s", reason)
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// if already exists, nothing to do
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM processed_trades WHERE trade_id = $1)`,
		ev.TradeID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check processed_trades: %w", err)
	}
	if !exists {
		if err := markProcessed(ctx, tx, ev.TradeID, "REJECTED", reason); err != nil {
			return false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit reject: %w", err)
	}
	return false, nil
}

func mulCheckOverflow(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	result := a * b
	if result/a != b {
		return 0, false
	}
	return result, true
}
