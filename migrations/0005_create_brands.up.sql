-- 0005_create_brands.up.sql

CREATE TABLE brands (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID         NOT NULL,
    name         VARCHAR(100) NOT NULL,
    slug         VARCHAR(50)  NOT NULL,
    status       VARCHAR(20)  NOT NULL DEFAULT 'active',
    industry     VARCHAR(100) NULL,

    -- BrandIdentity (colonnes individuelles — A9-2)
    identity_logo_url        TEXT         NULL,
    identity_primary_color   VARCHAR(7)   NULL,
    identity_secondary_color VARCHAR(7)   NULL,

    -- ToneOfVoice (colonnes individuelles — A9-2)
    tone_formality      VARCHAR(20) NULL,
    tone_humor_level    VARCHAR(20) NULL,
    tone_emojis_allowed BOOLEAN     NOT NULL DEFAULT true,

    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ  NULL,

    CONSTRAINT fk_brands_workspace
        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE RESTRICT,
    CONSTRAINT chk_brands_status
        CHECK (status IN ('active', 'archived', 'deleted')),
    CONSTRAINT chk_brands_primary_color
        CHECK (identity_primary_color IS NULL OR identity_primary_color ~ '^#[0-9A-Fa-f]{6}$'),
    CONSTRAINT chk_brands_secondary_color
        CHECK (identity_secondary_color IS NULL OR identity_secondary_color ~ '^#[0-9A-Fa-f]{6}$'),
    CONSTRAINT chk_brands_formality
        CHECK (tone_formality IS NULL OR tone_formality IN ('formal', 'neutral', 'casual')),
    CONSTRAINT chk_brands_humor
        CHECK (tone_humor_level IS NULL OR tone_humor_level IN ('none', 'light', 'moderate', 'heavy'))
);

-- Slug unique par workspace (pas global)
CREATE UNIQUE INDEX uq_brands_slug
    ON brands(workspace_id, slug)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_brands_workspace
    ON brands(workspace_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_brands_status
    ON brands(workspace_id, status)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_brands_updated_at
    BEFORE UPDATE ON brands
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE brands IS 'Aggregate Root Brand — entité éditoriale SM-08';
