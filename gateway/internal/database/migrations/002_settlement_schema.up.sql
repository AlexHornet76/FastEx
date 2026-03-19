CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Balances per user per asset (USD + instruments like BTC/AAPL)
CREATE TABLE IF NOT EXISTS balances (
    user_id UUID NOT NULL REFERENCES users(user_id),
    asset VARCHAR(16) NOT NULL,
    available NUMERIC(30, 10) NOT NULL DEFAULT 0,
    locked NUMERIC(30, 10) NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, asset)
);

CREATE INDEX IF NOT EXISTS idx_balances_user_id ON balances(user_id);
CREATE INDEX IF NOT EXISTS idx_balances_asset ON balances(asset);

-- Ledger entries (double-entry bookkeeping)
-- amount is signed:
--  +X = credit (user receives)
--  -X = debit  (user pays)
CREATE TABLE IF NOT EXISTS ledger_entries (
    entry_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trade_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(user_id),
    asset VARCHAR(16) NOT NULL,
    amount NUMERIC(30, 10) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_trade_id ON ledger_entries(trade_id);
CREATE INDEX IF NOT EXISTS idx_ledger_user_id ON ledger_entries(user_id);
CREATE INDEX IF NOT EXISTS idx_ledger_asset ON ledger_entries(asset);

-- Idempotency: mark processed trades
CREATE TABLE IF NOT EXISTS processed_trades (
    trade_id UUID PRIMARY KEY,
    processed_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Seed balances: seller holds BTC, buyer holds USD
INSERT INTO balances (user_id, asset, available, locked)
VALUES
  ('11111111-1111-1111-1111-111111111111', 'BTC', 100, 0),  -- seller has BTC to sell
  ('11111111-1111-1111-1111-111111111111', 'USD', 0,   0),
  ('22222222-2222-2222-2222-222222222222', 'USD', 200000, 0), -- buyer has USD to spend
  ('22222222-2222-2222-2222-222222222222', 'BTC', 0,   0)
ON CONFLICT (user_id, asset) DO NOTHING;