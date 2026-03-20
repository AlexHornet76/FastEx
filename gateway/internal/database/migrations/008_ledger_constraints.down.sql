DROP INDEX IF EXISTS uq_ledger_trade_user_asset;

ALTER TABLE ledger_entries
  DROP CONSTRAINT IF EXISTS ledger_amount_nonzero,
  DROP CONSTRAINT IF EXISTS ledger_asset_not_empty;