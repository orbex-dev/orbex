# 🚀 Orbex — Startup Plan

> **Domain:** `orbex.dev`

> **One-liner:** We run your jobs like you would — except we never forget to check.

> **Tagline:** "Run anything. Know everything."

---

## Table of Contents

1. [The Problem](#the-problem)
2. [Our Solution](#our-solution)
3. [Core Differentiators](#core-differentiators)
4. [Target Customer](#target-customer)
5. [Competitive Landscape](#competitive-landscape)
6. [Product Architecture](#product-architecture)
7. [MVP Scope — 10-Week Build](#mvp-scope)
8. [Features Roadmap](#features-roadmap)
9. [Pricing Strategy](#pricing-strategy)
10. [Go-to-Market Strategy](#go-to-market-strategy)
11. [Open Source Strategy](#open-source-strategy)
12. [Development Tooling](#development-tooling)
13. [Infrastructure & Costs](#infrastructure--costs)
14. [Revenue Projections](#revenue-projections)
15. [Risks & Mitigations](#risks--mitigations)
16. [Team & Unfair Advantage](#team--unfair-advantage)
17. [Success Metrics](#success-metrics)

---

## The Problem

Background jobs are the plumbing of modern software — cron tasks, data pipelines, scraping, report generation, webhook processing, AI agent runs. Every developer has them. Nobody enjoys managing them.

**Current pain points (validated):**

| Pain Point | Evidence |
|---|---|
| Silent failures | Cron jobs fail at 3am, nobody knows until Monday |
| Lambda/Cloud Run kill jobs at timeout | 15min / 60min hard limits — no recovery, start over |
| No anomaly detection | A job that usually runs 20min suddenly runs 3 hours — no alert |
| Trigger.dev requires SDK lock-in | Must use their SDK baked into your code |
| SQS polling limited to 10 messages | Hard AWS limit, adds complexity for batch workloads |
| No cost tracking per job | "My bill went up 40% — which job caused it?" |
| Pricing unpredictability | Usage-based = surprise bills = dev anxiety |

**Market signals:**
- 76% of developers won't use AI for deployment/monitoring — they want proven reliability
- Cost of poor software quality: **$2.41 trillion annually**
- Only 29% of developers trust AI outputs — trust deficit = reliability opportunity

---

## Our Solution

A **container-first** managed platform for background jobs, cron tasks, and long-running processes.

**Container-first means:**
- You bring a Docker image. We run it.
- Any language, any framework, any runtime. We don't care.
- No SDK required. No vendor lock-in.
- If it runs in a container, it runs on us.

**What makes us different from "just another PaaS":**
1. **Pause, don't kill** — when jobs exceed limits, we freeze them. You inspect and resume.
2. **Anomaly detection** — we learn your job's normal behavior and flag when something's off.
3. **Guaranteed resolution** — every job ends in ✅ Succeeded, ❌ Failed, or ⏸️ Paused. No silent failures. Ever.
4. **Predictable pricing** — flat tiers with transparent overages. No surprise bills.

---

## Core Differentiators

### 1. Pause Instead of Kill ⏸️

When a job exceeds its time limit (user-set or platform default):

```
Job hits timeout
    │
    ├─ PAUSE the container (cgroup freezer)
    │   ├─ Process frozen in place — memory preserved
    │   ├─ User notified: "Job X paused after 45min"
    │   ├─ User can:
    │   │   ├─ VIEW logs (stdout/stderr captured)
    │   │   ├─ RESUME (instant — continues from exact point)
    │   │   └─ KILL (if they decide it's broken)
    │   └─ Auto-kill after 24h if no user action
    │
    └─ Future (v2): CHECKPOINT to disk via CRIU/Podman
        ├─ Saves full state to disk, frees server memory
        └─ Restore weeks later, even on different machine
```

**Technical implementation:** Linux cgroup freezer via `docker pause` — mature, stable, production-ready. No experimental features. This is Day 1 technology.

**Why nobody else does this:**

| Platform | At Timeout |
|---|---|
| AWS Lambda | ❌ Killed. Start over. |
| Cloud Run | ❌ Killed. Start over. |
| Trigger.dev | ❌ Failed. Retry from scratch. |
| Railway | ❌ Killed. |
| **Us** | ⏸️ **Paused. Inspect. Resume. Your choice.** |

### 2. Anomaly Detection 🔍

After ~10 runs, we know your job's baseline behavior:

```
📱 Notification:
"⚠️ Job: daily_report_generator
 Usually completes in 20-24 minutes.
 Currently running for 97 minutes (4× longer).

 What would you like to do?
   [✓ It's fine, let it run]
   [⏸ Pause and let me check]  
   [✕ Kill it]
   [🔕 Mute for this job]"
```

**Implementation:** Basic statistics (mean + standard deviation) on job duration history in Postgres. ~200 lines of code. No ML. Threshold: flag at 3× standard deviation from mean.

**Handles edge cases:**
- **First 10 runs:** Shows "Building baseline..." — no false alerts
- **Variable-duration jobs:** User can flag as "variable" → wider threshold
- **Seasonal patterns:** Tracks day-of-week and time-of-day baselines

### 3. Container-First (No SDK Lock-in) 📦

**Trigger.dev approach:**
```javascript
// You MUST use their SDK
import { task } from "@trigger.dev/sdk/v3";
export const myTask = task({
  id: "my-task",
  run: async (payload) => { /* your code */ },
});
```

**Our approach:**
```dockerfile
# Just a Dockerfile. Any language. Any framework.
FROM python:3.12
COPY . /app
CMD ["python", "run_job.py"]
```

```bash
# Push and schedule
$ orbex push ./Dockerfile --name daily-report
$ orbex schedule daily-report --cron "0 8 * * *"
```

The container IS the job. No SDK. No lock-in. Move to another platform by just running your Docker image elsewhere.

### 4. Guaranteed Resolution ✅

Every job ends in one of three states:

| State | Meaning | What We Do |
|---|---|---|
| ✅ **Succeeded** | Exit code 0 | Notify via webhook/email |
| ❌ **Failed** | Non-zero exit code | Notify + capture last 1000 lines of logs |
| ⏸️ **Paused** | Exceeded time limit | Notify + await user decision |

**No fourth state.** No "unknown." No "maybe it ran." No silent failures.

**Heartbeat system:**
- Running containers send heartbeats every 30s
- No heartbeat for 2min → marked as "infrastructure failure"
- Auto-retry on different runner → notify user

---

## Target Customer

### Primary: Indie Developers & Small Teams (1-10 people)

- Running cron jobs on VPS with crontab
- Managing Celery/Sidekiq/Bull workers themselves
- Frustrated by Lambda's 15-minute limit
- Want reliability without DevOps overhead
- Willing to pay ₹749-2,499/mo for peace of mind

### Secondary: AI Agent Builders

- Deploying long-running AI agents (CrewAI, LangGraph, AutoGen)
- Need >15 min execution time
- Want cost tracking per agent run
- Container-first = any AI framework works

### Tertiary: Data Teams at Startups

- ETL/ELT pipelines running on cron
- Scraping jobs that take variable time
- Need anomaly detection ("this scraper usually takes 20 min, why is it at 3 hours?")

---

## Competitive Landscape

```
                    SDK Required ◄──────────────────► Container-First
                         │                                    │
              Trigger.dev │                                    │ Us (here)
         (TypeScript SDK) │                                    │ (any container)
                          │                                    │
                          │           Railway                  │
       Kill at Timeout ───┤     (general PaaS)   ├─── Pause at Timeout
                          │                                    │
            Lambda, CR    │                                    │
         (hard limits)    │                                    │
                          │                                    │
```

### Direct Competitors

| Platform | Strengths | Our Advantage |
|---|---|---|
| **Trigger.dev** | $16M funding, TypeScript ecosystem, growing Python | Container-first (no SDK), pause not kill, anomaly detection |
| **Inngest** | Event-driven, multi-step workflows | Container-first, simpler model, predictable pricing |
| **AWS Lambda** | Massive scale, ecosystem | No 15-min limit, pause not kill, predictable pricing |
| **Cloud Run Jobs** | GCP integration, auto-scaling | Anomaly detection, pause not kill, simpler |
| **Railway** | Great DX, general PaaS | Purpose-built for jobs, anomaly detection, job-level cost tracking |

---

## Product Architecture

### System Design (MVP)

```
┌─────────────────────────────────────────────────────────┐
│                     User Interface                       │
│  ┌─────────┐  ┌─────────┐  ┌──────────────────────┐    │
│  │   CLI    │  │   API   │  │     Dashboard        │    │
│  │ (orbex)  │  │  (REST) │  │  (Next.js / React)   │    │
│  └────┬─────┘  └────┬────┘  └──────────┬───────────┘    │
│       │              │                  │                │
└───────┼──────────────┼──────────────────┼────────────────┘
        │              │                  │
        ▼              ▼                  ▼
┌─────────────────────────────────────────────────────────┐
│                    API Server (Go)                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │  Auth    │ │  Jobs    │ │ Schedule │ │ Billing  │   │
│  │  Module  │ │  Module  │ │  Module  │ │  Module  │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
│                        │                                 │
│              ┌─────────┼───────────┐                    │
│              ▼         ▼           ▼                    │
│  ┌──────────────┐ ┌────────┐ ┌──────────────┐          │
│  │  Scheduler   │ │ Queue  │ │   Anomaly    │          │
│  │ (Cron Engine)│ │(Postgres│ │  Detector    │          │
│  │              │ │SKIP    │ │ (Statistics) │          │
│  │              │ │LOCKED) │ │              │          │
│  └──────┬───────┘ └───┬────┘ └──────┬───────┘          │
│         │             │             │                    │
└─────────┼─────────────┼─────────────┼────────────────────┘
          │             │             │
          ▼             ▼             ▼
┌─────────────────────────────────────────────────────────┐
│                 Job Runner (Go)                           │
│  ┌──────────────────────────────────────────────────┐   │
│  │              Container Runtime                    │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐            │   │
│  │  │ Job A   │ │ Job B   │ │ Job C   │  (Docker)  │   │
│  │  │ Python  │ │ Node.js │ │ Go      │            │   │
│  │  └─────────┘ └─────────┘ └─────────┘            │   │
│  │                                                   │   │
│  │  Features:                                        │   │
│  │  - cgroup limits (CPU, memory)                    │   │
│  │  - Heartbeat every 30s                           │   │
│  │  - stdout/stderr capture                          │   │
│  │  - Pause/Resume via cgroup freezer               │   │
│  │  - Exit code capture                              │   │
│  └──────────────────────────────────────────────────┘   │
│                        │                                 │
└────────────────────────┼─────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│                PostgreSQL (Single Instance)               │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │  Users   │ │  Jobs    │ │ Job Runs │ │  Queue   │  │
│  │  & Auth  │ │  Defs    │ │ History  │ │ (SKIP    │  │
│  │          │ │          │ │ & Stats  │ │  LOCKED) │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
└─────────────────────────────────────────────────────────┘
```

### Tech Stack

| Component | Technology | Rationale |
|---|---|---|
| **API Server** | Go | Fast, single binary, great for systems programming |
| **Queue** | PostgreSQL (SKIP LOCKED) | 10-100K msg/sec, ACID, no additional infra |
| **Scheduler** | Custom Go (cron parser) | Lightweight, no external dependency |
| **Container Runtime** | Docker Engine API | Mature, well-documented, cgroup control |
| **Dashboard** | Next.js | Fast, SSR, great DX |
| **CLI** | Go (Cobra) | Single binary distribution, cross-platform |
| **Notifications** | Webhooks + Email (Resend) | Simple, reliable |
| **Database** | PostgreSQL 16 | One database for everything |
| **Monitoring** | Prometheus + Grafana (self-hosted) | Free, industry standard |

### Key Design Decisions

1. **Monolith, not microservices.** One Go binary + one Postgres instance. Ship fast.
2. **Docker, not Kubernetes.** K8s is overengineered for <1000 containers. Docker Compose on VPS.
3. **Postgres for everything.** Queue, job state, user data, billing, anomaly stats. One database.
4. **Go, not Rust.** Faster to ship, good enough performance, better library ecosystem for Docker/HTTP.
5. **Podman as future migration path.** When we need CRIU checkpoint-to-disk, switch the runtime.

---

## MVP Scope

### 10-Week Build Plan

**Weeks 1-2: Foundation**
- [ ] Go project scaffold with API server (Chi/Echo router)
- [ ] PostgreSQL schema: users, jobs, job_runs, queue
- [ ] Authentication (API keys + JWT for dashboard)
- [ ] Docker Engine API integration: create, start, stop, pause, unpause, logs
- [ ] Basic container resource limits (CPU, memory via cgroups)

**Weeks 3-4: Core Job Engine**
- [ ] Job definition format (YAML/JSON): image, command, env vars, limits
- [ ] Job queue with Postgres `SELECT ... FOR UPDATE SKIP LOCKED`
- [ ] Heartbeat system (container → API every 30s)
- [ ] Zombie detection (no heartbeat → mark failed → notify)
- [ ] stdout/stderr log capture and storage
- [ ] Exit code capture and job status resolution

**Weeks 5-6: Scheduling & Triggers**
- [ ] Cron scheduler (cron expression parser, timezone support)
- [ ] API trigger (POST `/api/v1/jobs/{id}/run`)
- [ ] Webhook trigger (receive POST → run job)
- [ ] Pause/Resume implementation via `docker pause` / `docker unpause`
- [ ] Timeout handling: configurable per job, default 1h, pause on exceed

**Weeks 7-8: Dashboard & Notifications**
- [ ] Next.js dashboard: job list, run history, live logs, status
- [ ] Webhook notification on job completion/failure/pause
- [ ] Email notification via Resend
- [ ] Job cost tracking (compute time × rate)
- [ ] Anomaly detection: duration baseline + flagging

**Weeks 9-10: CLI, Polish & Launch Prep**
- [ ] CLI tool (`orbex`): push image, create job, schedule, view logs, pause/resume
- [ ] Onboarding flow: signup → API key → push first job → see it run
- [ ] Documentation site (Docusaurus)
- [ ] Landing page
- [ ] Show HN draft
- [ ] Free tier setup: 5 jobs, 100 runs/month, 512MB memory, 30min max duration

### What's NOT in MVP (intentionally)

- ❌ Visual cron builder (Month 3)
- ❌ Slack/Discord/Telegram notifications (Month 3)
- ❌ Fan-out / job chaining / dependencies (Month 4)
- ❌ CRIU checkpoint-to-disk (Month 6)
- ❌ AI agent templates (Month 4)
- ❌ BYOV (Bring Your Own VPS) (Month 6+)
- ❌ Team management / RBAC (Month 4)
- ❌ Multi-region (Month 6+)

---

## Features Roadmap

### Phase 1: MVP Launch (Week 1-10)
Core engine + CLI + dashboard + pause/resume + anomaly detection

### Phase 2: Developer Love (Month 3-4)
- Visual cron builder (connected to execution)
- Slack/Discord/Telegram notifications
- Job chaining & dependencies
- Team management (invite, RBAC)
- AI agent templates (CrewAI, LangGraph starter images)
- GitHub Actions integration (trigger jobs from CI/CD)

### Phase 3: Scale (Month 5-8)
- CRIU checkpoint-to-disk (pause → free memory → resume later)
- Multi-runner (scale job runners horizontally)
- Custom domains for webhook triggers
- API rate tier upgrades
- Job priority levels

### Phase 4: Enterprise (Month 9-12)
- BYOV (Bring Your Own VPS) mode
- Multi-region (Mumbai + Singapore/US)
- SOC 2 compliance prep
- SSO / SAML
- Volume discounts & annual contracts
- SLA agreements (99.9% uptime guarantee)

---

## Pricing Strategy

### Core Philosophy
Predictable base + transparent overages. No surprise bills. Developers should know their bill before the month ends.

### Tiers (INR pricing, USD in parentheses)

| | **Free** | **Builder** | **Pro** | **Scale** |
|---|---|---|---|---|
| **Price** | ₹0 | ₹749/mo (~$9) | ₹2,499/mo (~$30) | ₹7,499/mo (~$90) |
| **Jobs** | 3 | 15 | 50 | Unlimited |
| **Runs/month** | 100 | 1,000 | 10,000 | 100,000 |
| **Max memory** | 256MB | 512MB | 2GB | 8GB |
| **Max duration** | 15 min | 1 hour | 4 hours | 24 hours |
| **Pause on timeout** | ❌ (kill) | ✅ | ✅ | ✅ |
| **Anomaly detection** | ❌ | ✅ | ✅ | ✅ |
| **Max paused time** | — | 1 hour | 6 hours | 24 hours |
| **Log retention** | 24 hours | 7 days | 30 days | 90 days |
| **Notifications** | Webhook only | Webhook + Email | + Slack/Discord | + Custom |
| **Support** | Community | Email (48h) | Email (24h) | Priority (4h) |

### Overage Pricing (transparent, per-unit)
- Extra run: ₹0.50 ($0.006)
- Extra compute minute: ₹0.25 ($0.003)
- Extra paused minute: ₹0.06 ($0.0007) — 25% of running rate
- Spending cap: User-configurable, hard stop when reached

### Why This Works
- **Free tier**: Brings developers in, lets them experience the product
- **Builder → Pro jump**: Pause on timeout + anomaly detection = conversion trigger
- **Predictable**: Monthly cap with optional overages (user controls the cap)
- **India pricing**: ₹749 is ~₹25/day — less than a coffee

---

## Go-to-Market Strategy

### Pre-Launch (Weeks 1-8): Build in Public
- **Week 1, Day 1**: First commit, first tweet. "Building a background job platform that pauses instead of killing."
- **Platform**: X (Twitter) — daily/every-other-day updates
- **Content**: Technical decisions, architecture diagrams, stumbling blocks, progress
- **Goal**: 200-500 followers by launch day

### Launch 1: Show HN (Week 10)
- **When**: Tuesday or Wednesday, 8-11 AM UTC (1:30-4:30 PM IST)
- **Title**: "Show HN: [Name] – Background jobs that pause instead of killing at timeout"
- **Post as builder**: "I built this because my cron jobs kept dying silently at 3am..."
- **Link to**: GitHub repo (open-source CLI) + live product (free tier, no credit card)
- **Have ready**: Documentation site, 5-minute getting started guide, demo video
- **Target**: #1-3 on Show HN, 5,000+ visitors, 200+ signups

### Post-Launch (Weeks 11-14): Community & Content
- **Blog posts**: 
  - "Why we pause containers instead of killing them" (technical deep dive)
  - "Anomaly detection for background jobs with just SQL" (tutorial)
  - "The hidden cost of silent cron failures" (problem-awareness)
- **Communities**: Post on dev.to, Indie Hackers, r/selfhosted, r/devops
- **Engagement**: Respond to every comment, every issue, every DM

### Launch 2: Product Hunt (Week 14-16)
- **Prep**: Hunter coordination, visual assets, video demo
- **Position**: "Developer tool" category
- **Goal**: Top 5 of the day

### Ongoing Growth
- **SEO**: Documentation-led. Target "background job best practices," "cron job monitoring," "docker job scheduler"
- **Open source**: CLI + container runtime on GitHub → community contributions → word of mouth
- **Integrations**: GitHub Actions, GitLab CI, Zapier → distribution through existing ecosystems

---

## Open Source Strategy

### What's Open Source (MIT License)

| Component | Why Open Source |
|---|---|
| **CLI (`ourctl`)** | Distribution — developers PIP/brew install it, discover the platform |
| **Container runtime agent** | Trust — users can audit what runs their code |
| **Job definition spec** | Standard — no vendor lock-in, community can build on it |
| **Documentation** | Contributions — community fixes and improves docs |

### What's Proprietary

| Component | Why Proprietary |
|---|---|
| **Scheduler engine** | Core IP — the brain of the platform |
| **Multi-tenancy & isolation** | Security — can't expose isolation logic |
| **Dashboard** | Revenue — this is what people pay for |
| **Billing & metering** | Business — revenue tracking |
| **Anomaly detection** | Differentiator — our unique sauce |
| **Auto-scaling logic** | Competitive advantage |

---

## Development Tooling

### AI Coding Tools

> **My founder decision:** AI tools are force multipliers for a solo developer. They turn 10 weeks into achievable. But overspending on AI subscriptions at pre-revenue is dumb. Pick one IDE tool and one API.

| Tool | Cost | Decision | Rationale |
|---|---|---|---|
| **Cursor Pro** | $20/mo | ❌ Skip | Credit-based pricing is unpredictable; can burn through credits on a long coding session |
| **GitHub Copilot Pro** | $10/mo | ✅ **Primary** | Best value — unlimited completions, 300 premium requests/mo, predictable flat rate |
| **Windsurf Pro** | $15/mo | ❌ Skip | Good but credits add up. Copilot is cheaper for equivalent features |
| **Claude Pro (Chat)** | $20/mo | ✅ **Secondary** | For architecture discussions, debugging complex issues, writing docs. Heavy usage expected during build phase — API would cost more |
| **Claude API** | Pay-per-use | ❌ Skip for now | At $3-15/MTok, a heavy development month could cost $50-200. Pro subscription caps cost at $20 |

**Total AI cost: $30/mo (₹2,500/mo)**

### Project Management

> **Founder decision:** A solo developer doesn't need Linear. But I need SOMETHING to track what's shipped and what's next. Keep it free.

| Tool | Cost | Decision | Rationale |
|---|---|---|---|
| **Linear** | $10/user/mo | ❌ Overkill | Great tool, not needed for 1 person |
| **Notion** | Free (personal) | ✅ **Knowledge base** | Free personal plan. Use for internal docs, meeting notes, ideas |
| **GitHub Projects** | Free | ✅ **Task tracking** | Free with GitHub. Kanban boards, issues, milestones. Integrated with the codebase |
| **Plane.so** | Free (self-hosted) | ⚠️ Fallback | Best open-source Linear alternative. Self-host if GitHub Projects feels limiting |

**Total PM cost: $0/mo**

### Documentation (User-Facing)

| Tool | Cost | Decision | Rationale |
|---|---|---|---|
| **Docusaurus** | Free (open-source) | ✅ **API/User docs** | Meta-backed, React-based, versioning, search, deploy to Cloudflare Pages for free |
| **Mintlify** | Free (hobby) | ❌ Skip | Beautiful but limited on free tier. Docusaurus gives us more control |
| **Outline Wiki** | Free (self-hosted) | ❌ Skip for Day 1 | Great internal wiki but adds infrastructure overhead. Use Notion instead |

**Total docs cost: $0/mo** (hosted on Cloudflare Pages — free)

### Other Tools

| Tool | Cost | Notes |
|---|---|---|
| **GitHub** | $0 | Free for public repos, free Actions minutes |
| **Figma** | $0 | Free starter plan for landing page design |
| **Excalidraw** | $0 | Architecture diagrams |
| **Resend** | $0 | 100 emails/day free — enough for first 6 months |
| **Cloudflare** | $0 | DNS, CDN, Pages hosting — all free tier |
| **Sentry** | $0 | Error tracking free tier (5K events/mo) |
| **Umami** | $0 | Self-hosted analytics (open-source) |

**Total other tools: $0/mo**

---

## Infrastructure & Costs

### Complete 12-Month Budget

#### Phase 1: Building (Month 1-3) — Pre-Revenue

| Item | Monthly (₹) | Monthly ($) | Notes |
|---|---|---|---|
| Platform server (E2E Networks C3.8GB) | 2,263 | 27 | 4vCPU, 8GB RAM, 100GB SSD — API, scheduler, dashboard |
| Job runner server (E2E Networks C3.8GB) | 2,263 | 27 | Separate server for container isolation |
| Domain | 83 | 1 | ₹1,000/year |
| AI tools (Copilot + Claude Pro) | 2,500 | 30 | Primary development tools |
| Email (Resend) | 0 | 0 | Free tier |
| Monitoring (Grafana Cloud) | 0 | 0 | Free tier |
| Error tracking (Sentry) | 0 | 0 | Free tier |
| DNS + CDN (Cloudflare) | 0 | 0 | Free tier |
| SSL (Let's Encrypt) | 0 | 0 | Free |
| Hosting docs (Cloudflare Pages) | 0 | 0 | Free |
| **Subtotal** | **~7,100** | **~$85** | |

#### Phase 2: Launch (Month 4-6) — Early Revenue

| Item | Monthly (₹) | Monthly ($) | Change |
|---|---|---|---|
| Everything from Phase 1 | 7,100 | 85 | Same |
| 2nd job runner (scaling) | 2,263 | 27 | When first runner hits 60% |
| Razorpay fees | ~350-500 | ~4-6 | 2% + GST on revenue (est. 20 customers) |
| **Subtotal** | **~9,700-9,900** | **~$115-120** | |

#### Phase 3: Growth (Month 7-12) — Scaling Revenue

| Item | Monthly (₹) | Monthly ($) | Change |
|---|---|---|---|
| Platform server upgrade | 4,000 | 48 | Upgrade to 16GB for more headroom |
| 3 job runners | 6,789 | 82 | Scale with customers |
| AI tools | 2,500 | 30 | Same |
| Domain | 83 | 1 | Same |
| Managed Postgres (optional) | 2,000 | 24 | If self-hosted becomes risky |
| Email upgrade (Resend) | 1,500 | 18 | If 500+ notifications/day |
| Razorpay fees | 2,000-5,000 | 24-60 | Revenue-dependent |
| Company registration (one-time, amortized) | 833 | 10 | ₹10,000 total |
| **Subtotal** | **~19,700-22,700** | **~$237-$273** | |

### 12-Month Total

| Period | Months | Monthly Avg (₹) | Subtotal (₹) | Subtotal ($) |
|---|---|---|---|---|
| Phase 1 (build) | 3 | 7,100 | 21,300 | 255 |
| Phase 2 (launch) | 3 | 9,800 | 29,400 | 354 |
| Phase 3 (growth) | 6 | 21,200 | 1,27,200 | 1,530 |
| **Total Year 1** | **12** | **~14,800** | **~1,77,900** | **~$2,139** |

> **Year 1 total with AI tools: ~₹1,78,000 (~$2,140).** That's ₹14,800/month average (~$178/mo). Extremely lean.

### Budget-Conscious Alternative (Vyom Cloud)

| | E2E Networks | Vyom Cloud | Savings |
|---|---|---|---|
| Per server | ₹2,263/mo | ₹360-700/mo | 60-85% |
| Year 1 total | ~₹1,78,000 | ~₹1,10,000 | ~₹68,000 saved |
| Year 1 total ($) | ~$2,140 | ~$1,320 | ~$820 saved |

---

## Revenue Projections

### Conservative Scenario

| Month | Free Users | Paid Users | MRR (₹) | MRR ($) |
|---|---|---|---|---|
| Month 4 (launch) | 50 | 5 | 3,745 | 45 |
| Month 5 | 100 | 12 | 8,988 | 108 |
| Month 6 | 200 | 25 | 18,725 | 225 |
| Month 8 | 400 | 50 | 37,450 | 450 |
| Month 10 | 700 | 80 | 59,920 | 720 |
| Month 12 | 1,000 | 120 | 89,880 | 1,080 |

*Assumes: 80% Builder tier (₹749), 15% Pro (₹2,499), 5% Scale (₹7,499). Blended ARPU: ₹749.*

### Break-Even Analysis

| Scenario | Break-Even Point |
|---|---|
| Phase 1 costs (₹7,100/mo) | **10 Builder customers** |
| Phase 2 costs (₹9,800/mo) | **14 Builder customers** |
| Phase 3 costs (₹21,200/mo) | **29 Builder customers** OR **9 Pro customers** |
| Including personal salary (₹50,000/mo) | **67 Builder customers** OR **21 Pro customers** |

---

## Risks & Mitigations

### Technical Risks

| Risk | Severity | Mitigation |
|---|---|---|
| **Container escape (security)** | 🔴 High | Unprivileged containers, no `--privileged`, seccomp profiles, network isolation per tenant (separate Docker networks) |
| **Noisy neighbor** | 🟡 Medium | cgroup limits enforced per container. Hard CPU + memory caps. |
| **Paused containers eating memory** | 🟡 Medium | Max pause duration (tier-based), max paused containers per user (5), auto-kill at limit |
| **Postgres as queue at scale** | 🟢 Low (for now) | SKIP LOCKED handles 10-100K msg/sec. Migration path to RabbitMQ documented. Not needed until 1000+ concurrent jobs |
| **Data loss** | 🔴 High | Daily Postgres backups to S3/object storage. WAL archiving. Test restores monthly. |

### Business Risks

| Risk | Severity | Mitigation |
|---|---|---|
| **Trigger.dev adds container support** | 🟡 Medium | They're SDK-first — their architecture is built around it. Container-first is a fundamental design choice, not a feature to bolt on |
| **Too slow to get to 100 users** | 🟡 Medium | Build in public creates accountability. Show HN is a forcing function. If no traction by Month 6, pivot to India-specific (rupee pricing, local VPS) |
| **Price too low to sustain** | 🟡 Medium | ₹749 covers infra at 15 customers. Pro tier at ₹2,499 has 4x margins. Margins improve with density (more jobs per server) |
| **Solo founder burnout** | 🔴 High | Sustainable pace: 6-8 hours/day, 6 days/week. AI tools reduce grunt work significantly. Take Sundays off. Health > features. |

### "Guaranteed Resolution" Legal/SLA Risk

| Risk | Mitigation |
|---|---|
| User claims we "guaranteed" their job would succeed | Framing: "Every job resolves — succeed, fail, or pause. No unknowns." We guarantee **awareness**, not **success**. Terms of service explicitly state this. |
| Job stuck in paused state, user blames us | Auto-kill at max pause duration. Clear notification with 3 options. User explicitly chooses what happens. |
| Infrastructure failure during job | Heartbeat detection → auto-retry → notification. If retry also fails → refund compute credits for that run |

---

## Team & Unfair Advantage

### Founder Profile
- **Background job experience**: Has built and managed background job systems. Understands the pain firsthand.
- **VPS resourcefulness**: Can arrange infrastructure at Indian prices. Knows the local cloud landscape.
- **Grind commitment**: Willing to do 2am support. This matters in early days when reliability IS the product.
- **AI-leverage**: Using AI coding tools to ship at 3-5x solo developer speed.

### Why This Founder Wins Here
1. **Lives the problem** — not building from theory, building from frustration
2. **Cost advantage** — Indian infrastructure + solo founder = very low burn rate
3. **Speed** — AI-augmented development + no bureaucracy = fast iteration
4. **Community focus** — willing to spend time in forums, DMs, and calls that funded startups deprioritize

---

## Success Metrics

### Month 3 (End of Build)
- [ ] Working product: push container, schedule, run, pause, resume, dashboard
- [ ] Documentation site live
- [ ] 3 beta users running real jobs

### Month 6 (End of Early Phase)
- [ ] Show HN launched
- [ ] 200+ registered users
- [ ] 25+ paying customers
- [ ] <2 minutes to "first job running" (onboarding)
- [ ] 99.5% job resolution rate (no unknowns)

### Month 12 (End of Year 1)
- [ ] 1,000+ registered users
- [ ] 100+ paying customers
- [ ] MRR: ₹75,000+ (~$900+)
- [ ] Cash-flow positive on infrastructure
- [ ] Product Hunt launch completed
- [ ] Anomaly detection live for all paying users
- [ ] 0 "silent failure" incidents

### North Star Metric
**Jobs resolved / Jobs attempted = 100%.** Every job that enters our system exits with a known state. No unknowns. Ever.

---

## What's Not in This Plan (Intentionally)

- **Mobile app** — developers don't manage jobs from phones
- **Windows support** — Linux containers only, Docker Desktop handles Windows devs
- **GraphQL API** — REST is simpler, faster to build, good enough
- **Real-time collaboration** — solo developer product first
- **Marketplace / plugin system** — premature, adds complexity before PMF
- **VC fundraising** — not needed at this burn rate. Bootstrapped until revenue proves the model.

---

*Last updated: February 19, 2025*
*Status: Pre-build. All assumptions research-validated.*
