-- Sprint 1: Gateway + Authentication Schema

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    public_key TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Authentication challenges table
CREATE TABLE IF NOT EXISTS auth_challenges (
    challenge_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) NOT NULL,
    challenge TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_challenges_challenge ON auth_challenges(challenge);
CREATE INDEX IF NOT EXISTS idx_challenges_expires ON auth_challenges(expires_at);

-- Future Sprint 4: Accounts and balances (commented out)
-- CREATE TABLE IF NOT EXISTS accounts (
--     account_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     user_id UUID NOT NULL REFERENCES users(user_id),
--     currency VARCHAR(10) NOT NULL,
--     balance NUMERIC(20, 8) NOT NULL DEFAULT 0,
--     locked_balance NUMERIC(20, 8) NOT NULL DEFAULT 0,
--     created_at TIMESTAMP DEFAULT NOW(),
--     UNIQUE(user_id, currency)
-- );

-- Seed test users for settlement testing
INSERT INTO users (user_id, username, public_key)
VALUES
  (
    '11111111-1111-1111-1111-111111111111',
    'test_seller',
    'placeholder-public-key-seller'
  ),
  (
    '22222222-2222-2222-2222-222222222222',
    'test_buyer',
    'placeholder-public-key-buyer'
  )
ON CONFLICT (user_id) DO NOTHING;

-- Seed balances: seller holds BTC, buyer holds USD
INSERT INTO balances (user_id, asset, available, locked)
VALUES
  ('11111111-1111-1111-1111-111111111111', 'BTC', 100, 0),  -- seller has BTC to sell
  ('11111111-1111-1111-1111-111111111111', 'USD', 0,   0),
  ('22222222-2222-2222-2222-222222222222', 'USD', 200000, 0), -- buyer has USD to spend
  ('22222222-2222-2222-2222-222222222222', 'BTC', 0,   0)
ON CONFLICT (user_id, asset) DO NOTHING;