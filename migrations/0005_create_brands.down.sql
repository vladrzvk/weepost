-- 0005_create_brands.down.sql
DROP TRIGGER IF EXISTS trg_brands_updated_at ON brands;
DROP TABLE IF EXISTS brands CASCADE;
