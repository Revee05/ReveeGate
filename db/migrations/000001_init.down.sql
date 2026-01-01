-- migrations/000001_init.down.sql
-- Rollback ReveeGate Database Schema

-- Drop triggers first
DROP TRIGGER IF EXISTS update_donations_updated_at ON donations;
DROP TRIGGER IF EXISTS update_payments_updated_at ON payments;
DROP TRIGGER IF EXISTS update_admin_users_updated_at ON admin_users;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order of creation (respecting foreign keys)
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS overlay_tokens;
DROP TABLE IF EXISTS admin_users;
DROP TABLE IF EXISTS webhook_logs;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS donations;

-- Drop extension (optional, might be used by other databases)
-- DROP EXTENSION IF EXISTS "uuid-ossp";
