-- 0021_correctif_workspace_settings.up.sql

-- Add columns introduced in the T4 canonical WorkspaceSettings model
-- that were absent from the original workspaces schema.
ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS settings_date_format VARCHAR(20) NOT NULL DEFAULT 'DD/MM/YYYY'
        CHECK (settings_date_format IN ('DD/MM/YYYY', 'MM/DD/YYYY', 'YYYY-MM-DD')),
    ADD COLUMN IF NOT EXISTS settings_time_format VARCHAR(5)  NOT NULL DEFAULT '24h'
        CHECK (settings_time_format IN ('24h', '12h')),
    ADD COLUMN IF NOT EXISTS settings_week_start_day SMALLINT NOT NULL DEFAULT 1
        CHECK (settings_week_start_day BETWEEN 0 AND 6);

-- settings_require_approval removed from domain (not in T4 canonical WorkspaceSettings).
ALTER TABLE workspaces DROP COLUMN IF EXISTS settings_require_approval;

COMMENT ON COLUMN workspaces.settings_date_format     IS 'Display format for dates workspace-wide: DD/MM/YYYY | MM/DD/YYYY | YYYY-MM-DD.';
COMMENT ON COLUMN workspaces.settings_time_format     IS 'Clock convention used across the workspace: 24h or 12h (AM/PM).';
COMMENT ON COLUMN workspaces.settings_week_start_day  IS 'ISO-like day-of-week index for the first day of the week: 0 = Sunday, 1 = Monday (default), … 6 = Saturday.';
