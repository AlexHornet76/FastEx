ALTER TABLE processed_trades
ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'APPLIED',
ADD COLUMN IF NOT EXISTS reason TEXT;

CREATE INDEX IF NOT EXISTS idx_processed_trades_status ON processed_trades(status);