-- Slide contributors (mirrors post_contributors)
CREATE TABLE slide_contributors (
    slide_id INT NOT NULL REFERENCES Slides(slide_id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES Users(user_id) ON DELETE CASCADE,
    first_contributed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_contributed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (slide_id, user_id)
);
CREATE INDEX idx_slide_contributors_user ON slide_contributors(user_id);

-- Slide versions (mirrors post_versions)
CREATE TABLE slide_versions (
    id SERIAL PRIMARY KEY,
    slide_id INT NOT NULL REFERENCES Slides(slide_id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash CHAR(64) NOT NULL,
    created_by INT NOT NULL REFERENCES Users(user_id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (slide_id, version_number)
);
CREATE INDEX idx_slide_versions_slide_id ON slide_versions(slide_id);

-- New columns on Slides table
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS password_hash TEXT DEFAULT '';
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS slide_metadata JSONB DEFAULT '{}';
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS slide_count INT DEFAULT 0;
