-- 0003_create_workspace_members.up.sql

CREATE TABLE workspace_members (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id         UUID        NOT NULL,
    user_id              UUID        NOT NULL,
    role                 VARCHAR(20) NOT NULL,
    status               VARCHAR(20) NOT NULL DEFAULT 'active',
    invited_by_member_id UUID        NULL,       -- Self-référence : NULL pour le founder

    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ NULL,

    CONSTRAINT fk_workspace_members_workspace
        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE RESTRICT,
    CONSTRAINT fk_workspace_members_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_workspace_members_inviter
        FOREIGN KEY (invited_by_member_id) REFERENCES workspace_members(id) ON DELETE RESTRICT,
    CONSTRAINT chk_workspace_members_role
        CHECK (role IN ('owner', 'admin', 'manager', 'editor', 'viewer')),
    CONSTRAINT chk_workspace_members_status
        CHECK (status IN ('invited', 'active', 'suspended'))   -- SM-04 : 3 états canoniques
);

-- Un user ne peut avoir qu'un seul membership actif par workspace
CREATE UNIQUE INDEX uq_workspace_members
    ON workspace_members(workspace_id, user_id)
    WHERE deleted_at IS NULL;

-- Index RBAC niveau 1 — critique pour résolution permissions (Phase 6 §2 Algorithme)
CREATE INDEX idx_workspace_members_workspace
    ON workspace_members(workspace_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_workspace_members_user
    ON workspace_members(user_id)
    WHERE deleted_at IS NULL;

-- Index composite pour vérification rapide du rôle (IsOwner, IsAdmin)
CREATE INDEX idx_workspace_members_role
    ON workspace_members(workspace_id, role)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_workspace_members_updated_at
    BEFORE UPDATE ON workspace_members
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE workspace_members IS 'RBAC niveau 1 — appartenance et rôle workspace';
COMMENT ON COLUMN workspace_members.role IS 'MemberRole : owner|admin|manager|editor|viewer';
