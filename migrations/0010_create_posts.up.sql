-- 0010_create_posts.up.sql

CREATE TABLE posts (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    brand_id         UUID         NOT NULL,
    workspace_id     UUID         NOT NULL,   -- Dénormalisé pour perf
    created_by_user_id UUID        NOT NULL,
    title            VARCHAR(300) NOT NULL,
    main_caption     TEXT         NOT NULL,   -- Caption principal
    status           VARCHAR(30)  NOT NULL DEFAULT 'draft',

    -- D-05 Phase 5 : limite à 3 tentatives de publication
    -- Sans cette colonne, le worker PB-C-011 RetryFailedPost bouclerait infiniment
    retry_count      SMALLINT     NOT NULL DEFAULT 0,

    -- Planification
    scheduled_at     TIMESTAMPTZ  NULL,       -- Non-null si status = 'scheduled'
    published_at     TIMESTAMPTZ  NULL,       -- Non-null si status = 'published'
    publish_type     VARCHAR(20)  NOT NULL DEFAULT 'immediate',

    -- Workflow approbation
    rejection_reason TEXT         NULL,       -- D-06 Phase 5 : obligatoire si transition validated→rejected

    -- Stratégie éditoriale (optionnel)
    content_pillar_id UUID        NULL,

    version          INTEGER      NOT NULL DEFAULT 1,
    -- Optimistic locking : incrémenté à chaque mutation d'état.
    -- UPDATE posts SET ... WHERE id=$1 AND version=$2
    -- Si 0 rows affected → ErrCodeCONCURRENCY_CONFLICT

    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ  NULL,

    CONSTRAINT fk_posts_brand
        FOREIGN KEY (brand_id) REFERENCES brands(id) ON DELETE RESTRICT,
    CONSTRAINT fk_posts_workspace
        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE RESTRICT,
    CONSTRAINT fk_posts_author
        FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT chk_posts_status
        CHECK (status IN (
            'draft', 'pending_validation', 'validated',
            'rejected', 'scheduled', 'published', 'failed'
        )),
    CONSTRAINT chk_posts_publish_type
        CHECK (publish_type IN ('immediate', 'scheduled')),
    -- D-05 Phase 5 : le compteur de retry est borné à 3
    CONSTRAINT chk_posts_retry
        CHECK (retry_count >= 0 AND retry_count <= 3),
    -- Cohérence status/scheduled_at
    CONSTRAINT chk_posts_scheduled_at
        CHECK (
            (status = 'scheduled' AND scheduled_at IS NOT NULL) OR
            (status != 'scheduled')
        ),
    -- Cohérence status/published_at
    CONSTRAINT chk_posts_published_at
        CHECK (
            (status = 'published' AND published_at IS NOT NULL) OR
            (status != 'published')
        )
);

CREATE INDEX idx_posts_brand
    ON posts(brand_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_posts_workspace
    ON posts(workspace_id)
    WHERE deleted_at IS NULL;

-- Index composite pour listing par brand + status (UC PB-Q-001)
CREATE INDEX idx_posts_status
    ON posts(brand_id, status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_posts_author
    ON posts(created_by_user_id)
    WHERE deleted_at IS NULL;

-- Index Worker CRON : posts planifiés à publier
CREATE INDEX idx_posts_scheduled
    ON posts(scheduled_at)
    WHERE status = 'scheduled' AND deleted_at IS NULL;

-- Index Worker retry (PB-C-011 RetryFailedPost)
CREATE INDEX idx_posts_failed_retryable
    ON posts(id)
    WHERE status = 'failed' AND retry_count < 3 AND deleted_at IS NULL;

CREATE TRIGGER trg_posts_updated_at
    BEFORE UPDATE ON posts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE posts IS 'Aggregate Root Post — SM-01 PostStatus CRITIQUE';
COMMENT ON COLUMN posts.retry_count IS 'D-05 Phase 5 — limite 3 tentatives — sans cette colonne le worker boucle';
COMMENT ON COLUMN posts.rejection_reason IS 'D-06 Phase 5 — obligatoire si transition validated→rejected';
