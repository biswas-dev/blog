DELETE FROM site_settings WHERE key = 'goodreads_user_id';
DROP TABLE IF EXISTS goodreads_sync_state;
DROP TABLE IF EXISTS book_review_history;
