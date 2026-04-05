CREATE TABLE IF NOT EXISTS post_annotations (
    id SERIAL PRIMARY KEY,
    post_id INT NOT NULL REFERENCES Posts(post_id) ON DELETE CASCADE,
    author_id INT NOT NULL REFERENCES Users(user_id),
    start_offset INT NOT NULL,
    end_offset INT NOT NULL,
    selected_text TEXT NOT NULL,
    color VARCHAR(10) NOT NULL DEFAULT 'yellow',
    resolved BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_post_annotations_post_id ON post_annotations(post_id);

CREATE TABLE IF NOT EXISTS post_annotation_comments (
    id SERIAL PRIMARY KEY,
    annotation_id INT NOT NULL REFERENCES post_annotations(id) ON DELETE CASCADE,
    author_id INT NOT NULL REFERENCES Users(user_id),
    parent_comment_id INT REFERENCES post_annotation_comments(id),
    content TEXT NOT NULL,
    resolved BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_post_annotation_comments_ann_id ON post_annotation_comments(annotation_id);
