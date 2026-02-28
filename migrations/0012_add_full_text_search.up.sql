-- Add full-text search support for posts and slides

-- Add search_vector column to posts table
ALTER TABLE posts ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Add search_vector and search_content columns to slides table
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS search_content TEXT DEFAULT '';
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Create GIN indexes for fast full-text search
CREATE INDEX IF NOT EXISTS idx_posts_search_vector ON posts USING GIN (search_vector);
CREATE INDEX IF NOT EXISTS idx_slides_search_vector ON Slides USING GIN (search_vector);

-- Trigger function to auto-update posts search_vector on INSERT/UPDATE
CREATE OR REPLACE FUNCTION posts_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.content, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger function to auto-update slides search_vector on INSERT/UPDATE
CREATE OR REPLACE FUNCTION slides_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.search_content, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers
DROP TRIGGER IF EXISTS posts_search_vector_trigger ON posts;
CREATE TRIGGER posts_search_vector_trigger
    BEFORE INSERT OR UPDATE OF title, content ON posts
    FOR EACH ROW
    EXECUTE FUNCTION posts_search_vector_update();

DROP TRIGGER IF EXISTS slides_search_vector_trigger ON Slides;
CREATE TRIGGER slides_search_vector_trigger
    BEFORE INSERT OR UPDATE OF title, search_content ON Slides
    FOR EACH ROW
    EXECUTE FUNCTION slides_search_vector_update();

-- Backfill existing posts
UPDATE posts SET search_vector =
    setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(content, '')), 'B');

-- Backfill existing slides (title only; search_content will be populated by Go startup code)
UPDATE Slides SET search_vector =
    setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(search_content, '')), 'B');
