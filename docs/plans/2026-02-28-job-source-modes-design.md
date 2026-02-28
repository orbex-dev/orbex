# Job Source Modes — Design Document

## Problem

Orbex currently supports two ways to define what a job runs: pulling a pre-built Docker image, or writing an inline script. Both are limiting:

- **Docker Image mode** requires users to maintain their own image pipeline
- **Inline Script mode** only works for small, single-file scripts

Users need to bring their own code — files, Dockerfiles, repos — and have Orbex handle the rest.

## Solution

Introduce a unified **source type** model with five modes:

| Source Type | `source_type` | User Provides | Orbex Does |
|---|---|---|---|
| Docker Image | `image` | Image name | Pull & run |
| Inline Script | `script` | Code in browser | Mount script, run in base image |
| File Upload | `upload` | Files (zip/tarball/individual) | Store in MinIO, mount at runtime |
| Dockerfile | `dockerfile` | Dockerfile + context | Build image, run |
| GitHub Repo | `github` | Repo URL + branch | Clone, build on push, run on schedule |

Additionally, **docker-compose.yml** support allows multi-service definitions — Orbex parses the compose file and orchestrates linked containers.

---

## Architecture Decisions

| Decision | Choice | Rationale |
|---|---|---|
| File storage | MinIO (S3-compatible) | Multi-VPS scalable; swap to AWS S3 later |
| Build execution | Local Docker API | Already connected; build queue limits concurrency |
| Git provider | GitHub only (v1) | Best webhook/API ecosystem; GitLab later |
| Git trigger | Auto-build on push, manual/scheduled run | Fresh images without unwanted executions |
| Compose support | Parse & orchestrate | Differentiating feature; handles multi-service |
| Server modes | `ORBEX_MODE=all\|worker\|builder` | Future multi-VPS separation |

---

## Detailed Design

### 1. MinIO Integration

Deploy MinIO alongside Postgres (docker-compose or standalone). Configure via env vars:

```
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=orbex
MINIO_SECRET_KEY=orbexsecret
MINIO_BUCKET=orbex-storage
```

Bucket structure:
```
orbex-storage/
├── uploads/{user_id}/{job_id}/          # File uploads
├── builds/{user_id}/{job_id}/           # Dockerfile + context
├── repos/{user_id}/{job_id}/{commit}/   # Cloned repo snapshots
```

New package: `internal/storage/minio.go` — wraps MinIO client with `Upload`, `Download`, `List`, `Delete`.

### 2. File Upload Source

**Flow:**
1. User selects base image (or auto-detect from file extensions)
2. Uploads files via multipart form POST
3. Backend stores in MinIO `uploads/{user_id}/{job_id}/`
4. At runtime: worker downloads files from MinIO → writes to temp dir → bind-mounts into container at `/orbex/workspace/`
5. Entry command runs against the workspace

**Auto-detection rules:**
- `.py` → `python:3.12-slim`
- `.js` / `.ts` → `node:22-slim`
- `.go` → `golang:1.22`
- `.rb` → `ruby:3.3-slim`
- `.sh` → `alpine:latest`
- Mixed / unknown → user must select

**API:**
- `POST /api/v1/jobs/{jobID}/upload` — multipart file upload (max 50MB)
- `GET /api/v1/jobs/{jobID}/files` — list uploaded files
- `DELETE /api/v1/jobs/{jobID}/files/{filename}` — remove a file

### 3. Dockerfile Build Source

**Flow:**
1. User uploads Dockerfile + context files (requirements.txt, etc.)
2. Backend stores in MinIO `builds/{user_id}/{job_id}/`
3. Enqueues a build job in `build_queue` table
4. Build worker picks up job, downloads context from MinIO
5. Runs `docker build -t orbex/{job_id}:latest .`
6. Job runs use the built image

**Build Queue:**
```sql
CREATE TABLE build_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id),
    user_id UUID NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'pending',  -- pending, building, succeeded, failed
    image_tag TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);
```

Concurrency: max 2-3 concurrent builds per server (configurable via `ORBEX_MAX_BUILDS`).

### 4. Docker Compose Support

**Flow:**
1. User uploads `docker-compose.yml` (+ associated Dockerfiles/context)
2. Orbex parses the compose file to identify services
3. For each service: resolves `image:` or `build:` directive
4. Creates a Docker network for the job
5. Starts all services in dependency order (`depends_on`)
6. Monitors the "main" service (marked via label or first service without `depends_on`)
7. When main service exits, captures exit code, tears down all services

**Compose parsing — what we support (v1):**
- `image` — pull named image
- `build` — build from Dockerfile
- `environment` — env vars
- `depends_on` — startup ordering (simple, not health-check based)
- `ports` — internal service discovery (not exposed to host)
- `volumes` — named volumes between services
- `command` / `entrypoint` — override

**What we don't support (v1):** `networks` (custom), `configs`, `secrets`, `deploy`, `profiles`.

**Main service detection:**
1. Service with label `orbex.main: true`
2. Else: first service in the file without `depends_on`
3. Else: first service in the file

### 5. GitHub Repo Integration

**Authentication:** GitHub OAuth App
- User clicks "Connect GitHub" → OAuth flow → access token stored encrypted in DB
- Env vars: `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`

**Flow:**
1. User authorizes Orbex via GitHub OAuth
2. Orbex lists user's repos (API call with token)
3. User selects repo + branch + Dockerfile path (default: `./Dockerfile`)
4. Orbex registers a push webhook on the repo
5. On push: webhook hits `POST /api/v1/webhooks/github`
6. Orbex clones repo at commit SHA → stores in MinIO → enqueues build
7. Build worker builds image → tags as `orbex/{job_id}:{commit_sha}`
8. Job's scheduled runs automatically use the latest built image

**Database additions:**
```sql
ALTER TABLE jobs ADD COLUMN github_repo TEXT;       -- owner/repo
ALTER TABLE jobs ADD COLUMN github_branch TEXT;      -- default: main
ALTER TABLE jobs ADD COLUMN github_token_id UUID;    -- references github_tokens table
ALTER TABLE jobs ADD COLUMN dockerfile_path TEXT;    -- default: ./Dockerfile

CREATE TABLE github_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    access_token TEXT NOT NULL,  -- encrypted
    github_username TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

### 6. Database Changes

**Modify `jobs` table:**
```sql
ALTER TABLE jobs ADD COLUMN source_type TEXT NOT NULL DEFAULT 'image';
-- Values: 'image', 'script', 'upload', 'dockerfile', 'github', 'compose'
```

**New tables:** `build_queue`, `github_tokens` (as above).

### 7. Frontend — Unified Job Creation Wizard

Step 2 of the wizard becomes a **source type selector**:

```
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ 🐳 Docker   │ │ ✏️ Script   │ │ 📁 Upload   │
│   Image     │ │             │ │   Files     │
└─────────────┘ └─────────────┘ └─────────────┘
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ 📦 Docker   │ │ 🐙 GitHub   │ │ 🔧 Compose  │
│   file      │ │   Repo      │ │             │
└─────────────┘ └─────────────┘ └─────────────┘
```

Each source type drives a different Step 3:
- **Image**: image picker (current)
- **Script**: runtime picker + code editor (current)
- **Upload**: file dropzone + optional image override
- **Dockerfile**: file dropzone for Dockerfile + context
- **GitHub**: repo selector + branch + Dockerfile path
- **Compose**: file dropzone for docker-compose.yml + context

---

## Implementation Priority

1. **MinIO integration** — foundation for everything
2. **Source type model** — DB migration + API changes
3. **File upload** — simplest new source mode
4. **Dockerfile build** — build queue + Docker build API
5. **GitHub repo** — OAuth + webhooks + clone + build
6. **Docker Compose** — parsing + multi-container orchestration

---

## Verification Plan

- Unit tests for compose YAML parsing
- Integration test: upload file → run job → check logs
- Integration test: Dockerfile build → run → check logs
- Manual test: GitHub OAuth flow → connect repo → push → verify build
- Manual test: docker-compose.yml → multi-service startup → main service exit
