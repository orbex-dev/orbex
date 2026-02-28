# Platform Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fill the remaining gaps between the startup plan MVP and the current codebase. Make Orbex into a sellable product.

**Architecture:** Go backend + Next.js frontend + PostgreSQL + Docker Engine API. Monolith. Session auth (cookies) for dashboard, API keys for CLI/programmatic access.

**Tech Stack:** Go 1.21+, Next.js 14, React, Tailwind CSS, PostgreSQL 16, Docker Engine API, Monaco Editor (for inline scripts)

---

## Updated Gap Assessment

After full audit, the codebase is more complete than initially estimated. The following pages already exist and are functional:
- ✅ `jobs/[id]/page.tsx` — inline editing, run overrides with env vars, webhook management, all trigger methods, run history table
- ✅ `runs/[id]/page.tsx` — status cards, log viewer with search/filter/download, auto-scroll, pause/resume/kill controls, timeout progress bar
- ✅ `jobs/page.tsx` — job list with search, filter chips, "New Job" wizard (6 steps), run button
- ✅ `dashboard/page.tsx` — overview with stats, recent runs, quick actions
- ✅ `settings/page.tsx` — API key generation, account info, password change form (frontend only)

**Remaining gaps (prioritized):**

---

### Task 1: Add Env Vars to Job Creation Wizard

**Files:**
- Modify: `dashboard/src/app/dashboard/jobs/page.tsx` (CreateJobWizard component)

**Why:** Backend accepts `env` in `CreateJobRequest`, Docker layer passes it through, job detail page already has env var editing — but the creation wizard skips it entirely. Users can't set env vars when creating a job.

**Step 1: Add `env` to form state**

In `CreateJobWizard`, update the form state (around line 34):

```tsx
const [form, setForm] = useState({
    name: '', image: 'alpine:latest', command: '',
    memory_mb: 512, cpu_millicores: 1000, timeout_seconds: 3600,
    schedule: '', customSchedule: '',
});
// ADD this:
const [envPairs, setEnvPairs] = useState<Array<{ key: string; value: string }>>([]);
```

**Step 2: Add env vars wizard step**

Update `stepTitles` to include 'Env Vars' between 'Entry Command' and 'Resources':

```tsx
const stepTitles = ['Name', 'Image', 'Entry Command', 'Env Vars', 'Resources', 'Schedule', 'Review'];
```

Add new step (between step 3 and current step 4, renumbering subsequent steps):

```tsx
{step === 4 && (
    <div>
        <div className="flex items-center justify-between mb-2">
            <label className="block text-sm font-medium text-zinc-300">
                Environment Variables <span className="text-zinc-500 font-normal">(optional)</span>
            </label>
            <button
                onClick={() => setEnvPairs([...envPairs, { key: '', value: '' }])}
                className="text-xs text-blue-400 hover:text-blue-300"
            >+ Add Variable</button>
        </div>
        <div className="space-y-2">
            {envPairs.map((pair, i) => (
                <div key={i} className="flex items-center gap-2">
                    <input
                        className="input input-mono text-sm flex-1"
                        placeholder="KEY"
                        value={pair.key}
                        onChange={e => {
                            const p = [...envPairs];
                            p[i].key = e.target.value;
                            setEnvPairs(p);
                        }}
                    />
                    <input
                        className="input input-mono text-sm flex-1"
                        placeholder="value"
                        value={pair.value}
                        onChange={e => {
                            const p = [...envPairs];
                            p[i].value = e.target.value;
                            setEnvPairs(p);
                        }}
                    />
                    <button
                        onClick={() => setEnvPairs(envPairs.filter((_, j) => j !== i))}
                        className="text-zinc-500 hover:text-red-400 text-sm"
                    >✕</button>
                </div>
            ))}
            {envPairs.length === 0 && (
                <p className="text-xs text-zinc-500">No env vars. Use these for secrets, config values, API keys etc.</p>
            )}
        </div>
    </div>
)}
```

**Step 3: Include env in API call**

In `handleCreate`, convert `envPairs` to a map and pass to the API:

```tsx
const env: Record<string, string> = {};
envPairs.forEach(p => { if (p.key) env[p.key] = p.value; });

await api.createJob({
    ...existingFields,
    env: Object.keys(env).length > 0 ? env : undefined,
});
```

**Step 4: Update Review step to show env vars**

Add env vars row to the review table.

**Step 5: Update all step number references**

All `step === N` conditions, `setStep(N)` calls, and `step < N` checks need to be incremented by 1 for steps after env vars.

**Step 6: Commit**

```bash
git add dashboard/src/app/dashboard/jobs/page.tsx
git commit -m "feat(dashboard): add env vars step to job creation wizard"
```

---

### Task 2: Password Change Backend

**Files:**
- Modify: `internal/api/handlers_auth.go`
- Modify: `internal/api/router.go`
- Modify: `dashboard/src/app/dashboard/settings/page.tsx`
- Modify: `dashboard/src/lib/api.ts`

**Step 1: Add ChangePassword handler**

In `handlers_auth.go`, add:

```go
// ChangePassword updates the authenticated user's password.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
    user := r.Context().Value("user").(*models.User)

    var req struct {
        CurrentPassword string `json:"current_password"`
        NewPassword     string `json:"new_password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request"})
        return
    }
    if req.CurrentPassword == "" || req.NewPassword == "" {
        writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "both current and new password required"})
        return
    }
    if len(req.NewPassword) < 8 {
        writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "new password must be at least 8 characters"})
        return
    }

    // Verify current password
    var storedHash string
    err := h.db.Pool.QueryRow(r.Context(), "SELECT password FROM users WHERE id = $1", user.ID).Scan(&storedHash)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to verify password"})
        return
    }
    if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.CurrentPassword)); err != nil {
        writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "current password is incorrect"})
        return
    }

    // Hash new password
    newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to hash password"})
        return
    }

    // Update
    _, err = h.db.Pool.Exec(r.Context(), "UPDATE users SET password = $1, updated_at = now() WHERE id = $2", string(newHash), user.ID)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update password"})
        return
    }

    writeJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}
```

**Step 2: Add route**

In `router.go`, in the protected routes group:

```go
r.Post("/auth/change-password", authHandler.ChangePassword)
```

**Step 3: Add API client method**

In `api.ts`, add to the `auth` object:

```typescript
changePassword: (currentPassword: string, newPassword: string) =>
    apiFetch<{ status: string }>('/auth/change-password', {
        method: 'POST',
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    }),
```

**Step 4: Connect settings page frontend**

In `settings/page.tsx`, update `handlePasswordReset` to call the real API:

```typescript
async function handlePasswordReset() {
    if (!currentPassword || !newPassword) return;
    if (newPassword.length < 8) {
        setToast({ message: 'New password must be at least 8 characters', type: 'error' });
        return;
    }
    try {
        await auth.changePassword(currentPassword, newPassword);
        setToast({ message: 'Password updated successfully', type: 'success' });
        setCurrentPassword('');
        setNewPassword('');
    } catch (err: any) {
        setToast({ message: err.message || 'Failed to update password', type: 'error' });
    }
}
```

**Step 5: Commit**

```bash
git add internal/api/handlers_auth.go internal/api/router.go dashboard/src/lib/api.ts dashboard/src/app/dashboard/settings/page.tsx
git commit -m "feat: add password change endpoint and connect frontend"
```

---

### Task 3: Inline Script Editor

**Files:**
- Modify: `internal/models/models.go` — add `Script` and `ScriptLang` fields to Job
- Create: `internal/database/migrations/006_job_script.sql` — add columns
- Modify: `internal/api/handlers_jobs.go` — handle script in create/update
- Modify: `internal/worker/worker.go` — mount script file into container
- Modify: `dashboard/src/lib/api.ts` — add script fields to types
- Modify: `dashboard/src/app/dashboard/jobs/page.tsx` — add script step to wizard
- Modify: `dashboard/src/app/dashboard/jobs/[id]/page.tsx` — add script tab

**Step 1: Database migration**

Create `internal/database/migrations/006_job_script.sql`:

```sql
-- Add inline script support to jobs
ALTER TABLE jobs ADD COLUMN script TEXT;
ALTER TABLE jobs ADD COLUMN script_lang TEXT;  -- 'python', 'node', 'bash', 'go', 'ruby'
```

**Step 2: Update models**

In `models.go`, add to `Job` struct:

```go
Script     *string `json:"script,omitempty"`
ScriptLang *string `json:"script_lang,omitempty"`
```

Add to `CreateJobRequest`:

```go
Script     *string `json:"script,omitempty"`
ScriptLang *string `json:"script_lang,omitempty"`
```

Add to `UpdateJobRequest`:

```go
Script     *string `json:"script,omitempty"`
ScriptLang *string `json:"script_lang,omitempty"`
```

**Step 3: Update job creation handler**

In `handlers_jobs.go` `Create`, add script fields to the INSERT query and include them in the job struct.

**Step 4: Update worker to mount script**

In `worker.go` `executeRun`, before creating the container:
- If `job.Script` is not nil, write it to a temp file
- Add bind mount: `/tmp/orbex-scripts/<runID>.ext` → `/orbex/script.ext`
- Override command to run the script using the appropriate runtime:
  - `python` → `["python", "/orbex/script.py"]`
  - `node` → `["node", "/orbex/script.js"]`
  - `bash` → `["bash", "/orbex/script.sh"]`
- Clean up temp file after container exits

**Step 5: Update Docker client for bind mounts**

In `docker.go`, add `Binds []string` to `ContainerConfig` and pass to `HostConfig.Binds`.

**Step 6: Update frontend types**

In `api.ts`, add `script?: string` and `script_lang?: string` to `Job`, `CreateJobRequest`, `UpdateJobRequest`.

**Step 7: Add script step to creation wizard**

Add a new step 3 (between Image and Entry Command) that:
- Offers toggle: "Bring your own image" vs "Write inline script"
- If inline script: show runtime picker + code editor textarea (Monaco is heavy, start with a styled textarea with monospace font)
- If inline script is chosen, auto-set the image based on language, hide the Command step

**Step 8: Add script editor to job detail page**

In `jobs/[id]/page.tsx`, add a "Script" section below Configuration:
- Shows script content in a code block with the language highlighted
- "Edit" button switches to textarea editing mode
- Save calls `updateJob` with the new script content

**Step 9: Commit**

```bash
git add internal/database/migrations/006_job_script.sql internal/models/models.go internal/api/handlers_jobs.go internal/worker/worker.go internal/docker/docker.go dashboard/src/lib/api.ts dashboard/src/app/dashboard/jobs/page.tsx dashboard/src/app/dashboard/jobs/\\[id\\]/page.tsx
git commit -m "feat: add inline script editor for casual users

Users can now write Python/Node/Bash scripts directly in the dashboard.
Scripts are mounted into the container at runtime via bind mount.
No Docker knowledge required for simple jobs."
```

---

### Task 4: Notification Webhooks

**Files:**
- Create: `internal/database/migrations/007_notifications.sql`
- Modify: `internal/models/models.go` — add NotificationConfig
- Create: `internal/worker/notify.go` — already exists, extend it
- Modify: `internal/api/handlers_jobs.go` — CRUD for notification config
- Modify: `internal/api/router.go` — add routes
- Modify: `dashboard/src/app/dashboard/jobs/[id]/page.tsx` — notification config UI

**Step 1: Database migration**

```sql
CREATE TABLE notification_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id      UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL DEFAULT 'webhook',  -- 'webhook' or 'email'
    url         TEXT,
    on_success  BOOLEAN NOT NULL DEFAULT true,
    on_failure  BOOLEAN NOT NULL DEFAULT true,
    on_pause    BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notification_configs_job_id ON notification_configs (job_id);
```

**Step 2: Add model and handlers**

Add CRUD handlers for notification configs attached to a job.

**Step 3: Send notifications from worker**

After job state transitions in `executeRun`, query notification configs and POST to webhook URLs.

**Step 4: Add notification config UI to job detail page**

Add a "Notifications" section with webhook URL input and toggles for success/failure/pause.

**Step 5: Commit**

---

### Task 5: Anomaly Detection

**Files:**
- Create: `internal/database/migrations/008_job_stats.sql`
- Modify: `internal/worker/worker.go` — update stats after each run
- Create: `internal/worker/anomaly.go` — detection logic
- Modify: `dashboard/src/app/dashboard/jobs/[id]/page.tsx` — show baseline info

**Step 1: Database migration**

```sql
CREATE TABLE job_stats (
    job_id           UUID PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    run_count        INT NOT NULL DEFAULT 0,
    avg_duration_ms  BIGINT NOT NULL DEFAULT 0,
    stddev_duration_ms BIGINT NOT NULL DEFAULT 0,
    min_duration_ms  BIGINT NOT NULL DEFAULT 0,
    max_duration_ms  BIGINT NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Step 2: Update stats after each completed run**

In `worker.go`, after a run completes successfully, update `job_stats` using Welford's online algorithm for running mean/stddev.

**Step 3: Check during run**

In the heartbeat loop, if `run_count >= 10` and current duration > `avg + 3 * stddev`, trigger anomaly notification.

**Step 4: Show in dashboard**

On job detail page, show "Usually completes in X-Y min" based on stats. Show "Building baseline (N/10 runs)" if < 10 runs.

**Step 5: Commit**

---

### Task 6: Auto-Kill Paused Containers

**Files:**
- Modify: `internal/worker/worker.go` — add cleanup goroutine
- Modify: `internal/worker/heartbeat.go` — check paused durations

**Step 1: Add paused container reaper**

New goroutine that runs every 60s:
- Query runs with `status = 'paused'`
- If `paused_at + max_pause_duration < now()`, kill the container
- Max pause duration: 24h (hardcoded for MVP, tier-based later)

**Step 2: Commit**

---

## Verification Plan

### For Each Task
1. Restart the Go backend after changes
2. Run the dashboard (`npm run dev`)
3. Browser test the new UI flow
4. Verify API calls work end-to-end

### Specific Tests
- **Task 1:** Create a job with env vars → run it → verify env vars appear in container logs
- **Task 2:** Change password → logout → login with new password
- **Task 3:** Write a Python script in the wizard → run → see output in logs
- **Task 4:** Set up webhook notification → run job → verify webhook receives POST
- **Task 5:** Run a job 10+ times → verify baseline shows → trigger long run → verify anomaly alert
- **Task 6:** Pause a job → wait (or set short timeout) → verify auto-kill
