-- 0002_create_workspaces.down.sql
DROP TRIGGER IF EXISTS trg_workspaces_updated_at ON workspaces;
DROP TABLE IF EXISTS workspaces CASCADE;
