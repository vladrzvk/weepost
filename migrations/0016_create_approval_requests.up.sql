-- 0016_create_approval_requests.up.sql

CREATE TABLE approval_requests (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id                  UUID        NOT NULL,
    type                     VARCHAR(20) NOT NULL,
    status                   VARCHAR(20) NOT NULL DEFAULT 'pending',
    requested_by_member_id   UUID        NOT NULL,
    approver_member_id       UUID        NULL,   -- Approbateur interne
    approver_guest_id        UUID        NULL,   -- Approbateur externe (brand_guests)
    reviewed_at              TIMESTAMPTZ NULL,
    cancelled_at             TIMESTAMPTZ NULL,
    -- NULL tant que la demande n'est pas annulée.
    -- Défini par T19 CancelApprovalRequestService.
    -- SET quand status → 'cancelled'.
    rejection_reason         TEXT        NULL,

    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_approval_requests_post
        FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE RESTRICT,
    CONSTRAINT fk_approval_requests_requester
        FOREIGN KEY (requested_by_member_id) REFERENCES workspace_members(id) ON DELETE RESTRICT,
    CONSTRAINT fk_approval_requests_approver_member
        FOREIGN KEY (approver_member_id) REFERENCES workspace_members(id) ON DELETE RESTRICT,
    CONSTRAINT chk_approval_type
        CHECK (type IN ('internal', 'external')),
    CONSTRAINT chk_approval_status
        CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    -- D-03 Phase 5 : une seule demande en pending par post et type
    -- DEFERRABLE pour permettre cancel + create dans la même transaction
    CONSTRAINT uq_approval_active
        UNIQUE (post_id, type, status)
        DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX idx_approval_requests_post
    ON approval_requests(post_id, status);

CREATE INDEX idx_approval_requests_approver
    ON approval_requests(approver_member_id, status)
    WHERE approver_member_id IS NOT NULL;

CREATE INDEX idx_approval_requests_guest
    ON approval_requests(approver_guest_id, status)
    WHERE approver_guest_id IS NOT NULL;

-- Index pour listing des demandes en attente (Inbox approbateur)
CREATE INDEX idx_approval_requests_pending
    ON approval_requests(post_id)
    WHERE status = 'pending';

COMMENT ON TABLE approval_requests IS 'Workflow approbation — SM-10 ApprovalStatus — D-03 Phase 5';
COMMENT ON COLUMN approval_requests.approver_guest_id IS 'NULL si approbateur interne — FK vers brand_guests ajoutée en migration 0019';
