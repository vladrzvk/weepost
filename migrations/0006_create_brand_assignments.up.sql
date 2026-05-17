-- 0006_create_brand_assignments.up.sql

CREATE TABLE brand_assignments (
    id                     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    brand_id               UUID        NOT NULL,
    member_id              UUID        NOT NULL,
    role                   VARCHAR(30) NOT NULL,
    assigned_by_member_id  UUID        NOT NULL,

    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_brand_assignments_brand
        FOREIGN KEY (brand_id) REFERENCES brands(id) ON DELETE RESTRICT,
    CONSTRAINT fk_brand_assignments_member
        FOREIGN KEY (member_id) REFERENCES workspace_members(id) ON DELETE RESTRICT,
    CONSTRAINT fk_brand_assignments_assigner
        FOREIGN KEY (assigned_by_member_id) REFERENCES workspace_members(id) ON DELETE RESTRICT,
    CONSTRAINT uq_brand_assignments
        UNIQUE (brand_id, member_id),
    CONSTRAINT chk_brand_assignments_role
        CHECK (role IN ('brand_owner', 'brand_manager', 'brand_editor', 'brand_viewer'))
);

-- Un seul assignment par (brand, member)
CREATE UNIQUE INDEX uq_brand_assignments_idx
    ON brand_assignments(brand_id, member_id);

CREATE INDEX idx_brand_assignments_member
    ON brand_assignments(member_id);

CREATE INDEX idx_brand_assignments_brand
    ON brand_assignments(brand_id);

-- Index composite critique pour résolution permission RBAC niveau 2 (Phase 6 §2)
CREATE INDEX idx_brand_assignments_perm
    ON brand_assignments(member_id, brand_id, role);

COMMENT ON TABLE brand_assignments IS 'RBAC niveau 2 — accès member à une brand spécifique';
COMMENT ON COLUMN brand_assignments.role IS 'BrandRole : brand_owner|brand_manager|brand_editor|brand_viewer';
-- Pas de deleted_at : suppression physique. Un soft-delete de workspace_members
-- rend les assignments inactives via JOIN (member.deleted_at IS NOT NULL).
