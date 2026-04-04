-- page_engagement: per-page-view reader engagement tracking
-- Records how long a user spent on a page and per-section attention.
-- Sent via navigator.sendBeacon on page hide/unload.

CREATE TABLE page_engagement (
    id              BIGSERIAL       NOT NULL,
    session_id      UUID            NOT NULL,
    path            TEXT            NOT NULL,
    ip_address      INET            NOT NULL,
    user_id         INT,
    referrer        TEXT            NOT NULL DEFAULT '',
    user_agent      TEXT            NOT NULL DEFAULT '',
    started_at      TIMESTAMPTZ     NOT NULL,
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    active_seconds  INT             NOT NULL DEFAULT 0,   -- time tab was visible & active
    total_seconds   INT             NOT NULL DEFAULT 0,   -- wall-clock time on page
    max_scroll_pct  SMALLINT        NOT NULL DEFAULT 0,   -- 0-100
    sections        JSONB           NOT NULL DEFAULT '[]'::jsonb, -- [{id, text, active_seconds, viewed}]
    completed       BOOLEAN         NOT NULL DEFAULT FALSE, -- true when final beacon received
    PRIMARY KEY (id, started_at)
) PARTITION BY RANGE (started_at);

-- Monthly partitions
CREATE TABLE page_engagement_2026_04 PARTITION OF page_engagement
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE page_engagement_2026_05 PARTITION OF page_engagement
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

-- Upsert by session_id (unique per partition)
CREATE UNIQUE INDEX idx_page_engagement_session ON page_engagement (session_id, started_at);
CREATE INDEX idx_page_engagement_path_started ON page_engagement (path, started_at DESC);
CREATE INDEX idx_page_engagement_ip_started ON page_engagement (ip_address, started_at DESC);

-- Helper function: ensure next month's partition exists
CREATE OR REPLACE FUNCTION ensure_page_engagement_partition(target_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := date_trunc('month', target_date)::DATE;
    end_date := (start_date + INTERVAL '1 month')::DATE;
    partition_name := 'page_engagement_' || to_char(start_date, 'YYYY_MM');
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = partition_name) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF page_engagement FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
    END IF;
END;
$$ LANGUAGE plpgsql;
