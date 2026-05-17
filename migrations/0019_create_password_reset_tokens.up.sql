-- 0019_create_password_reset_tokens.up.sql

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id         UUID         PRIMARY KEY,
    user_id    UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      VARCHAR(128) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ  NOT NULL,
    used_at    TIMESTAMPTZ  NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE password_reset_tokens IS
    'Single-use tokens issued during the "forgot password" flow. '
    'A token is considered consumed once used_at is set; the application must '
    'additionally verify that expires_at has not elapsed. '
    'Rows may be purged periodically once used_at IS NOT NULL or expires_at < NOW().';

COMMENT ON COLUMN password_reset_tokens.token      IS 'Cryptographically random opaque token (minimum 128 bits of entropy) sent to the user via email.';
COMMENT ON COLUMN password_reset_tokens.expires_at IS 'Hard TTL — application rejects the token after this timestamp even if used_at is NULL.';
COMMENT ON COLUMN password_reset_tokens.used_at    IS 'Set when the token is successfully consumed; NULL means the token has not been used yet.';

-- Primary lookup path: validate an incoming token from the reset URL.
-- Partial index excludes already-consumed tokens so the hot path stays tiny.
CREATE INDEX IF NOT EXISTS idx_pwd_reset_token
    ON password_reset_tokens (token)
    WHERE used_at IS NULL;

-- List or revoke all outstanding reset requests for a given user.
CREATE INDEX IF NOT EXISTS idx_pwd_reset_user
    ON password_reset_tokens (user_id);
