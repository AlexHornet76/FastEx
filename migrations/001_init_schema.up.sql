-- Sprint 1: Gateway + Authentication Schema

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

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