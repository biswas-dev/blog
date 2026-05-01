-- Backfill slide_metadata.theme_id for every published slide so go-slide
-- knows which theme to render with. Heuristic:
--   * Decks whose content references the .aal-* design system (the AI Agent
--     Lens pitch) get theme_id = 'aal'.
--   * Everything else gets theme_id = 'default'.
-- The heuristic runs once; further edits go through the editor which sets
-- theme_id explicitly via the right-panel theme picker.
--
-- Defensive: some legacy rows store NULL or '' in slide_metadata. We
-- coalesce both into '{}' inline (CASE wraps the cast so the jsonb
-- conversion never sees an empty string). Single statement keeps the
-- migration idempotent: rerunning is a no-op for rows already carrying
-- theme_id.

UPDATE Slides
SET slide_metadata = jsonb_set(
    CASE
        WHEN slide_metadata IS NULL OR slide_metadata = '' THEN '{}'::jsonb
        ELSE slide_metadata::jsonb
    END,
    '{theme_id}',
    to_jsonb(
        CASE
            WHEN content ILIKE '%class="aal %'
              OR content ILIKE '%class="aal-light %'
              OR content ILIKE '%class="aal pptx-slide%'
              OR content ILIKE '%class="aal-light pptx-slide%' THEN 'aal'
            ELSE 'default'
        END
    ),
    true
)::text
WHERE slide_metadata IS NULL
   OR slide_metadata = ''
   OR (
        -- Skip rows where theme_id already exists (re-running is a no-op).
        -- Wrap the cast in a defensive CASE so we never hit invalid JSON.
        CASE
            WHEN slide_metadata IS NULL OR slide_metadata = '' THEN NULL
            ELSE slide_metadata::jsonb -> 'theme_id'
        END
   ) IS NULL;
