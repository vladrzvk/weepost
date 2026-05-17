-- 0000_bootstrap.down.sql
DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE;
DROP EXTENSION IF EXISTS "pg_trgm";
DROP EXTENSION IF EXISTS "pgcrypto";
