-- The theme_id backfill happens in Go startup code (SlideService.MigrateThemeID).
-- Doing it in SQL is fragile because legacy rows can carry empty strings or
-- malformed JSON in slide_metadata, which crashes the ::jsonb cast even
-- when wrapped in a CASE — Postgres evaluates the cast expression eagerly
-- in some plan shapes.
--
-- Keep this migration in the chain so the version counter stays monotonic
-- across environments.

SELECT 1;
