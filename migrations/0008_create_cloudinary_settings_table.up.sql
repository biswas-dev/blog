CREATE TABLE IF NOT EXISTS cloudinary_settings (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    cloud_name VARCHAR(100) NOT NULL,
    api_key VARCHAR(100) NOT NULL,
    api_secret_encrypted BYTEA NOT NULL,
    api_secret_nonce BYTEA NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    status VARCHAR(20) DEFAULT 'unknown',
    last_checked_at TIMESTAMP,
    consecutive_failures INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
