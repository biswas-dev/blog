-- Roll back theme_id backfill (best-effort: removes the key, leaves
-- otherwise-modified metadata alone).
UPDATE Slides
SET slide_metadata = (slide_metadata::jsonb - 'theme_id')::text
WHERE slide_metadata IS NOT NULL
  AND slide_metadata <> ''
  AND slide_metadata::jsonb -> 'theme_id' IS NOT NULL;
