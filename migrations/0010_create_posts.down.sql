-- 0010_create_posts.down.sql
DROP TRIGGER IF EXISTS trg_posts_updated_at ON posts;
DROP TABLE IF EXISTS posts CASCADE;
