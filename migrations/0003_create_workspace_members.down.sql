-- 0003_create_workspace_members.down.sql
DROP TRIGGER IF EXISTS trg_workspace_members_updated_at ON workspace_members;
DROP TABLE IF EXISTS workspace_members CASCADE;
