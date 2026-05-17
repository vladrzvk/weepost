-- 0015_create_publish_jobs.up.sql

CREATE TABLE publish_jobs (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    post_variant_id   UUID         NOT NULL,
    idempotency_key   VARCHAR(255) NOT NULL UNIQUE,   -- Format : variant_id:attempt_number
    status            VARCHAR(20)  NOT NULL DEFAULT 'pending',
    scheduled_for     TIMESTAMPTZ  NOT NULL,
    attempt_count     SMALLINT     NOT NULL DEFAULT 0,
    last_attempted_at TIMESTAMPTZ  NULL,
    next_retry_at     TIMESTAMPTZ  NULL,

    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_publish_jobs_variant
        FOREIGN KEY (post_variant_id) REFERENCES post_variants(id) ON DELETE CASCADE,
    CONSTRAINT chk_publish_jobs_status
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')),
    CONSTRAINT chk_publish_jobs_attempts
        CHECK (attempt_count >= 0)
);

CREATE UNIQUE INDEX uq_publish_jobs_idempotency
    ON publish_jobs(idempotency_key);

-- Index Worker : jobs à traiter maintenant
CREATE INDEX idx_publish_jobs_pending
    ON publish_jobs(scheduled_for)
    WHERE status = 'pending';

CREATE INDEX idx_publish_jobs_variant
    ON publish_jobs(post_variant_id);

CREATE TRIGGER trg_publish_jobs_updated_at
    BEFORE UPDATE ON publish_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE publish_jobs IS 'File de publication persistante — PB-C-010 PublishPostAsync — NOUVEAU IC9-5 (C9-8)';
COMMENT ON COLUMN publish_jobs.idempotency_key IS 'Format : {post_variant_id}:{attempt_number} — garantit pas de double publication';
-- V0 : le worker Redis (Phase 8 T17) utilise idempotency_key pour éviter les doublons.
-- Post-V0 : peut servir de fallback si Redis est indisponible.
