-- 0015_create_publish_jobs.down.sql
DROP TRIGGER IF EXISTS trg_publish_jobs_updated_at ON publish_jobs;
DROP TABLE IF EXISTS publish_jobs CASCADE;
