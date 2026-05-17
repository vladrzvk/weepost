-- 0017_create_user_sessions.up.sql

CREATE TABLE IF NOT EXISTS user_sessions (
    id          UUID        PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    jti         VARCHAR(36) UNIQUE,
    status      VARCHAR(20) NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'revoked', 'expired')),
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE user_sessions IS
    'Tracks active and historical authentication sessions for users. '
    'Each row corresponds to one issued JWT; the jti column mirrors the JWT ID '
    'claim for fast token lookup and revocation.';

COMMENT ON COLUMN user_sessions.jti        IS 'JWT ID claim — used by GetByJTI to validate bearer tokens.';
COMMENT ON COLUMN user_sessions.status     IS 'Lifecycle state: active | revoked (manual) | expired (TTL elapsed).';
COMMENT ON COLUMN user_sessions.expires_at IS 'Hard expiry enforced at the application layer.';

-- Fast lookup for the active sessions of a given user (dashboard, revocation list).
CREATE INDEX IF NOT EXISTS idx_user_sessions_user
    ON user_sessions (user_id)
    WHERE status = 'active';

-- Fast O(1) token validation path used on every authenticated request.
CREATE INDEX IF NOT EXISTS idx_user_sessions_jti
    ON user_sessions (jti)
    WHERE jti IS NOT NULL;
