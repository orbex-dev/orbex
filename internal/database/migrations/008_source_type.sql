-- Source type for jobs: image, script, upload, dockerfile, github, compose
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT 'image';

-- Build queue for Dockerfile/GitHub builds
CREATE TABLE IF NOT EXISTS build_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    image_tag TEXT,
    build_log TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

-- GitHub tokens for repo access
CREATE TABLE IF NOT EXISTS github_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_token TEXT NOT NULL,
    github_username TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(user_id)
);

-- GitHub repo config on jobs
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS github_repo TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS github_branch TEXT DEFAULT 'main';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS github_token_id UUID REFERENCES github_tokens(id);
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS dockerfile_path TEXT DEFAULT './Dockerfile';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS source_config JSONB DEFAULT '{}';
