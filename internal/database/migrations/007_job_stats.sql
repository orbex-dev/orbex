-- Track running statistics for anomaly detection
CREATE TABLE IF NOT EXISTS job_stats (
    job_id           UUID PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    run_count        INT NOT NULL DEFAULT 0,
    avg_duration_ms  BIGINT NOT NULL DEFAULT 0,
    stddev_duration_ms BIGINT NOT NULL DEFAULT 0,
    min_duration_ms  BIGINT NOT NULL DEFAULT 0,
    max_duration_ms  BIGINT NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
