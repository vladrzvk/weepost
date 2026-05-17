-- 0020_create_encryption_key_rotations.up.sql

CREATE TABLE IF NOT EXISTS encryption_key_rotations (
    id                       UUID        PRIMARY KEY,
    key_version              INT         NOT NULL,
    status                   VARCHAR(20) NOT NULL DEFAULT 'pending'
                                 CHECK (status IN ('pending', 'in_progress', 'completed', 'failed')),
    initiated_by_member_id   UUID        NULL REFERENCES workspace_members(id) ON DELETE SET NULL,
    channels_total           INT         NOT NULL DEFAULT 0,
    channels_rotated         INT         NOT NULL DEFAULT 0,
    channels_failed          INT         NOT NULL DEFAULT 0,
    started_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at             TIMESTAMPTZ NULL,
    notes                    TEXT        NULL
);

COMMENT ON TABLE encryption_key_rotations IS
    'Tracks each encryption key rotation job that re-encrypts channel payloads '
    'from one key version to the next. Progress counters (channels_total, '
    'channels_rotated, channels_failed) are updated in-place by the rotation '
    'worker; status transitions: pending -> in_progress -> completed | failed.';

COMMENT ON COLUMN encryption_key_rotations.key_version            IS 'Target key version being rotated to (monotonically increasing integer managed by the secrets store).';
COMMENT ON COLUMN encryption_key_rotations.status                 IS 'Lifecycle state of the rotation job.';
COMMENT ON COLUMN encryption_key_rotations.initiated_by_member_id IS 'Workspace member who triggered the rotation; NULL when initiated by an automated system process.';
COMMENT ON COLUMN encryption_key_rotations.channels_total         IS 'Total number of channels that must be re-encrypted in this rotation.';
COMMENT ON COLUMN encryption_key_rotations.channels_rotated       IS 'Running count of channels successfully re-encrypted.';
COMMENT ON COLUMN encryption_key_rotations.channels_failed        IS 'Running count of channels that failed re-encryption (will require a retry run).';
COMMENT ON COLUMN encryption_key_rotations.completed_at           IS 'Timestamp when the job reached a terminal state (completed or failed); NULL while still running.';
COMMENT ON COLUMN encryption_key_rotations.notes                  IS 'Free-text operator notes, failure summaries, or references to incident tickets.';

-- Monitor all non-terminal jobs (operations dashboard / health checks).
CREATE INDEX IF NOT EXISTS idx_key_rotations_status
    ON encryption_key_rotations (status);

-- Lookup the rotation history for a specific key version.
CREATE INDEX IF NOT EXISTS idx_key_rotations_version
    ON encryption_key_rotations (key_version);
