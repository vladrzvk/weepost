-- 0021_correctif_workspace_settings.down.sql

ALTER TABLE workspaces
    DROP COLUMN IF EXISTS settings_date_format,
    DROP COLUMN IF EXISTS settings_time_format,
    DROP COLUMN IF EXISTS settings_week_start_day,
    ADD COLUMN IF NOT EXISTS settings_require_approval BOOLEAN NOT NULL DEFAULT false;
