-- 0014_create_publication_logs.up.sql

CREATE TABLE publication_logs (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    post_variant_id  UUID        NOT NULL,
    channel_id       UUID        NOT NULL,
    status           VARCHAR(20) NOT NULL,
    attempt_number   SMALLINT    NOT NULL DEFAULT 1,
    platform_response JSONB      NULL,   -- Réponse brute — cas JSONB légitime (A9-2)
    error_code       VARCHAR(100) NULL,
    error_message    TEXT         NULL,
    duration_ms      INTEGER      NULL,

    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- Immutable : pas de updated_at, pas de deleted_at, pas de trigger

    CONSTRAINT fk_publication_logs_variant
        FOREIGN KEY (post_variant_id) REFERENCES post_variants(id) ON DELETE RESTRICT,
    CONSTRAINT fk_publication_logs_channel
        FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE RESTRICT,
    CONSTRAINT chk_publication_logs_status
        CHECK (status IN ('success', 'failure', 'retry')),
    CONSTRAINT chk_publication_logs_attempt
        CHECK (attempt_number >= 1 AND attempt_number <= 3)
);

CREATE INDEX idx_publication_logs_variant
    ON publication_logs(post_variant_id);

CREATE INDEX idx_publication_logs_channel
    ON publication_logs(channel_id, created_at DESC);

CREATE INDEX idx_publication_logs_status
    ON publication_logs(status, created_at DESC);

COMMENT ON TABLE publication_logs IS 'Log immutable tentatives publication — pas de updated_at ni deleted_at';
COMMENT ON COLUMN publication_logs.platform_response IS 'JSONB réponse brute plateforme — cas légitime A9-2';
