CREATE TABLE IF NOT EXISTS site_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed default draw hover delay (seconds)
INSERT INTO site_settings (key, value) VALUES ('draw_hover_delay', '3')
ON CONFLICT (key) DO NOTHING;
