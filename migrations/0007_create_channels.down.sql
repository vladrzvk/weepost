-- 0007_create_channels.down.sql
DROP TRIGGER IF EXISTS trg_channels_updated_at ON channels;
DROP TABLE IF EXISTS channels CASCADE;
