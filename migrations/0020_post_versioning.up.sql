-- Track all editors per post (distinct from original author)
CREATE TABLE post_contributors (
    post_id INT NOT NULL REFERENCES Posts(post_id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES Users(user_id) ON DELETE CASCADE,
    first_contributed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_contributed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (post_id, user_id)
);
CREATE INDEX idx_post_contributors_user ON post_contributors(user_id);

-- Version snapshots
CREATE TABLE post_versions (
    id SERIAL PRIMARY KEY,
    post_id INT NOT NULL REFERENCES Posts(post_id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash CHAR(64) NOT NULL,
    created_by INT NOT NULL REFERENCES Users(user_id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (post_id, version_number)
);
CREATE INDEX idx_post_versions_post_id ON post_versions(post_id);
