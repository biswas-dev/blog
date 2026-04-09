-- Extend post_annotations to also support guides.
-- post_id becomes nullable; a new guide_id column references Guides. A CHECK
-- constraint ensures exactly one of post_id/guide_id is set per annotation.

ALTER TABLE post_annotations ALTER COLUMN post_id DROP NOT NULL;
ALTER TABLE post_annotations ADD COLUMN IF NOT EXISTS guide_id INT REFERENCES Guides(guide_id) ON DELETE CASCADE;

ALTER TABLE post_annotations DROP CONSTRAINT IF EXISTS post_annotations_target_check;
ALTER TABLE post_annotations ADD CONSTRAINT post_annotations_target_check
    CHECK ((post_id IS NOT NULL AND guide_id IS NULL) OR (post_id IS NULL AND guide_id IS NOT NULL));

CREATE INDEX IF NOT EXISTS idx_post_annotations_guide_id ON post_annotations(guide_id);
