CREATE TABLE wiki_pages (
    page_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES Users(user_id),
    title VARCHAR(500) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_wiki_pages_slug ON wiki_pages(slug);

CREATE TABLE wiki_page_versions (
    version_id SERIAL PRIMARY KEY,
    page_id INT NOT NULL REFERENCES wiki_pages(page_id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    content TEXT NOT NULL,
    content_hash CHAR(64) NOT NULL,
    created_by INT NOT NULL REFERENCES Users(user_id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (page_id, version_number)
);

CREATE INDEX idx_wiki_page_versions_page_id ON wiki_page_versions(page_id);
