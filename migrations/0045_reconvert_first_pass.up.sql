-- Decks converted by the first converter (release of 2026-11-20, staging only) drew some themes
-- wrong. Clear those conversions so the startup job converts them again from their HTML. Only rows
-- the converter wrote and nobody has saved since: converted name, HTML kept, updated_at not bumped
-- (SaveConvertedDeck leaves it alone; a save from DeckCraft sets it to now).
UPDATE Slides
SET deck_name = NULL, deck_file = NULL
WHERE deck_name = slug || '.pptx'
  AND COALESCE(content, '') <> ''
  AND updated_at < TIMESTAMP '2026-11-20 16:00:00';
