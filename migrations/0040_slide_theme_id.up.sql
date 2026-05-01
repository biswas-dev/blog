-- Backfill slide_metadata.theme_id for every published slide so go-slide
-- knows which theme to render with. Heuristic:
--   * Decks whose content references the .aal- design system (the AI Agent
--     Lens pitch) get theme_id = 'aal'.
--   * Everything else gets theme_id = 'default'.
-- The heuristic runs once; further edits go through the editor which sets
-- theme_id explicitly via the right-panel theme picker.
--
-- Idempotent: a slide that already has theme_id in its metadata is left
-- alone.

UPDATE Slides
SET slide_metadata = jsonb_set(
    COALESCE(slide_metadata::jsonb, '{}'::jsonb),
    '{theme_id}',
    to_jsonb(
        CASE
            WHEN content ILIKE '%class="aal %' OR content ILIKE '%class="aal-light %' THEN 'aal'
            ELSE 'default'
        END
    ),
    true
)::text
WHERE slide_metadata IS NULL
   OR slide_metadata = ''
   OR slide_metadata::jsonb -> 'theme_id' IS NULL;
