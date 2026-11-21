-- Staging's books table lacked amazon_asin although schema_migrations was past 0037, so
-- /books failed with "column b.amazon_asin does not exist". Re-assert it; a no-op where it exists.
ALTER TABLE Books ADD COLUMN IF NOT EXISTS amazon_asin VARCHAR(10) DEFAULT '';
