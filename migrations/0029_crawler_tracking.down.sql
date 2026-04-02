DROP INDEX IF EXISTS idx_page_views_crawler;
ALTER TABLE page_views DROP COLUMN IF EXISTS crawler_type;
