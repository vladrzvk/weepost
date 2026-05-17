-- 0018_create_audit_logs.up.sql

CREATE TABLE IF NOT EXISTS audit_logs (
    id            UUID         PRIMARY KEY,
    workspace_id  UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_id      UUID         NOT NULL,
    actor_type    VARCHAR(20)  NOT NULL DEFAULT 'member'
                      CHECK (actor_type IN ('member', 'system', 'guest')),
    action        VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50)  NOT NULL,
    resource_id   UUID         NOT NULL,
    metadata      JSONB        NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE audit_logs IS
    'Immutable, append-only record of every significant action taken within a workspace. '
    'Rows are never updated or soft-deleted; retention is managed externally (archival / partitioning). '
    'actor_id is intentionally unkeyed so system and guest actors do not require a row in users.';

COMMENT ON COLUMN audit_logs.actor_type    IS 'Origin of the action: member (human user), system (background job), guest (unauthenticated or limited access).';
COMMENT ON COLUMN audit_logs.action        IS 'Verb describing what happened, e.g. "post.created", "member.invited", "channel.deleted".';
COMMENT ON COLUMN audit_logs.resource_type IS 'Domain entity affected, e.g. "post", "channel", "workspace_member".';
COMMENT ON COLUMN audit_logs.resource_id   IS 'UUID of the specific resource instance affected.';
COMMENT ON COLUMN audit_logs.metadata      IS 'Arbitrary structured payload (before/after snapshots, IP, user-agent, etc.).';

-- Primary read pattern: chronological audit trail for a workspace.
CREATE INDEX IF NOT EXISTS idx_audit_logs_workspace
    ON audit_logs (workspace_id, created_at DESC);

-- Lookup of all actions performed by a specific actor across workspaces.
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor
    ON audit_logs (actor_id);

-- Lookup of the full history of a specific resource instance.
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource
    ON audit_logs (resource_type, resource_id);
