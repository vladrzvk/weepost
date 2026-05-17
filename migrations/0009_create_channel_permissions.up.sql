-- 0009_create_channel_permissions.up.sql

CREATE TABLE channel_permissions (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    assignment_id UUID        NOT NULL,   -- FK vers brand_assignments
    channel_id    UUID        NOT NULL,
    action        VARCHAR(50) NOT NULL,
    granted       BOOLEAN     NOT NULL DEFAULT true,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_channel_permissions_assignment
        FOREIGN KEY (assignment_id) REFERENCES brand_assignments(id) ON DELETE CASCADE,
    CONSTRAINT fk_channel_permissions_channel
        FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE,
    CONSTRAINT uq_channel_permissions
        UNIQUE (assignment_id, channel_id, action),
    CONSTRAINT chk_channel_permissions_action
        CHECK (action IN (
            'view', 'create', 'edit', 'schedule', 'publish',
            'moderate', 'reply', 'analytics', 'connect', 'disconnect'
        ))
);

CREATE UNIQUE INDEX uq_channel_permissions_idx
    ON channel_permissions(assignment_id, channel_id, action);

-- Index critique pour résolution RBAC niveau 3 (Phase 6 §2 Algorithme)
CREATE INDEX idx_channel_permissions_lookup
    ON channel_permissions(assignment_id, channel_id)
    WHERE granted = true;

COMMENT ON TABLE channel_permissions IS 'RBAC niveau 3 — permissions granulaires par channel et action';
