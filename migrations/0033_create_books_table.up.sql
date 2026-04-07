-- Book genres (separate from post/guide Categories — different concerns)
CREATE TABLE IF NOT EXISTS Book_Genres (
    genre_id   SERIAL PRIMARY KEY,
    genre_name VARCHAR(255) NOT NULL UNIQUE,
    genre_group VARCHAR(50) NOT NULL DEFAULT 'non-fiction',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Books: reading tracker + public book reviews
CREATE TABLE IF NOT EXISTS Books (
    book_id          SERIAL PRIMARY KEY,
    user_id          INT REFERENCES Users(user_id),
    title            VARCHAR(255) NOT NULL,
    slug             VARCHAR(255) NOT NULL UNIQUE,
    book_author      VARCHAR(255) NOT NULL DEFAULT '',
    isbn             VARCHAR(20) DEFAULT '',
    publisher        VARCHAR(255) DEFAULT '',
    page_count       INT DEFAULT 0,
    cover_image_url  TEXT DEFAULT '',
    content          TEXT NOT NULL DEFAULT '',
    description      TEXT DEFAULT '',
    my_thoughts      TEXT DEFAULT '',
    link_url         TEXT DEFAULT '',
    reading_status   VARCHAR(20) NOT NULL DEFAULT 'want-to-read'
        CHECK (reading_status IN ('reading', 'completed', 'want-to-read', 'abandoned')),
    rating           SMALLINT DEFAULT 0 CHECK (rating >= 0 AND rating <= 5),
    date_started     DATE,
    date_finished    DATE,
    is_published     BOOLEAN DEFAULT FALSE,
    publication_date TIMESTAMP,
    last_edit_date   TIMESTAMP,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Many-to-many genres for books
CREATE TABLE IF NOT EXISTS Book_Genre_Map (
    book_id  INT REFERENCES Books(book_id) ON DELETE CASCADE,
    genre_id INT REFERENCES Book_Genres(genre_id) ON DELETE CASCADE,
    PRIMARY KEY (book_id, genre_id)
);

-- Book version tracking (same pattern as post_versions)
CREATE TABLE IF NOT EXISTS book_versions (
    id             SERIAL PRIMARY KEY,
    book_id        INT NOT NULL REFERENCES Books(book_id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    title          VARCHAR(255) NOT NULL,
    content        TEXT NOT NULL DEFAULT '',
    content_hash   VARCHAR(64) NOT NULL,
    created_by     INT NOT NULL REFERENCES Users(user_id),
    created_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (book_id, version_number)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_books_user_id ON Books(user_id);
CREATE INDEX IF NOT EXISTS idx_books_slug ON Books(slug);
CREATE INDEX IF NOT EXISTS idx_books_published ON Books(is_published);
CREATE INDEX IF NOT EXISTS idx_books_reading_status ON Books(reading_status);
CREATE INDEX IF NOT EXISTS idx_books_date_finished ON Books(date_finished);
CREATE INDEX IF NOT EXISTS idx_books_date_started ON Books(date_started);
CREATE INDEX IF NOT EXISTS idx_book_genre_map_book_id ON Book_Genre_Map(book_id);
CREATE INDEX IF NOT EXISTS idx_book_genre_map_genre_id ON Book_Genre_Map(genre_id);
CREATE INDEX IF NOT EXISTS idx_book_versions_book_id ON book_versions(book_id);

-- Seed genres (4 groups × ~7 each = 29 genres)
INSERT INTO Book_Genres (genre_name, genre_group) VALUES
    -- Tech
    ('Software Engineering', 'tech'),
    ('System Design', 'tech'),
    ('Programming Languages', 'tech'),
    ('DevOps & Cloud', 'tech'),
    ('Data Science', 'tech'),
    ('Artificial Intelligence', 'tech'),
    ('Cybersecurity', 'tech'),
    -- Non-fiction
    ('Biography', 'non-fiction'),
    ('Memoir', 'non-fiction'),
    ('Business', 'non-fiction'),
    ('Self-Help', 'non-fiction'),
    ('History', 'non-fiction'),
    ('Science', 'non-fiction'),
    ('Philosophy', 'non-fiction'),
    ('Psychology', 'non-fiction'),
    ('Economics', 'non-fiction'),
    ('Politics', 'non-fiction'),
    ('Travel', 'non-fiction'),
    -- Fiction
    ('Literary Fiction', 'fiction'),
    ('Science Fiction', 'fiction'),
    ('Fantasy', 'fiction'),
    ('Mystery', 'fiction'),
    ('Thriller', 'fiction'),
    ('Historical Fiction', 'fiction'),
    -- Non-tech (general)
    ('Productivity', 'non-tech'),
    ('Leadership', 'non-tech'),
    ('Creativity', 'non-tech'),
    ('Communication', 'non-tech'),
    ('Startups', 'non-tech')
ON CONFLICT (genre_name) DO NOTHING;
