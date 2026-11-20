-- DeckCraft decks: a slide whose deck_name is non-NULL is edited and shown by DeckCraft
-- (static/vendor/deckcraft); its presentation file lives here so slide passwords and
-- unpublished state guard it. NULL keeps the legacy go-slide HTML deck.
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS deck_name TEXT;
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS deck_file BYTEA;
