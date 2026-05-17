-- 0000_bootstrap.up.sql
-- Extensions PostgreSQL requises
CREATE EXTENSION IF NOT EXISTS "pgcrypto";   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "pg_trgm";   -- Recherche ILIKE optimisée

-- Fonction trigger update_updated_at_column() — Pattern [TRIGGER_UPD]
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Post-V0: add outbox table for transactional outbox pattern
-- See Phase 9 §IC9-7 and Phase 3 §1.3

COMMENT ON SCHEMA public IS 'WeePost V0 — Schema initial — Avril 2026';
