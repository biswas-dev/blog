ALTER TABLE Books DROP COLUMN IF EXISTS medium;
ALTER TABLE Books DROP COLUMN IF EXISTS ebook_reader;
DELETE FROM site_settings WHERE key = 'ebook_readers';
