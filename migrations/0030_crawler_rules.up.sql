CREATE TABLE IF NOT EXISTS crawler_rules (
    id SERIAL PRIMARY KEY,
    crawler_pattern VARCHAR(100) NOT NULL,
    action VARCHAR(20) NOT NULL DEFAULT 'allow',
    time_start INT,
    time_end INT,
    reason TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
