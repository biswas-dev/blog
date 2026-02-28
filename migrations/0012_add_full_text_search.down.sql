-- Remove full-text search support

-- Drop triggers
DROP TRIGGER IF EXISTS posts_search_vector_trigger ON posts;
DROP TRIGGER IF EXISTS slides_search_vector_trigger ON Slides;

-- Drop trigger functions
DROP FUNCTION IF EXISTS posts_search_vector_update();
DROP FUNCTION IF EXISTS slides_search_vector_update();

-- Drop indexes
DROP INDEX IF EXISTS idx_posts_search_vector;
DROP INDEX IF EXISTS idx_slides_search_vector;

-- Drop columns
ALTER TABLE posts DROP COLUMN IF EXISTS search_vector;
ALTER TABLE Slides DROP COLUMN IF EXISTS search_vector;
ALTER TABLE Slides DROP COLUMN IF EXISTS search_content;
