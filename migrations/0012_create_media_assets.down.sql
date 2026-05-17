-- 0012_create_media_assets.down.sql
DROP TRIGGER IF EXISTS trg_media_assets_updated_at ON media_assets;
DROP TABLE IF EXISTS media_assets CASCADE;
