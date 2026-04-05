-- Add featured/cover image to slides
ALTER TABLE Slides ADD COLUMN IF NOT EXISTS featured_image_url TEXT DEFAULT '';
