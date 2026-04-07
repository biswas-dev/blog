-- Allow half-star ratings (e.g., 4.5, 3.5)
ALTER TABLE Books DROP CONSTRAINT IF EXISTS books_rating_check;
ALTER TABLE Books ALTER COLUMN rating TYPE NUMERIC(2,1) USING rating::NUMERIC(2,1);
ALTER TABLE Books ADD CONSTRAINT books_rating_check CHECK (rating >= 0 AND rating <= 5);
