-- Performance indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_posts_is_published_created ON posts(is_published, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
