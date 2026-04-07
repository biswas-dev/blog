-- Reading medium (how the book was consumed)
ALTER TABLE Books ADD COLUMN IF NOT EXISTS medium VARCHAR(20) DEFAULT ''
    CHECK (medium IN ('', 'physical', 'ebook', 'audiobook'));

-- eBook reader device used (optional)
ALTER TABLE Books ADD COLUMN IF NOT EXISTS ebook_reader VARCHAR(100) DEFAULT '';

-- Seed configured ebook readers as site setting (JSON array)
INSERT INTO site_settings (key, value) VALUES (
    'ebook_readers',
    '["Kindle Scribe Gen 1","Boox Go 7","Kobo Libra 2","Boox Tab Ultra 13"]'
) ON CONFLICT (key) DO NOTHING;
