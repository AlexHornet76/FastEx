ALTER TABLE ledger_entries
  ADD CONSTRAINT ledger_asset_not_empty CHECK (length(asset) > 0),
  ADD CONSTRAINT ledger_amount_nonzero CHECK (amount <> 0);

-- Each trade should affect a given (user, asset) once in this simplified model:
-- buyer: instrument + USD, seller: instrument + USD
CREATE UNIQUE INDEX IF NOT EXISTS uq_ledger_trade_user_asset
  ON ledger_entries(trade_id, user_id, asset);