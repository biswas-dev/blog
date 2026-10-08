-- Previous star rating + my_thoughts for a book, captured each time either one
-- changes (editor, API, Goodreads sync or a revert), so any of them can be restored.
CREATE TABLE IF NOT EXISTS book_review_history (
    id          SERIAL PRIMARY KEY,
    book_id     INTEGER NOT NULL REFERENCES Books(book_id) ON DELETE CASCADE,
    rating      NUMERIC(2,1) NOT NULL DEFAULT 0,
    my_thoughts TEXT NOT NULL DEFAULT '',
    replaced_by VARCHAR(32) NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_book_review_history_book ON book_review_history (book_id, created_at DESC);

-- Last rating/review seen on Goodreads per book. A sync only writes to the blog
-- when Goodreads has changed since then, so blog-side edits and reverts stick.
CREATE TABLE IF NOT EXISTS goodreads_sync_state (
    book_id           INTEGER PRIMARY KEY REFERENCES Books(book_id) ON DELETE CASCADE,
    goodreads_book_id VARCHAR(32) NOT NULL DEFAULT '',
    rating            NUMERIC(2,1) NOT NULL DEFAULT 0,
    review            TEXT NOT NULL DEFAULT '',
    synced_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO site_settings (key, value, updated_at) VALUES ('goodreads_user_id', '40689440', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
