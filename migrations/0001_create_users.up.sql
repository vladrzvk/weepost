-- 0001_create_users.up.sql

CREATE TABLE users (
    -- Identité
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) NOT NULL,
    email_verified BOOLEAN    NOT NULL DEFAULT false,
    status        VARCHAR(20)  NOT NULL DEFAULT 'pending_verification',

    -- Authentification locale (NULL si SSO uniquement)
    -- Argon2id (OWASP 2024) : m=65536,t=1,p=4 — jamais bcrypt (U-3 Phase 2)
    -- Format : $argon2id$v=19$m=65536,t=1,p=4$<salt-b64>$<hash-b64>
    -- Géré par le Value Object Password (Phase 8 T2)
    password_hash TEXT         NULL,

    -- Profil
    first_name    VARCHAR(100) NULL,
    last_name     VARCHAR(100) NULL,
    avatar_url    TEXT         NULL,
    locale        VARCHAR(10)  NOT NULL DEFAULT 'fr',

    -- 2FA et suivi connexion (migration 0042)
    two_fa_enabled        BOOLEAN     NOT NULL DEFAULT false,
    totp_secret           TEXT,
    two_fa_backup_codes   TEXT[],
    failed_login_attempts SMALLINT    NOT NULL DEFAULT 0,
    locked_until          TIMESTAMPTZ,
    locked                BOOLEAN     NOT NULL DEFAULT false,
    last_login_at         TIMESTAMPTZ,
    last_login_ip         TEXT,

    -- Audit
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ  NULL,    -- RGPD : purge physique possible après délai légal

    -- Contraintes
    CONSTRAINT chk_users_status CHECK (
        status IN ('active', 'locked', 'suspended', 'pending_verification', 'deleted')
    ),
    CONSTRAINT chk_users_email_format CHECK (email ~* '^[^@\s]+@[^@\s]+\.[^@\s]+$')
);

-- Index unique sur email parmi les non-supprimés
-- Un email RGPD-purgé (deleted_at NOT NULL) peut être réutilisé
CREATE UNIQUE INDEX uq_users_email
    ON users(email)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_users_status
    ON users(status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_users_active
    ON users(id)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE users IS 'Aggregate Root User — authentification et profil global';
COMMENT ON COLUMN users.password_hash IS 'Argon2id — NULL si compte SSO';
COMMENT ON COLUMN users.deleted_at IS 'RGPD soft-delete — purge physique via RG-C-003 après délai légal';
