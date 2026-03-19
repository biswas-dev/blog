-- Move slide content from filesystem into database
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS content TEXT DEFAULT '';
