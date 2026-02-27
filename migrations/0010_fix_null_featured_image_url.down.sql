BEGIN;

ALTER TABLE posts ALTER COLUMN featured_image_url DROP NOT NULL;
ALTER TABLE posts ALTER COLUMN featured_image_url DROP DEFAULT;

ALTER TABLE users ALTER COLUMN profile_picture_url DROP NOT NULL;
ALTER TABLE users ALTER COLUMN profile_picture_url DROP DEFAULT;

COMMIT;
