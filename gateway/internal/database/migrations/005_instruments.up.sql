CREATE TABLE IF NOT EXISTS instruments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(10) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    base_asset VARCHAR(10) NOT NULL,
    quote_asset VARCHAR(10) NOT NULL DEFAULT 'USD',
    min_order_size NUMERIC(20,8) NOT NULL DEFAULT 0.00000001,
    price_precision INT NOT NULL DEFAULT 2,
    quantity_precision INT NOT NULL DEFAULT 8,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert default instruments
INSERT INTO instruments (symbol, name, base_asset, quote_asset, price_precision, quantity_precision)
VALUES
    ('BTC', 'Bitcoin', 'BTC', 'USD', 2, 8),
    ('ETH', 'Ethereum', 'ETH', 'USD', 2, 8),
    ('AAPL', 'Apple Inc', 'AAPL', 'USD', 2, 4),
    ('GOOGL', 'Alphabet', 'GOOGL', 'USD', 2, 4),
    ('TSLA', 'Tesla', 'TSLA', 'USD', 2, 4)
ON CONFLICT (symbol) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_instruments_symbol ON instruments(symbol);