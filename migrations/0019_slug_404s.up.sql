CREATE TABLE IF NOT EXISTS slug_404s (
    id SERIAL PRIMARY KEY,
    slug TEXT NOT NULL,
    hit_count INT NOT NULL DEFAULT 1,
    first_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    whitelisted BOOLEAN NOT NULL DEFAULT FALSE,
    whitelist_note TEXT,
    CONSTRAINT slug_404s_slug_unique UNIQUE (slug)
);
CREATE INDEX IF NOT EXISTS idx_slug_404s_last_seen ON slug_404s(last_seen);
CREATE INDEX IF NOT EXISTS idx_slug_404s_hit_count ON slug_404s(hit_count DESC);
