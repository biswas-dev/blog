CREATE TABLE IF NOT EXISTS brevo_settings (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    api_key_encrypted BYTEA NOT NULL,
    api_key_nonce BYTEA NOT NULL,
    from_email VARCHAR(255) NOT NULL,
    from_name VARCHAR(255) NOT NULL DEFAULT 'Blog',
    is_enabled BOOLEAN DEFAULT true,
    status VARCHAR(20) DEFAULT 'unknown',
    last_checked_at TIMESTAMP,
    consecutive_failures INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
