ALTER TABLE page_views ADD COLUMN IF NOT EXISTS crawler_type VARCHAR(30) DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_page_views_crawler ON page_views(crawler_type) WHERE crawler_type != '';
