-- 0002_create_workspaces.up.sql

CREATE TABLE workspaces (
    -- Identité
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          VARCHAR(50)  NOT NULL UNIQUE,   -- Immutable après création (Invariant W-2 Phase 2)
    name          VARCHAR(100) NOT NULL,
    owner_user_id UUID         NOT NULL,

    -- État
    status        VARCHAR(30)  NOT NULL DEFAULT 'active',
    current_mode  VARCHAR(20)  NOT NULL DEFAULT 'simple',  -- Calculé auto — A.1 Phase 1 P3d
    plan_id       VARCHAR(50)  NOT NULL DEFAULT 'free',

    -- WorkspaceSettings (colonnes individuelles — A9-2 : queryable, indexable, CHECK SQL)
    settings_language              VARCHAR(10)  NOT NULL DEFAULT 'fr',
    settings_timezone              VARCHAR(50)  NOT NULL DEFAULT 'Europe/Paris',
    settings_notifications_enabled BOOLEAN      NOT NULL DEFAULT true,
    settings_require_approval      BOOLEAN      NOT NULL DEFAULT false,

    -- Limites plan (dénormalisées depuis plan — mise à jour lors d'un upgrade)
    limits_max_members   SMALLINT NOT NULL DEFAULT 1,
    limits_max_brands    SMALLINT NOT NULL DEFAULT 1,
    limits_max_channels  SMALLINT NOT NULL DEFAULT 3,

    -- Audit
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ  NULL,

    -- Contraintes
    CONSTRAINT fk_workspaces_owner
        FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT chk_workspaces_slug_format
        CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$'),
    CONSTRAINT chk_workspaces_name_length
        CHECK (char_length(name) >= 3),
    CONSTRAINT chk_workspaces_status
        CHECK (status IN ('active', 'suspended', 'pending_deletion', 'deleted')),
    CONSTRAINT chk_workspaces_mode
        CHECK (current_mode IN ('simple', 'team', 'agency')),
    CONSTRAINT chk_workspaces_plan
        CHECK (plan_id IN ('free', 'starter', 'team', 'agency')),
    CONSTRAINT chk_workspaces_limits
        CHECK (limits_max_members > 0 AND limits_max_brands > 0 AND limits_max_channels > 0)
);

-- slug est immutable — l'index UNIQUE global (pas partiel) est intentionnel :
-- un slug soft-deleted ne peut pas être réutilisé (Invariant W-2 Phase 2)
CREATE INDEX idx_workspaces_owner
    ON workspaces(owner_user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_workspaces_status
    ON workspaces(status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_workspaces_plan
    ON workspaces(plan_id);

CREATE INDEX idx_workspaces_active
    ON workspaces(id)
    WHERE status = 'active' AND deleted_at IS NULL;

CREATE TRIGGER trg_workspaces_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE workspaces IS 'Aggregate Root Workspace — unité isolation multi-tenant principale';
COMMENT ON COLUMN workspaces.slug IS 'Identifiant URL unique global — immutable après création (Invariant W-2)';
COMMENT ON COLUMN workspaces.current_mode IS 'Évolue automatiquement simple→team→agency selon usage';
-- A9-2 : settings en colonnes individuelles (pas JSONB) pour queryability et CHECK SQL
