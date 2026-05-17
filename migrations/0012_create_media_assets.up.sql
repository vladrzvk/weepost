-- 0012_create_media_assets.up.sql

CREATE TABLE media_assets (
    id                       UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id             UUID         NOT NULL,
    uploaded_by_member_id    UUID         NOT NULL,
    type                     VARCHAR(20)  NOT NULL,
    status                   VARCHAR(20)  NOT NULL DEFAULT 'pending_scan',
    original_filename        VARCHAR(500) NOT NULL,
    storage_key              TEXT         NOT NULL UNIQUE,  -- Chemin S3/GCS
    mime_type                VARCHAR(100) NOT NULL,
    size_bytes               BIGINT       NOT NULL,
    width_px                 INTEGER      NULL,   -- Images et vidéos
    height_px                INTEGER      NULL,
    duration_seconds         INTEGER      NULL,   -- Vidéos et audio
    alt_text                 TEXT         NULL,

    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at               TIMESTAMPTZ  NULL,

    CONSTRAINT fk_media_assets_workspace
        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE RESTRICT,
    CONSTRAINT fk_media_assets_uploader
        FOREIGN KEY (uploaded_by_member_id) REFERENCES workspace_members(id) ON DELETE RESTRICT,
    CONSTRAINT chk_media_assets_type
        CHECK (type IN ('image', 'video', 'gif', 'audio', 'document')),
    CONSTRAINT chk_media_assets_status
        CHECK (status IN ('pending_scan', 'processing', 'ready', 'failed', 'quarantined', 'deleted')),
    CONSTRAINT chk_media_assets_size
        CHECK (size_bytes > 0)
);

CREATE INDEX idx_media_assets_workspace
    ON media_assets(workspace_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_media_assets_type
    ON media_assets(workspace_id, type)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_media_assets_status
    ON media_assets(status)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_media_assets_updated_at
    BEFORE UPDATE ON media_assets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE media_assets IS 'Aggregate Root MediaAsset — SM-09 MediaStatus';
