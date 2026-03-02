-- Rollback Sprint 1 schema

DROP INDEX IF EXISTS idx_challenges_expires;
DROP INDEX IF EXISTS idx_challenges_challenge;
DROP TABLE IF EXISTS auth_challenges;

DROP INDEX IF EXISTS idx_users_username;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS "uuid-ossp";