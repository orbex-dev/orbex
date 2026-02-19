# Orbex

> Run anything. Know everything.

**We run your jobs like you would — except we never forget to check.**

Orbex is a container-first background job platform. Push a Docker image, schedule it, and we handle the rest — with pause instead of kill, anomaly detection, and guaranteed resolution for every job.

---

## Why Orbex?

- **🐳 Container-first** — Any language, any framework. Bring a Dockerfile, not an SDK.
- **⏸️ Pause, don't kill** — Jobs that exceed their time limit are frozen, not terminated. Inspect, resume, or kill — your choice.
- **🔍 Anomaly detection** — We learn your job's baseline and flag when something's off.
- **✅ Guaranteed resolution** — Every job ends as Succeeded, Failed, or Paused. No silent failures. Ever.
- **💰 Predictable pricing** — Flat tiers, no surprise bills.

## Quick Start

```bash
# Push your container
$ orbex push ./Dockerfile --name daily-report

# Schedule it
$ orbex schedule daily-report --cron "0 8 * * *"

# Check status
$ orbex status daily-report
```

## Status

🚧 **Building in public.** Follow along:

- **Website:** [orbex.dev](https://orbex.dev)
- **Twitter/X:** [@orbexdev](https://x.com/orbexdev)

## License

MIT — see [LICENSE](LICENSE) for details.
