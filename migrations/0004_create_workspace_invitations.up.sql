-- 0004_create_workspace_invitations.up.sql

CREATE TABLE workspace_invitations (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID         NOT NULL,
    inviter_member_id   UUID         NOT NULL,
    email               VARCHAR(255) NOT NULL,
    role                VARCHAR(20)  NOT NULL,   -- Rôle proposé
    status              VARCHAR(20)  NOT NULL DEFAULT 'pending',
    token               VARCHAR(128) NOT NULL UNIQUE,  -- SHA-256 — validation lien email
    expires_at          TIMESTAMPTZ  NOT NULL,
    accepted_at         TIMESTAMPTZ  NULL,
    cancelled_at        TIMESTAMPTZ  NULL,   -- C4-02 : NULL si non annulée

    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_invitations_workspace
        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE RESTRICT,
    CONSTRAINT fk_invitations_inviter
        FOREIGN KEY (inviter_member_id) REFERENCES workspace_members(id) ON DELETE RESTRICT,
    CONSTRAINT chk_invitations_role
        CHECK (role IN ('admin', 'manager', 'editor', 'viewer')),
    CONSTRAINT chk_invitations_status
        CHECK (status IN ('pending', 'accepted', 'expired', 'cancelled'))
);

CREATE UNIQUE INDEX uq_invitations_token
    ON workspace_invitations(token);

CREATE INDEX idx_invitations_workspace
    ON workspace_invitations(workspace_id, status);

CREATE INDEX idx_invitations_email
    ON workspace_invitations(email, status);

-- Index pour le CRON d'expiration des invitations
CREATE INDEX idx_invitations_expiry
    ON workspace_invitations(expires_at)
    WHERE status = 'pending';

COMMENT ON TABLE workspace_invitations IS 'Invitations workspace — SM-06 InvitationStatus';
COMMENT ON COLUMN workspace_invitations.token IS 'SHA-256 du token email — jamais le token brut';
