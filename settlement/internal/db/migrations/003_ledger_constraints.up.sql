DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'ledger_asset_not_empty'
          AND table_name      = 'ledger_entries'
    ) THEN
        ALTER TABLE ledger_entries
            ADD CONSTRAINT ledger_asset_not_empty CHECK (length(asset) > 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'ledger_amount_nonzero'
          AND table_name      = 'ledger_entries'
    ) THEN
        ALTER TABLE ledger_entries
            ADD CONSTRAINT ledger_amount_nonzero CHECK (amount <> 0);
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ledger_trade_user_asset
    ON ledger_entries(trade_id, user_id, asset);
