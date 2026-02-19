-- Orbex Initial Schema
-- Migration 001: Core tables for users, API keys, jobs, job runs, and queue

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- Users
-- ============================================================
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT NOT NULL UNIQUE,
    password    TEXT NOT NULL,  -- bcrypt hash
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users (email);

-- ============================================================
-- API Keys
-- ============================================================
CREATE TABLE api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT 'default',
    key_hash    TEXT NOT NULL UNIQUE,  -- SHA-256 hash of the actual key
    prefix      TEXT NOT NULL,          -- First 8 chars for identification (e.g., "obx_1a2b")
    last_used   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_keys_key_hash ON api_keys (key_hash);
CREATE INDEX idx_api_keys_user_id ON api_keys (user_id);

-- ============================================================
-- Jobs (definitions)
-- ============================================================
CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    image           TEXT NOT NULL,           -- Docker image (e.g., "python:3.12")
    command         TEXT[],                  -- Override CMD
    env             JSONB NOT NULL DEFAULT '{}',  -- Environment variables
    memory_mb       INT NOT NULL DEFAULT 512,
    cpu_millicores  INT NOT NULL DEFAULT 1000,    -- 1000 = 1 CPU core
    timeout_seconds INT NOT NULL DEFAULT 3600,    -- Default 1 hour
    schedule        TEXT,                    -- Cron expression (NULL = manual only)
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(user_id, name)
);

CREATE INDEX idx_jobs_user_id ON jobs (user_id);
CREATE INDEX idx_jobs_schedule ON jobs (schedule) WHERE schedule IS NOT NULL;

-- ============================================================
-- Job Runs (execution history)
-- ============================================================
CREATE TYPE run_status AS ENUM (
    'pending',
    'running',
    'succeeded',
    'failed',
    'paused',
    'cancelled'
);

CREATE TABLE job_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          run_status NOT NULL DEFAULT 'pending',
    container_id    TEXT,                    -- Docker container ID
    exit_code       INT,
    error_message   TEXT,
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    paused_at       TIMESTAMPTZ,
    duration_ms     BIGINT,                 -- Actual compute time in milliseconds
    logs_tail       TEXT,                    -- Last 1000 lines of stdout/stderr
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_job_runs_job_id ON job_runs (job_id);
CREATE INDEX idx_job_runs_user_id ON job_runs (user_id);
CREATE INDEX idx_job_runs_status ON job_runs (status);
CREATE INDEX idx_job_runs_created_at ON job_runs (created_at DESC);

-- ============================================================
-- Job Queue (Postgres-based with SKIP LOCKED)
-- ============================================================
CREATE TABLE job_queue (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    run_id          UUID NOT NULL REFERENCES job_runs(id) ON DELETE CASCADE,
    priority        INT NOT NULL DEFAULT 0,     -- Higher = picked first
    scheduled_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    picked_at       TIMESTAMPTZ,               -- When a worker picks it up
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_job_queue_pending ON job_queue (priority DESC, scheduled_at ASC)
    WHERE picked_at IS NULL;
