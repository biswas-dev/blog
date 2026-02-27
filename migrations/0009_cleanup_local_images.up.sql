-- Migration 0009: Clean up local image references after Cloudinary migration
-- This is a one-time data cleanup. Local images have been migrated to Cloudinary.

BEGIN;

-- 1. Clear featured_image_url for posts with local paths (preserve Cloudinary URLs)
UPDATE posts
SET featured_image_url = ''
WHERE featured_image_url LIKE '/static/uploads/%';

-- 2. Remove markdown image references to local uploads: ![alt](/static/uploads/...)
UPDATE posts
SET content = regexp_replace(
    content,
    '!\[[^\]]*\]\(/static/uploads/[^)]+\)',
    '',
    'g'
)
WHERE content LIKE '%/static/uploads/%';

-- 3. Remove HTML image references to local uploads: <img ... src="/static/uploads/..." ...>
UPDATE posts
SET content = regexp_replace(
    content,
    '<img\s[^>]*src="/static/uploads/[^"]*"[^>]*/?>',
    '',
    'g'
)
WHERE content LIKE '%/static/uploads/%';

-- 4. Clear profile_picture_url for users with local paths
UPDATE users
SET profile_picture_url = ''
WHERE profile_picture_url LIKE '/static/uploads/%';

COMMIT;
