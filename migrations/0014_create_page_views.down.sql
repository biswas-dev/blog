DROP FUNCTION IF EXISTS ensure_page_views_partition(DATE);
DROP FUNCTION IF EXISTS aggregate_page_views_daily(DATE);
DROP TABLE IF EXISTS page_views_daily;
DROP TABLE IF EXISTS page_views;
