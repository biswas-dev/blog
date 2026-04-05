ALTER TABLE Slides DROP COLUMN IF EXISTS slide_count;
ALTER TABLE Slides DROP COLUMN IF EXISTS description;
ALTER TABLE Slides DROP COLUMN IF EXISTS slide_metadata;
ALTER TABLE Slides DROP COLUMN IF EXISTS password_hash;

DROP TABLE IF EXISTS slide_versions;
DROP TABLE IF EXISTS slide_contributors;
