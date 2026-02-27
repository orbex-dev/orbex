'use client';

export default function LandingPage() {
    return (
        <div className="min-h-screen bg-zinc-950 text-zinc-100">
            {/* Nav */}
            <nav className="border-b border-zinc-800/50 backdrop-blur-sm bg-zinc-950/80 fixed top-0 w-full z-50">
                <div className="max-w-6xl mx-auto px-6 py-4 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <span className="text-2xl font-bold bg-gradient-to-r from-violet-400 to-cyan-400 bg-clip-text text-transparent">
                            orbex
                        </span>
                        <span className="text-xs text-zinc-500 font-mono">.dev</span>
                    </div>
                    <div className="flex items-center gap-6">
                        <a href="https://github.com/orbex-dev/orbex" className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors">GitHub</a>
                        <a href="/docs" className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors">Docs</a>
                        <a href="/dashboard" className="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm font-medium rounded-lg transition-colors">
                            Dashboard →
                        </a>
                    </div>
                </div>
            </nav>

            {/* Hero */}
            <section className="pt-32 pb-20 px-6">
                <div className="max-w-4xl mx-auto text-center">
                    <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-violet-500/10 border border-violet-500/20 text-violet-400 text-xs font-medium mb-8">
                        <span className="w-2 h-2 bg-violet-400 rounded-full animate-pulse" />
                        Now in public beta
                    </div>
                    <h1 className="text-5xl md:text-7xl font-bold leading-tight tracking-tight">
                        Run anything.
                        <br />
                        <span className="bg-gradient-to-r from-violet-400 via-purple-400 to-cyan-400 bg-clip-text text-transparent">
                            Know everything.
                        </span>
                    </h1>
                    <p className="text-lg text-zinc-400 mt-6 max-w-2xl mx-auto leading-relaxed">
                        Orbex is a lightweight job orchestration platform that runs Docker containers
                        with scheduling, monitoring, and real-time observability. Built for teams that move fast.
                    </p>
                    <div className="flex items-center justify-center gap-4 mt-10">
                        <a href="/dashboard" className="px-6 py-3 bg-violet-600 hover:bg-violet-500 text-white font-medium rounded-xl transition-all hover:shadow-lg hover:shadow-violet-500/20">
                            Get Started Free
                        </a>
                        <a href="https://github.com/orbex-dev/orbex" className="px-6 py-3 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 font-medium rounded-xl transition-colors border border-zinc-700">
                            View on GitHub
                        </a>
                    </div>

                    {/* Terminal demo */}
                    <div className="mt-16 bg-zinc-900 border border-zinc-800 rounded-2xl p-1 max-w-2xl mx-auto shadow-2xl">
                        <div className="flex items-center gap-2 px-4 py-3 border-b border-zinc-800">
                            <div className="w-3 h-3 rounded-full bg-red-500/80" />
                            <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
                            <div className="w-3 h-3 rounded-full bg-green-500/80" />
                            <span className="text-xs text-zinc-500 ml-2 font-mono">terminal</span>
                        </div>
                        <div className="p-5 font-mono text-sm text-left space-y-2">
                            <div><span className="text-violet-400">$</span> <span className="text-zinc-300">orbex jobs create --name etl-pipeline --image python:3.12</span></div>
                            <div className="text-emerald-400">✓ Created job: etl-pipeline (a3b8d1b6)</div>
                            <div className="mt-3"><span className="text-violet-400">$</span> <span className="text-zinc-300">orbex run a3b8d1b6</span></div>
                            <div className="text-emerald-400">✓ Run triggered: 7d2e9f41 (status: pending)</div>
                            <div className="mt-3"><span className="text-violet-400">$</span> <span className="text-zinc-300">orbex runs list a3b8d1b6</span></div>
                            <div className="text-zinc-400">
                                ID&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;STATUS&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;EXIT&nbsp;&nbsp;DURATION<br />
                                7d2e9f41&nbsp;&nbsp;succeeded&nbsp;&nbsp;0&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;2.3s
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            {/* Features */}
            <section className="py-20 px-6 border-t border-zinc-800/50">
                <div className="max-w-6xl mx-auto">
                    <h2 className="text-3xl font-bold text-center mb-4">Everything you need to run jobs at scale</h2>
                    <p className="text-zinc-500 text-center mb-16 max-w-xl mx-auto">
                        From a one-off container to a full production pipeline — Orbex handles it.
                    </p>
                    <div className="grid md:grid-cols-3 gap-6">
                        {[
                            {
                                icon: '🐳',
                                title: 'Docker Native',
                                desc: 'Run any Docker image with resource limits. CPU capping, memory guards, and automatic cleanup.',
                            },
                            {
                                icon: '⏰',
                                title: 'Cron Scheduling',
                                desc: 'Set cron expressions and Orbex auto-enqueues runs. No external scheduler needed.',
                            },
                            {
                                icon: '🔗',
                                title: 'Webhook Triggers',
                                desc: 'Generate a URL and trigger jobs from GitHub Actions, Stripe, or any HTTP client.',
                            },
                            {
                                icon: '💓',
                                title: 'Heartbeat & Recovery',
                                desc: 'Automatic stale job detection. Dead containers get cleaned up and marked as failed.',
                            },
                            {
                                icon: '📊',
                                title: 'Real-time Dashboard',
                                desc: 'Live log viewer, run history, status badges, and job management in a dark-themed UI.',
                            },
                            {
                                icon: '⚡',
                                title: 'CLI First',
                                desc: 'Full-featured CLI for power users. Manage jobs, trigger runs, and tail logs from the terminal.',
                            },
                        ].map(feature => (
                            <div key={feature.title} className="bg-zinc-900/50 border border-zinc-800 rounded-xl p-6 hover:border-zinc-700 transition-colors group">
                                <div className="text-3xl mb-4">{feature.icon}</div>
                                <h3 className="text-lg font-semibold mb-2 group-hover:text-violet-400 transition-colors">{feature.title}</h3>
                                <p className="text-sm text-zinc-400 leading-relaxed">{feature.desc}</p>
                            </div>
                        ))}
                    </div>
                </div>
            </section>

            {/* API showcase */}
            <section className="py-20 px-6 border-t border-zinc-800/50">
                <div className="max-w-4xl mx-auto">
                    <h2 className="text-3xl font-bold text-center mb-4">Simple, powerful API</h2>
                    <p className="text-zinc-500 text-center mb-12">17 RESTful endpoints. One API key. Full control.</p>
                    <div className="grid md:grid-cols-2 gap-6">
                        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5">
                            <p className="text-xs text-zinc-500 uppercase tracking-wider mb-3">Create a Job</p>
                            <pre className="text-sm font-mono text-zinc-300 overflow-x-auto">
                                {`curl -X POST /api/v1/jobs \\
  -H "Authorization: Bearer obx_..." \\
  -d '{
    "name": "my-etl",
    "image": "python:3.12",
    "command": ["python", "run.py"],
    "timeout_seconds": 300
  }'`}
                            </pre>
                        </div>
                        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5">
                            <p className="text-xs text-zinc-500 uppercase tracking-wider mb-3">Trigger via Webhook</p>
                            <pre className="text-sm font-mono text-zinc-300 overflow-x-auto">
                                {`# No API key needed — just the token
curl -X POST \\
  /api/v1/webhooks/whk_abc123/trigger

# Response: 202 Accepted
{
  "id": "7d2e9f41-...",
  "status": "pending"
}`}
                            </pre>
                        </div>
                    </div>
                </div>
            </section>

            {/* CTA */}
            <section className="py-20 px-6 border-t border-zinc-800/50">
                <div className="max-w-2xl mx-auto text-center">
                    <h2 className="text-3xl font-bold mb-4">Ready to run?</h2>
                    <p className="text-zinc-400 mb-8">
                        Orbex is open source and free to self-host. Star us on GitHub and try it locally in 60 seconds.
                    </p>
                    <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-4 font-mono text-sm text-zinc-300 mb-8">
                        <span className="text-violet-400">$</span> git clone https://github.com/orbex-dev/orbex && cd orbex && make dev
                    </div>
                    <div className="flex items-center justify-center gap-4">
                        <a href="https://github.com/orbex-dev/orbex" className="px-6 py-3 bg-violet-600 hover:bg-violet-500 text-white font-medium rounded-xl transition-all hover:shadow-lg hover:shadow-violet-500/20">
                            ⭐ Star on GitHub
                        </a>
                        <a href="https://twitter.com/orbexdev" className="px-6 py-3 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 font-medium rounded-xl transition-colors border border-zinc-700">
                            Follow @orbexdev
                        </a>
                    </div>
                </div>
            </section>

            {/* Footer */}
            <footer className="border-t border-zinc-800/50 py-8 px-6">
                <div className="max-w-6xl mx-auto flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <span className="text-lg font-bold bg-gradient-to-r from-violet-400 to-cyan-400 bg-clip-text text-transparent">orbex</span>
                        <span className="text-xs text-zinc-600">.dev</span>
                    </div>
                    <p className="text-xs text-zinc-600">© 2026 Orbex. Open source under MIT.</p>
                </div>
            </footer>
        </div>
    );
}
