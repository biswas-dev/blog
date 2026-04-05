-- Guides: long-form structured content with floating table of contents
CREATE TABLE IF NOT EXISTS Guides (
    guide_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES Users(user_id),
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    slug VARCHAR(255) NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    featured_image_url TEXT DEFAULT '',
    is_published BOOLEAN DEFAULT FALSE,
    publication_date TIMESTAMP,
    last_edit_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Many-to-many categories for guides
CREATE TABLE IF NOT EXISTS Guide_Categories (
    guide_id INT REFERENCES Guides(guide_id) ON DELETE CASCADE,
    category_id INT REFERENCES Categories(category_id) ON DELETE CASCADE,
    PRIMARY KEY (guide_id, category_id)
);

-- Allow comments on guides (alongside existing post comments)
ALTER TABLE Comments ADD COLUMN IF NOT EXISTS guide_id INT REFERENCES Guides(guide_id) ON DELETE CASCADE;

-- Indexes
CREATE INDEX IF NOT EXISTS idx_guides_user_id ON Guides(user_id);
CREATE INDEX IF NOT EXISTS idx_guides_slug ON Guides(slug);
CREATE INDEX IF NOT EXISTS idx_guides_published ON Guides(is_published);
CREATE INDEX IF NOT EXISTS idx_guide_categories_guide_id ON Guide_Categories(guide_id);
CREATE INDEX IF NOT EXISTS idx_guide_categories_category_id ON Guide_Categories(category_id);
CREATE INDEX IF NOT EXISTS idx_comments_guide_id ON Comments(guide_id);
