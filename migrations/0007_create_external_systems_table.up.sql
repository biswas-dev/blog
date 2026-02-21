-- Create external_systems table for registering other blog instances
CREATE TABLE IF NOT EXISTS external_systems (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    api_key_encrypted BYTEA,
    api_key_nonce BYTEA,
    custom_headers_encrypted BYTEA,
    custom_headers_nonce BYTEA,
    is_active BOOLEAN DEFAULT true,
    last_sync_at TIMESTAMP,
    last_sync_status VARCHAR(50),
    last_sync_message TEXT,
    created_by INTEGER REFERENCES users(user_id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Unique constraints
CREATE UNIQUE INDEX idx_external_systems_name ON external_systems(name);
CREATE UNIQUE INDEX idx_external_systems_base_url ON external_systems(base_url);

-- Create sync_logs table for tracking sync operations
CREATE TABLE IF NOT EXISTS sync_logs (
    id SERIAL PRIMARY KEY,
    external_system_id INTEGER NOT NULL REFERENCES external_systems(id) ON DELETE CASCADE,
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('pull', 'push')),
    content_type VARCHAR(50) NOT NULL DEFAULT 'posts',
    status VARCHAR(50) NOT NULL DEFAULT 'started',
    items_synced INTEGER DEFAULT 0,
    items_skipped INTEGER DEFAULT 0,
    items_failed INTEGER DEFAULT 0,
    error_message TEXT,
    initiated_by INTEGER REFERENCES users(user_id),
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX idx_sync_logs_system_id ON sync_logs(external_system_id);
CREATE INDEX idx_sync_logs_started_at ON sync_logs(started_at DESC);
