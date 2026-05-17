-- 0011_create_post_variants.up.sql

CREATE TABLE post_variants (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id          UUID         NOT NULL,
    channel_id       UUID         NOT NULL,
    content          TEXT         NULL,        -- Override du contenu principal (optionnel)
    status           VARCHAR(30)  NOT NULL DEFAULT 'draft',
    platform_post_id VARCHAR(255) NULL,        -- ID sur la plateforme après publication
    platform_data    JSONB        NULL,        -- Réponse brute — seul cas JSONB légitime (A9-2)
    platform_url     TEXT         NULL,        -- URL du post publié sur la plateforme
    failed_reason    TEXT         NULL,        -- Message d'erreur si status = 'failed'
    retry_count      SMALLINT     NOT NULL DEFAULT 0,  -- Tentatives de publication (max 3)
    published_at     TIMESTAMPTZ  NULL,

    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_post_variants_post
        FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    CONSTRAINT fk_post_variants_channel
        FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE RESTRICT,
    CONSTRAINT uq_post_variants_channel
        UNIQUE (post_id, channel_id),
    CONSTRAINT chk_post_variants_status
        CHECK (status IN ('draft', 'scheduled', 'published', 'failed', 'cancelled'))
);

CREATE INDEX idx_post_variants_post
    ON post_variants(post_id);

CREATE INDEX idx_post_variants_channel
    ON post_variants(channel_id, status);

-- Index Worker async : variantes à publier
CREATE INDEX idx_post_variants_pending
    ON post_variants(post_id)
    WHERE status = 'scheduled';

COMMENT ON TABLE post_variants IS 'Adaptation du post pour un channel spécifique';
COMMENT ON COLUMN post_variants.platform_data IS 'JSONB réponse brute plateforme — cas légitime A9-2';
