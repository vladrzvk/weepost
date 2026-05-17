-- 0007_create_channels.up.sql

CREATE TABLE channels (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    brand_id      UUID         NOT NULL,
    workspace_id  UUID         NOT NULL,   -- Dénormalisé depuis brands.workspace_id pour perf RBAC
    type          VARCHAR(30)  NOT NULL,
    status        VARCHAR(30)  NOT NULL DEFAULT 'active',
    external_id   VARCHAR(255) NOT NULL,   -- ID page/compte sur la plateforme sociale
    display_name  VARCHAR(255) NOT NULL,

    -- State Machine D-01 (Phase 5) : compteur d'échecs consécutifs
    consecutive_failures SMALLINT    NOT NULL DEFAULT 0,
    last_failure_at      TIMESTAMPTZ NULL,

    -- Dénormalisé depuis channel_credentials.expires_at pour le CRON CH-C-005
    token_expires_at     TIMESTAMPTZ NULL,

    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ  NULL,

    CONSTRAINT fk_channels_brand
        FOREIGN KEY (brand_id) REFERENCES brands(id) ON DELETE RESTRICT,
    CONSTRAINT fk_channels_workspace
        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE RESTRICT,
    CONSTRAINT uq_channels_external
        UNIQUE (brand_id, type, external_id),
    CONSTRAINT chk_channels_type
        CHECK (type IN (
            'facebook_page',
            'instagram_business',
            'linkedin_page',
            'twitter_account'  -- reserved, not yet implemented
        )),
    CONSTRAINT chk_channels_status
        CHECK (status IN (
            'active',
            'disconnected',
            'token_expired',
            'error',
            'pending_review',
            'revoked',
            'token_expiring'
        )),
    CONSTRAINT chk_channels_failures
        CHECK (consecutive_failures >= 0)
);

CREATE INDEX idx_channels_brand
    ON channels(brand_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_channels_workspace
    ON channels(workspace_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_channels_status
    ON channels(status)
    WHERE deleted_at IS NULL;

-- Index CRON CH-C-005 : canaux avec token expirant dans les 24h
CREATE INDEX idx_channels_token_expiry
    ON channels(token_expires_at)
    WHERE status = 'active' AND deleted_at IS NULL;

-- Index CRON CH-C-006 : canaux en erreur à surveiller
CREATE INDEX idx_channels_errors
    ON channels(consecutive_failures)
    WHERE consecutive_failures > 0 AND deleted_at IS NULL;

CREATE TRIGGER trg_channels_updated_at
    BEFORE UPDATE ON channels
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE channels IS 'Aggregate Root Channel — SM-04 ChannelStatus — ex brand_channels (A9-1)';
COMMENT ON COLUMN channels.workspace_id IS 'Dénormalisé depuis brands.workspace_id — évite JOIN dans queries RBAC';
COMMENT ON COLUMN channels.consecutive_failures IS 'D-01 Phase 5 — seuil 3 : transition vers status=error';
COMMENT ON COLUMN channels.token_expires_at IS 'Dénormalisé depuis channel_credentials — pour index CRON CH-C-005';
