-- Page Views: partitioned raw events table
CREATE TABLE IF NOT EXISTS page_views (
    id          BIGSERIAL       NOT NULL,
    viewed_at   TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    ip_address  INET            NOT NULL,
    path        TEXT            NOT NULL,
    user_agent  TEXT            NOT NULL DEFAULT '',
    referrer    TEXT            NOT NULL DEFAULT '',
    user_id     INT,
    content_type VARCHAR(10)   NOT NULL DEFAULT 'other',
    PRIMARY KEY (id, viewed_at)
) PARTITION BY RANGE (viewed_at);

-- Create initial monthly partitions (Feb-May 2026)
CREATE TABLE IF NOT EXISTS page_views_2026_02 PARTITION OF page_views
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE TABLE IF NOT EXISTS page_views_2026_03 PARTITION OF page_views
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE IF NOT EXISTS page_views_2026_04 PARTITION OF page_views
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

CREATE TABLE IF NOT EXISTS page_views_2026_05 PARTITION OF page_views
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

-- Indexes on partitioned table (applied to each partition automatically)
CREATE INDEX IF NOT EXISTS idx_page_views_viewed_at ON page_views (viewed_at DESC);
CREATE INDEX IF NOT EXISTS idx_page_views_path_viewed ON page_views (path, viewed_at DESC);
CREATE INDEX IF NOT EXISTS idx_page_views_ip_viewed ON page_views (ip_address, viewed_at DESC);
CREATE INDEX IF NOT EXISTS idx_page_views_content_type_viewed ON page_views (content_type, viewed_at DESC);

-- Daily summary table (pre-aggregated)
CREATE TABLE IF NOT EXISTS page_views_daily (
    view_date       DATE        NOT NULL,
    path            TEXT        NOT NULL,
    content_type    VARCHAR(10) NOT NULL DEFAULT 'other',
    total_views     BIGINT      NOT NULL DEFAULT 0,
    unique_visitors BIGINT      NOT NULL DEFAULT 0,
    PRIMARY KEY (view_date, path, content_type)
);

CREATE INDEX IF NOT EXISTS idx_pvd_date ON page_views_daily (view_date DESC);
CREATE INDEX IF NOT EXISTS idx_pvd_content_date ON page_views_daily (content_type, view_date DESC);

-- Idempotent daily aggregation function
CREATE OR REPLACE FUNCTION aggregate_page_views_daily(target_date DATE)
RETURNS VOID AS $$
BEGIN
    INSERT INTO page_views_daily (view_date, path, content_type, total_views, unique_visitors)
    SELECT
        target_date,
        path,
        content_type,
        COUNT(*),
        COUNT(DISTINCT ip_address)
    FROM page_views
    WHERE viewed_at >= target_date
      AND viewed_at < target_date + INTERVAL '1 day'
    GROUP BY path, content_type
    ON CONFLICT (view_date, path, content_type)
    DO UPDATE SET
        total_views     = EXCLUDED.total_views,
        unique_visitors = EXCLUDED.unique_visitors;
END;
$$ LANGUAGE plpgsql;

-- Auto-create monthly partitions
CREATE OR REPLACE FUNCTION ensure_page_views_partition(target_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date     DATE;
    end_date       DATE;
BEGIN
    start_date     := DATE_TRUNC('month', target_date);
    end_date       := start_date + INTERVAL '1 month';
    partition_name := 'page_views_' || TO_CHAR(start_date, 'YYYY_MM');

    -- Check if partition already exists
    IF NOT EXISTS (
        SELECT 1 FROM pg_class WHERE relname = partition_name
    ) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF page_views FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
    END IF;
END;
$$ LANGUAGE plpgsql;
