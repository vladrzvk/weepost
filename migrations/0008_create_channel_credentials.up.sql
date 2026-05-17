-- 0008_create_channel_credentials.up.sql

CREATE TABLE channel_credentials (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id        UUID        NOT NULL UNIQUE,   -- 1 credential par channel
    access_token_enc  TEXT        NOT NULL,    -- AES-256-GCM base64 — Invariant C-3
    refresh_token_enc TEXT        NULL,        -- AES-256-GCM base64 — NULL si non supporté
    token_type        VARCHAR(20) NOT NULL DEFAULT 'Bearer',
    scope             TEXT        NULL,        -- Scopes OAuth accordés
    expires_at        TIMESTAMPTZ NOT NULL,    -- Expiration token (miroir dans channels.token_expires_at)

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- ON DELETE CASCADE : si le channel est détruit, ses credentials le sont aussi
    -- Pas de soft-delete sur les credentials — données sensibles chiffrées
    CONSTRAINT fk_channel_credentials_channel
        FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_channel_credentials_channel
    ON channel_credentials(channel_id);

-- Index pour le CRON de rotation (SC-C-014)
CREATE INDEX idx_channel_credentials_expiry
    ON channel_credentials(expires_at);

COMMENT ON TABLE channel_credentials IS 'OAuthTokens chiffrés — Invariant C-3 — ex brand_channel_credentials (A9-1)';
COMMENT ON COLUMN channel_credentials.access_token_enc IS 'AES-256-GCM base64 — Phase 8 T18 AESCryptoService';
COMMENT ON COLUMN channel_credentials.refresh_token_enc IS 'AES-256-GCM base64 — NULL si la plateforme ne supporte pas le refresh';
-- Pas de deleted_at : suppression physique par CASCADE depuis channels
