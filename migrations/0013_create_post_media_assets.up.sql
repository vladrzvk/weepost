-- 0013_create_post_media_assets.up.sql

CREATE TABLE post_media_assets (
    post_id        UUID     NOT NULL,
    media_asset_id UUID     NOT NULL,
    display_order  SMALLINT NOT NULL DEFAULT 0,   -- Ordre d'affichage dans le post
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (post_id, media_asset_id),

    CONSTRAINT fk_post_media_post
        FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    CONSTRAINT fk_post_media_asset
        FOREIGN KEY (media_asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT,
    CONSTRAINT chk_post_media_order
        CHECK (display_order >= 0)
);

CREATE INDEX idx_post_media_post
    ON post_media_assets(post_id);

CREATE INDEX idx_post_media_asset
    ON post_media_assets(media_asset_id);

COMMENT ON TABLE post_media_assets IS 'M:N posts ↔ media_assets — NOUVEAU IC9-5 (absent plan-complet initial)';
