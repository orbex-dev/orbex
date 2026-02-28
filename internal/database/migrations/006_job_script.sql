-- Add inline script support to jobs
ALTER TABLE jobs ADD COLUMN script TEXT;
ALTER TABLE jobs ADD COLUMN script_lang TEXT;  -- 'python', 'node', 'bash', 'go', 'ruby'
