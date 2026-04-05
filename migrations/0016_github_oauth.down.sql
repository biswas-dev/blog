DROP INDEX IF EXISTS idx_oauth_providers_user_id;
DROP TABLE IF EXISTS oauth_providers;
ALTER TABLE Users DROP COLUMN IF EXISTS auth_provider;
