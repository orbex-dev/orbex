-- Migration 004: Add notification fields to jobs

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS notify_webhook TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS notify_email TEXT;
