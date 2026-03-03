CREATE TABLE IF NOT EXISTS ip_rules (
    id          SERIAL      PRIMARY KEY,
    ip_address  INET        NOT NULL,
    action      VARCHAR(10) NOT NULL,
    reason      TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ip_rules_action_check  CHECK (action IN ('ban', 'allow')),
    CONSTRAINT ip_rules_ip_unique     UNIQUE (ip_address)
);

CREATE INDEX IF NOT EXISTS idx_ip_rules_ip     ON ip_rules (ip_address);
CREATE INDEX IF NOT EXISTS idx_ip_rules_action ON ip_rules (action);
