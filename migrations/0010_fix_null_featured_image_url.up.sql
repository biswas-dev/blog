-- Migration 0010: Convert NULL featured_image_url to empty string
-- Go's database/sql cannot scan NULL into a plain string field,
-- causing panics in the application.

BEGIN;

UPDATE posts SET featured_image_url = '' WHERE featured_image_url IS NULL;
ALTER TABLE posts ALTER COLUMN featured_image_url SET DEFAULT '';
ALTER TABLE posts ALTER COLUMN featured_image_url SET NOT NULL;

UPDATE users SET profile_picture_url = '' WHERE profile_picture_url IS NULL;
ALTER TABLE users ALTER COLUMN profile_picture_url SET DEFAULT '';
ALTER TABLE users ALTER COLUMN profile_picture_url SET NOT NULL;

COMMIT;
