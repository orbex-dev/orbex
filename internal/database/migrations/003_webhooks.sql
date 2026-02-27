-- Migration 003: Add webhook token for external triggers

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS webhook_token TEXT UNIQUE;

CREATE INDEX IF NOT EXISTS idx_jobs_webhook_token 
    ON jobs (webhook_token) 
    WHERE webhook_token IS NOT NULL;
