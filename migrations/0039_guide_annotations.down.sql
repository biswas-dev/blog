-- Revert: remove guide_id and restore post_id NOT NULL.
ALTER TABLE post_annotations DROP CONSTRAINT IF EXISTS post_annotations_target_check;
DROP INDEX IF EXISTS idx_post_annotations_guide_id;
DELETE FROM post_annotations WHERE post_id IS NULL;
ALTER TABLE post_annotations DROP COLUMN IF EXISTS guide_id;
ALTER TABLE post_annotations ALTER COLUMN post_id SET NOT NULL;
