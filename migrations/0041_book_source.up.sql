-- Where the book came from (e.g. "Audible", "Hoopla · Markham Public Library") and a link to it
ALTER TABLE Books ADD COLUMN IF NOT EXISTS source_name VARCHAR(255) DEFAULT '';
ALTER TABLE Books ADD COLUMN IF NOT EXISTS source_url TEXT DEFAULT '';
