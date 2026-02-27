-- Migration 002: Add heartbeat column for detecting stuck runs

ALTER TABLE job_runs ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_job_runs_heartbeat 
    ON job_runs (heartbeat_at) 
    WHERE status IN ('running', 'paused');
