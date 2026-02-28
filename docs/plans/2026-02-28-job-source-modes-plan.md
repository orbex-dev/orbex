# Job Source Modes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add file upload, Dockerfile build, GitHub repo integration, and docker-compose support as job source modes, backed by MinIO object storage and a build queue.

**Architecture:** Jobs gain a `source_type` column (`image|script|upload|dockerfile|github|compose`). Files are stored in MinIO. Builds go through a `build_queue` table with concurrency limits. GitHub uses OAuth for repo access + webhooks for auto-build on push.

**Tech Stack:** Go, MinIO SDK (`github.com/minio/minio-go/v7`), Docker Build API, GitHub OAuth, YAML parser (`gopkg.in/yaml.v3`)

---

## Task 1: MinIO Integration

**Files:**
- Create: `internal/storage/minio.go`
- Modify: `internal/config/config.go` (add MinIO config)
- Modify: `cmd/orbex-server/main.go` (initialize MinIO client)
- Modify: `.env` (add MinIO vars)

**Step 1: Add MinIO dependency**

```bash
go get github.com/minio/minio-go/v7
```

**Step 2: Create MinIO storage client**

Create `internal/storage/minio.go`:

```go
package storage

import (
    "context"
    "io"
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
    minio  *minio.Client
    bucket string
}

func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error)
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, error)
func (c *Client) List(ctx context.Context, prefix string) ([]ObjectInfo, error)
func (c *Client) Delete(ctx context.Context, key string) error
func (c *Client) EnsureBucket(ctx context.Context) error
```

**Step 3: Add config and wire into main.go**

Add to config:
```
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=orbex
MINIO_SECRET_KEY=orbexsecret
MINIO_BUCKET=orbex-storage
MINIO_USE_SSL=false
```

Initialize in `main.go`, pass to handlers that need it.

**Step 4: Start MinIO via Docker**

```bash
docker run -d --name orbex-minio \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=orbex \
  -e MINIO_ROOT_PASSWORD=orbexsecret \
  minio/minio server /data --console-address ":9001"
```

**Step 5: Verify & commit**

```bash
go build ./...
git add -A && git commit -m "feat: add MinIO storage client"
```

---

## Task 2: Source Type Model & Migration

**Files:**
- Create: `internal/database/migrations/008_source_type.sql`
- Modify: `internal/models/models.go` (add SourceType to Job)
- Modify: `internal/api/handlers_jobs.go` (include source_type in CRUD)

**Step 1: Create migration**

```sql
-- Add source_type column to jobs
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT 'image';
-- Values: image, script, upload, dockerfile, github, compose

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
```

`source_config` is a flexible JSONB field for source-specific metadata (e.g., compose service names, uploaded file list).

**Step 2: Update Go models**

Add `SourceType`, `GithubRepo`, `GithubBranch`, `DockerfilePath`, `SourceConfig` to `models.Job`, `CreateJobRequest`, `UpdateJobRequest`.

**Step 3: Update handlers to include new columns in SQL**

**Step 4: Verify & commit**

```bash
go build ./...
git add -A && git commit -m "feat: add source_type model, build_queue, github_tokens tables"
```

---

## Task 3: File Upload Source

**Files:**
- Create: `internal/api/handlers_upload.go`
- Modify: `internal/api/router.go` (add upload routes)
- Modify: `internal/worker/worker.go` (download files from MinIO at runtime)
- Modify: `dashboard/src/app/dashboard/jobs/page.tsx` (add Upload source tile + dropzone)
- Modify: `dashboard/src/lib/api.ts` (add upload functions)

**Step 1: Create upload handler**

```go
// POST /api/v1/jobs/{jobID}/upload — multipart file upload
// GET  /api/v1/jobs/{jobID}/files  — list uploaded files
// DELETE /api/v1/jobs/{jobID}/files/{filename} — delete file
```

Max file size: 50MB. Store at `uploads/{userID}/{jobID}/{filename}`.

Auto-detect image from file extensions if source_type is `upload` and no image specified:
- `.py` → `python:3.12-slim`
- `.js`/`.ts` → `node:22-slim`
- `.go` → `golang:1.22`
- `.rb` → `ruby:3.3-slim`
- `.sh` → `alpine:latest`

**Step 2: Update worker to handle `upload` source type**

In `executeRun`, when `source_type = 'upload'`:
1. Download all files from MinIO `uploads/{userID}/{jobID}/`
2. Write to temp dir
3. Bind-mount temp dir into container at `/orbex/workspace/`
4. Run entry command
5. Cleanup temp dir after

**Step 3: Add frontend upload UI**

Add "📁 Upload Files" tile to the source type selector. Step 3 becomes a file dropzone with drag-and-drop. Show uploaded files list with delete buttons.

**Step 4: Verify & commit**

```bash
go build ./...
git add -A && git commit -m "feat: add file upload source mode with MinIO storage"
```

---

## Task 4: Dockerfile Build Source

**Files:**
- Create: `internal/worker/builder.go` (build queue processor)
- Modify: `internal/api/handlers_upload.go` (allow Dockerfile upload)
- Modify: `internal/worker/worker.go` (use built image for dockerfile source)
- Modify: `dashboard/src/app/dashboard/jobs/page.tsx` (add Dockerfile source tile)

**Step 1: Create build worker**

```go
// RunBuilder polls build_queue for pending builds and executes them.
func (w *Worker) RunBuilder(ctx context.Context) {
    // Poll for pending builds
    // Download context from MinIO
    // docker build -t orbex/{jobID}:latest .
    // Update build_queue status
}
```

Use Docker SDK `ImageBuild` API. Tar the build context directory.

**Step 2: Add Dockerfile upload flow**

When user uploads a Dockerfile (detected by filename), set `source_type = 'dockerfile'`. Store at `builds/{userID}/{jobID}/`. Auto-enqueue build on upload.

**Step 3: Worker uses built image**

When `source_type = 'dockerfile'`, the worker uses `orbex/{jobID}:latest` as the image instead of the user-specified image.

**Step 4: Start builder loop in main.go**

```go
go worker.RunBuilder(ctx)
```

**Step 5: Frontend — add Dockerfile tile and upload**

**Step 6: Verify & commit**

```bash
go build ./...
git add -A && git commit -m "feat: add Dockerfile build source with build queue"
```

---

## Task 5: GitHub Repo Integration

**Files:**
- Create: `internal/api/handlers_github.go` (OAuth + webhook + repo listing)
- Modify: `internal/api/router.go` (add GitHub routes)
- Modify: `internal/worker/builder.go` (clone repo + build)
- Create: `dashboard/src/app/dashboard/jobs/github/` (repo selector component)
- Modify: `dashboard/src/lib/api.ts` (GitHub API calls)

**Step 1: GitHub OAuth flow**

```
GET  /api/v1/auth/github          → redirect to GitHub OAuth
GET  /api/v1/auth/github/callback → exchange code for token, store in github_tokens
GET  /api/v1/github/repos         → list user's repos (using stored token)
GET  /api/v1/github/repos/{owner}/{repo}/branches → list branches
```

**Step 2: Webhook registration**

When user connects a repo to a job:
1. Register a GitHub webhook on the repo: `POST /repos/{owner}/{repo}/hooks`
2. Webhook URL: `{ORBEX_URL}/api/v1/webhooks/github`
3. Events: `push`

**Step 3: Webhook receiver**

```
POST /api/v1/webhooks/github → receives push events
```

On push:
1. Identify job by repo + branch
2. Clone repo at commit SHA (via GitHub API tarball download, not git clone)
3. Store in MinIO
4. Enqueue build in build_queue

**Step 4: Frontend — GitHub repo selector**

Add "🐙 GitHub Repo" tile. Step 3 becomes:
1. "Connect GitHub" button (if no token) → starts OAuth
2. Repo dropdown (auto-populated)
3. Branch dropdown
4. Dockerfile path input (default: `./Dockerfile`)

**Step 5: Verify & commit**

```bash
go build ./...
git add -A && git commit -m "feat: add GitHub repo integration with OAuth and webhooks"
```

---

## Task 6: Docker Compose Support

**Files:**
- Create: `internal/compose/parser.go` (parse docker-compose.yml)
- Create: `internal/compose/orchestrator.go` (multi-container lifecycle)
- Modify: `internal/worker/worker.go` (compose execution path)
- Modify: `dashboard/src/app/dashboard/jobs/page.tsx` (add Compose source tile)

**Step 1: Compose YAML parser**

Parse `docker-compose.yml` into Go structs. Support:
- `services.*.image`
- `services.*.build` (context + dockerfile)
- `services.*.environment`
- `services.*.depends_on` (simple list)
- `services.*.command` / `entrypoint`
- `services.*.ports` (for inter-service networking)
- `services.*.volumes` (named volumes)

**Step 2: Compose orchestrator**

```go
type Orchestrator struct {
    docker  *docker.Client
    storage *storage.Client
}

func (o *Orchestrator) Run(ctx context.Context, composeFile []byte, jobID uuid.UUID) (exitCode int, logs string, err error)
```

Flow:
1. Parse compose YAML
2. Create Docker network for the job (`orbex-{jobID}`)
3. Build any services with `build:` directive
4. Start services in dependency order
5. Wait for main service to exit
6. Capture logs from all services
7. Tear down all containers + network

**Step 3: Main service detection**

1. Service with label `orbex.main: true`
2. First service without `depends_on`
3. First service

**Step 4: Wire into worker**

When `source_type = 'compose'`: download compose file from MinIO, pass to orchestrator.

**Step 5: Frontend — Compose tile + upload**

**Step 6: Verify & commit**

```bash
go build ./...
git add -A && git commit -m "feat: add docker-compose support with multi-container orchestration"
```

---

## Task 7: Frontend — Unified Source Type Selector

**Files:**
- Modify: `dashboard/src/app/dashboard/jobs/page.tsx` (full refactor of Step 2)
- Modify: `dashboard/src/lib/api.ts` (add all new API functions)

**Step 1: Refactor Step 2 to 6-tile source selector**

Replace the Docker/Script toggle with 6 tiles:
```
🐳 Docker Image    ✏️ Write Script    📁 Upload Files
📦 Dockerfile      🐙 GitHub Repo     🔧 Compose
```

Each tile drives different subsequent steps in the wizard.

**Step 2: Add file upload components**

Drag-and-drop zone, file list with sizes, delete buttons.

**Step 3: Add GitHub repo selector**

Connect button, repo dropdown, branch picker, Dockerfile path input.

**Step 4: Verify & commit**

```bash
npm run build  # verify no TS errors
git add -A && git commit -m "feat: unified source type selector in job creation wizard"
```
