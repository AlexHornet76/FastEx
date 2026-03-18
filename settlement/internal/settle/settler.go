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
func (s *Settler) ApplyTrade(ctx context.Context, ev *events.TradeExecutedEvent) (applied bool, err error) {
	// int64 -> NUMERIC safely by passing as string
	// We'll store instrument quantity and USD value.
	instrumentQty := ev.Quantity
	usdValue := ev.Price * ev.Quantity

	// Sanity checks
	if ev.TradeID == [16]byte{} {
		return false, fmt.Errorf("missing trade_id")
	}
	if ev.Instrument == "" {
		return false, fmt.Errorf("missing instrument")
	}
	if instrumentQty <= 0 {
		return false, fmt.Errorf("invalid quantity: %d", instrumentQty)
	}
	if ev.Price <= 0 {
		return false, fmt.Errorf("invalid price: %d", ev.Price)
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

	// Insert ledger entries
	// Convention: amount signed (NUMERIC)
	// buyer: +instrument, -USD
	// seller: -instrument, +USD

	now := time.Now().UTC()

	// instrument leg
	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_entries (trade_id, user_id, asset, amount, created_at)
		VALUES
			($1, $2, $3, $4, $5),  -- buyer +instrument
			($1, $6, $3, $7, $5)   -- seller -instrument
	`,
		ev.TradeID,
		ev.BuyerUserID, ev.Instrument, fmt.Sprintf("%d", instrumentQty), now,
		ev.SellerUserID, fmt.Sprintf("-%d", instrumentQty),
	); err != nil {
		return false, fmt.Errorf("insert instrument ledger: %w", err)
	}

	// USD leg
	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_entries (trade_id, user_id, asset, amount, created_at)
		VALUES
			($1, $2, $3, $4, $5),  -- buyer -USD
			($1, $6, $3, $7, $5)   -- seller +USD
	`,
		ev.TradeID,
		ev.BuyerUserID, USDAsset, fmt.Sprintf("-%d", usdValue), now,
		ev.SellerUserID, fmt.Sprintf("%d", usdValue),
	); err != nil {
		return false, fmt.Errorf("insert USD ledger: %w", err)
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
