# Orbex Platform Gap Analysis & Design

> Cross-reference of `startup_plan.md` MVP scope vs. actual codebase state.  
> Target user: **Option C** — power users bring Docker images, casual users write inline scripts.

---

## 1. MVP Status: What Exists vs. What's Missing

### ✅ What's Built & Working

| Feature | Backend | Frontend | Notes |
|---|---|---|---|
| User registration/login | ✅ | ✅ | Session-based (httpOnly cookies) |
| API key auth | ✅ | ✅ | Bearer token for CLI/API |
| Job CRUD | ✅ | ✅ | Create, list, get, update, delete |
| Docker container execution | ✅ | — | Pull, create, start, wait, logs |
| Container resource limits | ✅ | ✅ | CPU (cgroup), memory, timeout |
| Cron scheduler | ✅ | ✅ | Cron expression parsing, auto-enqueue |
| Job queue (Postgres SKIP LOCKED) | ✅ | — | Worker polls and claims jobs |
| Pause/Resume/Kill | ✅ | ✅ | `docker pause`/`unpause`/`stop` |
| Webhook triggers | ✅ | ✅ | Token-based, no auth needed |
| Run history & logs | ✅ | ✅ | Stored in DB, viewable in dashboard |
| Heartbeat system | ✅ | — | 30s heartbeat, zombie detection |
| Landing page | — | ✅ | Marketing page at `/` |
| Dashboard (overview, jobs, settings) | — | ✅ | Functional but needs polish |

### ❌ Critical Gaps (Blocking "Sellable Product")

| # | Gap | Startup Plan Reference | Impact |
|---|---|---|---|
| **G1** | **No env vars in job creation UI** | "env vars" in job definition format (Week 3-4) | Users can't configure jobs properly |
| **G2** | **No script/code delivery mechanism** | "You bring a Docker image. We run it." + option C (inline scripts) | Users with no Docker image can't use the platform |
| **G3** | **No inline script editor** | Option C requirement | Casual users have no way to run code |
| **G4** | **No `orbex push` / image registry flow** | "Push and schedule" CLI flow (Week 9-10) | Power users can't push custom images |
| **G5** | **No job detail page** | Dashboard (Week 7-8) | Can't view individual job config, edit, or see run history |
| **G6** | **No notification system** | Webhook/email notification on completion (Week 7-8) | Silent failures — the exact problem we're solving |
| **G7** | **No anomaly detection** | Core differentiator #2 (Week 7-8) | Missing key selling point |
| **G8** | **No cost/usage tracking** | Job cost tracking (Week 7-8) | Can't implement pricing tiers |
| **G9** | **No CLI tool** | `orbex` CLI (Week 9-10) | No programmatic/terminal interface |
| **G10** | **No onboarding flow** | "signup → API key → push first job → see it run" (Week 9-10) | New users are lost |
| **G11** | **No password change backend** | Settings page has frontend form but no backend endpoint | Security gap |
| **G12** | **Auto-kill timer on paused containers missing** | "Auto-kill after 24h if no user action" | Paused containers can stay forever |

### ⚠️ UX/Polish Gaps (Not Blocking but Hurting)

| # | Gap | Impact |
|---|---|---|
| **U1** | Job creation wizard doesn't collect env vars | Can't pass secrets/config to containers |
| **U2** | No run details page — inline expand only | Hard to debug failed jobs |
| **U3** | No real-time log streaming | Must refresh to see job progress |
| **U4** | No "duplicate job" action | Common task requires manual recreation |
| **U5** | No job editing after creation (UI) | Backend PATCH exists, no frontend exposure |
| **U6** | Dashboard overview stats are basic | No graphs, trends, or anomaly indicators |
| **U7** | No confirmation dialogs for destructive actions | Delete job, kill run — no safety net |
| **U8** | No timezone display for schedules | Users don't know which TZ cron uses |
| **U9** | No search within run logs | Debugging large output is painful |

---

## 2. Proposed Approach: Phased Implementation

### Phase A: "Actually Usable" (Critical Path)

> Goal: A developer can sign up, create a job, run it, and get results — the minimum loop.

1. **Env vars in job creation** — add wizard step + pass to API (backend already supports it)
2. **Inline script editor** — for Option C casual users: pick a runtime (Python/Node/Bash), write code in a Monaco editor, we wrap it in the selected base image at run time
3. **Job detail page** — view config, edit job, see run history with logs, pause/resume/kill controls
4. **Notification webhooks** — on job completion/failure/pause, POST to user-configured URL
5. **Password change backend** — complete the settings page flow

### Phase B: "Worth Paying For" (Differentiators)

> Goal: The features that make Orbex different from running `docker run` yourself.

6. **Anomaly detection** — track duration stats per job, flag outliers at 3σ
7. **Auto-kill paused containers** — tier-based max pause duration
8. **Cost/usage metering** — track compute minutes per job, display in dashboard
9. **Email notifications** — via Resend API on job state changes
10. **Onboarding flow** — first-run wizard: create account → create first job → see it run

### Phase C: "Ready for Launch" (CLI + Polish)

> Goal: Show HN ready — CLI, docs, landing page polish.

11. **CLI tool (`orbex`)** — push image, create job, schedule, logs, pause/resume
12. **Real-time log streaming** — WebSocket or SSE for live log tailing
13. **Dashboard polish** — graphs, trends, proper empty states, confirmation dialogs

---

## 3. Design: Inline Script Editor (G2 + G3)

This is the biggest new feature. Three approaches considered:

### Approach A: Build Script Into Image at Runtime ⭐ RECOMMENDED

**How it works:**
1. User selects runtime (Python 3.12 / Node 22 / Bash / Go / Ruby)
2. User writes script in Monaco editor in the dashboard
3. On "Run", backend:
   - Stores script content in DB (new `script` column on `jobs` table)
   - At execution time, worker creates a temp file, mounts it into container via Docker bind mount
   - Container runs: `python /orbex/script.py` (or `node /orbex/script.js`, etc.)

**Pros:**
- No image build step — instant execution
- Script editable without rebuilding anything
- Works with existing Docker infrastructure
- Power users can still bring their own image (script field is optional)

**Cons:**
- Limited to single-file scripts (multi-file needs an image)
- Can't install custom dependencies (unless we support a `requirements.txt` equivalent)

### Approach B: Build Custom Image On Submit

Server builds a custom Docker image per job (inject script, install deps). Slow (30-60s build time), needs image registry.

### Approach C: Just Document "Bring Your Own Image"

No inline editor. Users must build and push Docker images. Simpler but loses casual users entirely.

**Recommendation: Approach A** — fast, simple, covers 80% of use cases. Power users still bring their own images. We add a `requirements.txt` / `package.json` install step later as enhancement.

---

## 4. Design: Env Vars in Job Creation (G1)

Simple addition:
- New wizard step between "Container Command" and "Resources"
- Key-value editor: add/remove rows, key input + value input (with show/hide toggle for secrets)
- Stored in existing `env JSONB` column in DB (already supported)
- Passed through existing `ContainerConfig.Env` in Docker layer (already works)

---

## 5. Design: Job Detail Page (G5)

New page at `/dashboard/jobs/[id]`:
- **Header**: Job name, status badge, image, created date, quick actions (Run, Edit, Delete)
- **Tabs**:
  - **Runs**: Table of all runs with status, duration, exit code, started_at. Click to expand inline logs.
  - **Configuration**: View/edit job config (image, command, env, resources, schedule)
  - **Webhooks**: Generate/view webhook URL, test trigger
  - **Script** (if inline script job): Monaco editor to view/edit the script
- **Run detail panel**: Click a run → slide-out with full logs, resource usage, timeline

---

## 6. Design: Notification System (G6)

Phase 1 (webhook notifications):
- New `notification_config` table: `job_id`, `type` (webhook/email), `url`/`email`, `on_success`, `on_failure`, `on_pause`
- Worker calls notification URLs after job state transitions
- Dashboard: notification config section in job detail page

Phase 2 (email via Resend):
- Resend API integration
- Templates for: job succeeded, job failed, job paused (with action buttons)

---

## 7. Design: Anomaly Detection (G7)

Per startup plan — basic statistics, no ML:
- New `job_stats` table: `job_id`, `run_count`, `avg_duration_ms`, `stddev_duration_ms`, `min_duration_ms`, `max_duration_ms`
- Updated after each completed run
- During run: if duration > mean + 3σ, trigger anomaly notification
- Dashboard: show baseline indicator on job detail page ("Usually completes in 20-24 min")
- First 10 runs: "Building baseline..." — no alerts

---

## Priority Order

| Priority | Item | Effort | Value |
|---|---|---|---|
| 🔴 P0 | Env vars in wizard | Small (2-3h) | Unblocks real job config |
| 🔴 P0 | Job detail page | Medium (6-8h) | Core navigation missing |
| 🔴 P0 | Password change backend | Small (1-2h) | Security gap |
| 🟠 P1 | Inline script editor | Medium (6-8h) | Unlocks casual users |
| 🟠 P1 | Notification webhooks | Medium (4-6h) | Core differentiator promise |
| 🟠 P1 | Job editing UI | Small (2-3h) | Backend exists, no UI |
| 🟡 P2 | Anomaly detection | Medium (4-6h) | Key selling point |
| 🟡 P2 | Auto-kill paused containers | Small (2-3h) | Safety feature |
| 🟡 P2 | Cost/usage tracking | Medium (4-6h) | Needed for pricing |
| 🟢 P3 | CLI tool | Large (10-15h) | Show HN requirement |
| 🟢 P3 | Real-time log streaming | Medium (4-6h) | Polish |
| 🟢 P3 | Email notifications | Small (2-3h) | Enhanced notifications |
| 🟢 P3 | Onboarding flow | Medium (4-6h) | First-run experience |

---

*Generated from full codebase audit + startup_plan.md cross-reference.*
