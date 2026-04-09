ALTER TABLE Books DROP CONSTRAINT IF EXISTS books_rating_check;
ALTER TABLE Books ALTER COLUMN rating TYPE SMALLINT USING rating::SMALLINT;
ALTER TABLE Books ADD CONSTRAINT books_rating_check CHECK (rating >= 0 AND rating <= 5);
