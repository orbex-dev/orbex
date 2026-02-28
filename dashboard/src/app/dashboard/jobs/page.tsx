'use client';

import { useEffect, useState, useCallback } from 'react';
import { api, Job, JobRun, searchJobs } from '@/lib/api';
import { StatusBadge, SectionHeader, EmptyState, Skeleton, Toast, Dialog, timeAgo } from '@/components/ui';

// ─── Filter types ──────────────────────────────────────────
type Filter = 'all' | 'active' | 'scheduled' | 'webhook';
type Sort = 'created' | 'name' | 'updated';

// ─── Popular images for wizard ─────────────────────────────
const popularImages = [
    { name: 'Alpine', image: 'alpine:latest', desc: 'Minimal Linux (~5MB)' },
    { name: 'Python', image: 'python:3.12-slim', desc: 'Python runtime' },
    { name: 'Node.js', image: 'node:22-slim', desc: 'JavaScript runtime' },
    { name: 'Ubuntu', image: 'ubuntu:24.04', desc: 'Ubuntu Linux' },
    { name: 'Go', image: 'golang:1.23', desc: 'Go toolchain' },
    { name: 'Postgres', image: 'postgres:17', desc: 'Database tools' },
];

const schedulePresets = [
    { label: 'Every 5 min', value: '*/5 * * * *' },
    { label: 'Every 15 min', value: '*/15 * * * *' },
    { label: 'Hourly', value: '0 * * * *' },
    { label: 'Daily (midnight)', value: '0 0 * * *' },
    { label: 'Weekly (Sun)', value: '0 0 * * 0' },
    { label: 'Custom', value: '' },
];

// ─── Job Creation Wizard ───────────────────────────────────
function CreateJobWizard({ onCreated, onClose }: { onCreated: () => void; onClose: () => void }) {
    const [step, setStep] = useState(1);
    const [creating, setCreating] = useState(false);
    const [jobMode, setJobMode] = useState<'docker' | 'script'>('docker');
    const [scriptLang, setScriptLang] = useState('python');
    const [scriptContent, setScriptContent] = useState('');
    const [form, setForm] = useState({
        name: '', image: 'alpine:latest', command: '',
        memory_mb: 512, cpu_millicores: 1000, timeout_seconds: 3600,
        schedule: '', customSchedule: '',
    });
    const [envPairs, setEnvPairs] = useState<Array<{ key: string; value: string }>>([]);

    async function handleCreate() {
        setCreating(true);
        try {
            const env: Record<string, string> = {};
            envPairs.forEach(p => { if (p.key) env[p.key] = p.value; });
            const runtimeImages: Record<string, string> = {
                python: 'python:3.12-slim', node: 'node:22-slim', bash: 'alpine:latest',
                go: 'golang:1.22-alpine', ruby: 'ruby:3.3-slim',
            };
            await api.createJob({
                name: form.name,
                image: jobMode === 'script' ? runtimeImages[scriptLang] || 'alpine:latest' : form.image,
                command: jobMode === 'docker' && form.command ? form.command.split(' ') : undefined,
                env: Object.keys(env).length > 0 ? env : undefined,
                memory_mb: form.memory_mb,
                cpu_millicores: form.cpu_millicores,
                timeout_seconds: form.timeout_seconds,
                schedule: (form.schedule === '' && form.customSchedule) ? form.customSchedule : form.schedule || undefined,
                script: jobMode === 'script' && scriptContent ? scriptContent : undefined,
                script_lang: jobMode === 'script' ? scriptLang : undefined,
            });
            onCreated();
            onClose();
        } catch (err) {
            console.error(err);
        } finally {
            setCreating(false);
        }
    }

    const stepTitles = ['Name', 'Image', 'Entry Command', 'Env Vars', 'Resources', 'Schedule', 'Review'];

    return (
        <Dialog onClose={onClose}>
            <div className="p-6">
                {/* Progress */}
                <div className="flex items-center gap-1 mb-5">
                    {stepTitles.map((t, i) => (
                        <div key={i} className="flex items-center gap-1">
                            <button
                                onClick={() => i + 1 < step && setStep(i + 1)}
                                className={`text-[11px] font-medium px-2 py-0.5 rounded-full transition-colors ${i + 1 === step ? 'bg-blue-500/20 text-blue-400' :
                                    i + 1 < step ? 'text-zinc-400 hover:text-zinc-300 cursor-pointer' :
                                        'text-zinc-600'
                                    }`}
                            >{t}</button>
                            {i < stepTitles.length - 1 && <span className="text-zinc-700">→</span>}
                        </div>
                    ))}
                </div>

                {/* Step 1: Name */}
                {step === 1 && (
                    <div>
                        <label className="block text-sm font-medium text-zinc-300 mb-2">Job Name</label>
                        <input
                            className="input text-base" placeholder="my-etl-job"
                            value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                            autoFocus
                            onKeyDown={e => e.key === 'Enter' && form.name && setStep(2)}
                        />
                        <p className="text-xs text-zinc-500 mt-2">A readable name. Must be unique within your account.</p>
                    </div>
                )}

                {/* Step 2: Image / Script Mode */}
                {step === 2 && (
                    <div>
                        {/* Mode toggle */}
                        <div className="flex gap-2 mb-4">
                            <button
                                onClick={() => setJobMode('docker')}
                                className={`flex-1 p-3 rounded-lg text-sm font-medium text-center transition-all ${jobMode === 'docker'
                                    ? 'bg-blue-500/10 border border-blue-500/30 text-blue-400'
                                    : 'text-zinc-400 border hover:border-zinc-600'
                                    }`}
                                style={{ borderColor: jobMode === 'docker' ? undefined : 'var(--border)' }}
                            >
                                📦 Docker Image
                                <p className="text-[10px] text-zinc-500 mt-0.5 font-normal">Bring your own image</p>
                            </button>
                            <button
                                onClick={() => setJobMode('script')}
                                className={`flex-1 p-3 rounded-lg text-sm font-medium text-center transition-all ${jobMode === 'script'
                                    ? 'bg-violet-500/10 border border-violet-500/30 text-violet-400'
                                    : 'text-zinc-400 border hover:border-zinc-600'
                                    }`}
                                style={{ borderColor: jobMode === 'script' ? undefined : 'var(--border)' }}
                            >
                                ✏️ Write Script
                                <p className="text-[10px] text-zinc-500 mt-0.5 font-normal">No Docker required</p>
                            </button>
                        </div>

                        {jobMode === 'docker' ? (
                            <>
                                <label className="block text-sm font-medium text-zinc-300 mb-2">Docker Image</label>
                                <div className="grid grid-cols-3 gap-2 mb-3">
                                    {popularImages.map(img => (
                                        <button
                                            key={img.image}
                                            onClick={() => setForm({ ...form, image: img.image })}
                                            className={`p-3 rounded-lg text-left transition-all ${form.image === img.image
                                                ? 'bg-blue-500/10 border border-blue-500/30'
                                                : 'border hover:border-zinc-600'
                                                }`}
                                            style={{ borderColor: form.image === img.image ? undefined : 'var(--border)' }}
                                        >
                                            <span className="text-xs font-semibold text-zinc-200 block">{img.name}</span>
                                            <span className="text-[10px] text-zinc-500 font-mono">{img.image}</span>
                                        </button>
                                    ))}
                                </div>
                                <input
                                    className="input input-mono" placeholder="or type custom image..."
                                    value={form.image} onChange={e => setForm({ ...form, image: e.target.value })}
                                />
                            </>
                        ) : (
                            <>
                                <label className="block text-sm font-medium text-zinc-300 mb-2">Runtime</label>
                                <div className="grid grid-cols-3 gap-2">
                                    {[
                                        { lang: 'python', label: 'Python', desc: '3.12' },
                                        { lang: 'node', label: 'Node.js', desc: '22 LTS' },
                                        { lang: 'bash', label: 'Bash', desc: 'Alpine' },
                                        { lang: 'go', label: 'Go', desc: '1.22' },
                                        { lang: 'ruby', label: 'Ruby', desc: '3.3' },
                                    ].map(rt => (
                                        <button
                                            key={rt.lang}
                                            onClick={() => setScriptLang(rt.lang)}
                                            className={`p-3 rounded-lg text-left transition-all ${scriptLang === rt.lang
                                                ? 'bg-violet-500/10 border border-violet-500/30'
                                                : 'border hover:border-zinc-600'
                                                }`}
                                            style={{ borderColor: scriptLang === rt.lang ? undefined : 'var(--border)' }}
                                        >
                                            <span className="text-xs font-semibold text-zinc-200 block">{rt.label}</span>
                                            <span className="text-[10px] text-zinc-500">{rt.desc}</span>
                                        </button>
                                    ))}
                                </div>
                            </>
                        )}
                    </div>
                )}

                {/* Step 3: Command / Script */}
                {step === 3 && (
                    <div>
                        {jobMode === 'script' ? (
                            <>
                                <label className="block text-sm font-medium text-zinc-300 mb-2">Script Code</label>
                                <textarea
                                    className="input input-mono text-sm leading-relaxed"
                                    rows={12}
                                    placeholder={scriptLang === 'python' ? 'print("Hello from Orbex!")' :
                                        scriptLang === 'node' ? 'console.log("Hello from Orbex!");' :
                                            scriptLang === 'bash' ? 'echo "Hello from Orbex!"' :
                                                scriptLang === 'go' ? 'package main\n\nimport "fmt"\n\nfunc main() {\n    fmt.Println("Hello from Orbex!")\n}' :
                                                    '# Your code here'}
                                    value={scriptContent}
                                    onChange={e => setScriptContent(e.target.value)}
                                    autoFocus
                                    style={{ tabSize: 2, resize: 'vertical', minHeight: '200px' }}
                                />
                                <p className="text-xs text-zinc-500 mt-2">Write your {scriptLang} code above. It will be mounted into a container and executed at runtime.</p>
                            </>
                        ) : (
                            <>
                                <label className="block text-sm font-medium text-zinc-300 mb-2">Container Command <span className="text-zinc-500 font-normal">(optional)</span></label>
                                <input
                                    className="input input-mono" placeholder="python run.py --verbose"
                                    value={form.command} onChange={e => setForm({ ...form, command: e.target.value })}
                                    autoFocus
                                />
                                <p className="text-xs text-zinc-500 mt-2">What should the container execute when it starts? This overrides the Docker image&apos;s built-in CMD. Leave empty to use the image&apos;s default entrypoint.</p>
                                {form.image.includes('python') && (
                                    <p className="text-xs text-blue-400/60 mt-1">💡 Try: <code className="text-blue-400">python -c &quot;print(&apos;hello&apos;)&quot;</code></p>
                                )}
                                {form.image.includes('node') && (
                                    <p className="text-xs text-blue-400/60 mt-1">💡 Try: <code className="text-blue-400">node -e &quot;console.log(&apos;hello&apos;)&quot;</code></p>
                                )}
                            </>
                        )}
                    </div>
                )}

                {/* Step 4: Env Vars */}
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

                {/* Step 5: Resources */}
                {step === 5 && (
                    <div className="space-y-4">
                        <div>
                            <label className="text-sm font-medium text-zinc-300 flex items-center justify-between">
                                Memory
                                <span className="text-xs text-zinc-500 font-mono">{form.memory_mb}MB</span>
                            </label>
                            <input type="range" min="64" max="4096" step="64"
                                value={form.memory_mb} onChange={e => setForm({ ...form, memory_mb: parseInt(e.target.value) })}
                                className="w-full mt-2 accent-blue-500"
                            />
                            <div className="flex justify-between text-[10px] text-zinc-600 mt-0.5"><span>64MB</span><span>4GB</span></div>
                        </div>
                        <div>
                            <label className="text-sm font-medium text-zinc-300 flex items-center justify-between">
                                CPU
                                <span className="text-xs text-zinc-500 font-mono">{(form.cpu_millicores / 1000).toFixed(1)} cores</span>
                            </label>
                            <input type="range" min="100" max="4000" step="100"
                                value={form.cpu_millicores} onChange={e => setForm({ ...form, cpu_millicores: parseInt(e.target.value) })}
                                className="w-full mt-2 accent-blue-500"
                            />
                            <div className="flex justify-between text-[10px] text-zinc-600 mt-0.5"><span>0.1</span><span>4.0</span></div>
                        </div>
                        <div>
                            <label className="text-sm font-medium text-zinc-300 flex items-center justify-between">
                                Timeout
                                <span className="text-xs text-zinc-500 font-mono">{form.timeout_seconds >= 3600 ? `${(form.timeout_seconds / 3600).toFixed(1)}h` : `${form.timeout_seconds}s`}</span>
                            </label>
                            <input type="range" min="30" max="86400" step="30"
                                value={form.timeout_seconds} onChange={e => setForm({ ...form, timeout_seconds: parseInt(e.target.value) })}
                                className="w-full mt-2 accent-blue-500"
                            />
                            <div className="flex justify-between text-[10px] text-zinc-600 mt-0.5"><span>30s</span><span>24h</span></div>
                        </div>
                    </div>
                )}

                {/* Step 6: Schedule */}
                {step === 6 && (
                    <div>
                        <label className="block text-sm font-medium text-zinc-300 mb-2">Schedule <span className="text-zinc-500 font-normal">(optional)</span></label>
                        <div className="grid grid-cols-3 gap-2 mb-3">
                            {schedulePresets.map(p => (
                                <button
                                    key={p.label}
                                    onClick={() => setForm({ ...form, schedule: p.value })}
                                    className={`p-2.5 rounded-lg text-xs font-medium text-center transition-all ${form.schedule === p.value
                                        ? 'bg-blue-500/10 border border-blue-500/30 text-blue-400'
                                        : 'text-zinc-400 border hover:border-zinc-600'
                                        }`}
                                    style={{ borderColor: form.schedule === p.value ? undefined : 'var(--border)' }}
                                >{p.label}</button>
                            ))}
                        </div>
                        {form.schedule === '' && (
                            <input
                                className="input input-mono" placeholder="*/10 * * * *"
                                value={form.customSchedule}
                                onChange={e => setForm({ ...form, customSchedule: e.target.value })}
                            />
                        )}
                        <p className="text-xs text-zinc-500 mt-2">Leave empty for manual-only execution via UI, webhook, or API.</p>
                    </div>
                )}

                {/* Step 7: Review */}
                {step === 7 && (
                    <div>
                        <h3 className="text-sm font-semibold text-zinc-200 mb-3">Review Job</h3>
                        <div className="space-y-2 text-sm">
                            {[
                                ['Name', form.name],
                                ['Mode', jobMode === 'script' ? `Inline Script (${scriptLang})` : 'Docker Image'],
                                ...(jobMode === 'docker' ? [
                                    ['Image', form.image],
                                    ['Entry Command', form.command || '(image default)'],
                                ] : [
                                    ['Runtime', scriptLang],
                                    ['Script', scriptContent ? `${scriptContent.split('\n').length} lines` : '(empty)'],
                                ]),
                                ['Env Vars', envPairs.filter(p => p.key).length > 0 ? envPairs.filter(p => p.key).map(p => `${p.key}=${p.value}`).join(', ') : '(none)'],
                                ['Memory', `${form.memory_mb}MB`],
                                ['CPU', `${(form.cpu_millicores / 1000).toFixed(1)} cores`],
                                ['Timeout', form.timeout_seconds >= 3600 ? `${(form.timeout_seconds / 3600).toFixed(1)}h` : `${form.timeout_seconds}s`],
                                ['Schedule', (form.schedule || form.customSchedule) || 'Manual only'],
                            ].map(([label, value]) => (
                                <div key={label} className="flex justify-between py-1.5 border-b" style={{ borderColor: 'var(--border)' }}>
                                    <span className="text-zinc-500">{label}</span>
                                    <span className="text-zinc-200 font-mono text-xs">{value}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}
            </div>

            {/* Footer */}
            <div className="border-t px-6 py-4 flex items-center justify-between" style={{ borderColor: 'var(--border)' }}>
                <button onClick={step > 1 ? () => setStep(step - 1) : onClose} className="btn-ghost btn-sm">
                    {step > 1 ? '← Back' : 'Cancel'}
                </button>
                {step < 7 ? (
                    <button onClick={() => setStep(step + 1)} disabled={step === 1 && !form.name} className="btn btn-primary btn-sm">
                        Next →
                    </button>
                ) : (
                    <button onClick={handleCreate} disabled={creating} className="btn btn-primary btn-sm">
                        {creating ? 'Creating...' : 'Create Job'}
                    </button>
                )}
            </div>
        </Dialog>
    );
}

// ─── Jobs Page ─────────────────────────────────────────────
export default function JobsPage() {
    const [jobs, setJobs] = useState<Job[]>([]);
    const [jobRuns, setJobRuns] = useState<Record<string, JobRun[]>>({});
    const [loading, setLoading] = useState(true);
    const [showCreate, setShowCreate] = useState(false);
    const [query, setQuery] = useState('');
    const [filter, setFilter] = useState<Filter>('all');
    const [sort, setSort] = useState<Sort>('created');
    const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

    async function loadJobs() {
        try {
            const data = await api.listJobs();
            setJobs(data);
            // Load recent runs for each job (for card status)
            const runsMap: Record<string, JobRun[]> = {};
            for (const job of data) {
                const runs = await api.listRuns(job.id);
                runsMap[job.id] = runs;
            }
            setJobRuns(runsMap);
        } catch (err) {
            console.error(err);
        } finally {
            setLoading(false);
        }
    }

    useEffect(() => { loadJobs(); }, []);

    // Quick actions
    async function handleQuickRun(jobId: string) {
        try {
            await api.triggerRun(jobId);
            setToast({ message: 'Run triggered', type: 'success' });
            setTimeout(loadJobs, 1000);
        } catch (err) {
            setToast({ message: `Failed: ${err}`, type: 'error' });
        }
    }

    async function handleQuickAction(runId: string, action: 'pause' | 'resume' | 'kill') {
        try {
            if (action === 'pause') await api.pauseRun(runId);
            else if (action === 'resume') await api.resumeRun(runId);
            else await api.killRun(runId);
            setToast({ message: `Run ${action}d`, type: 'success' });
            setTimeout(loadJobs, 500);
        } catch (err) {
            setToast({ message: `Failed: ${err}`, type: 'error' });
        }
    }

    // Filtering + sorting
    let displayed = searchJobs(jobs, query);
    if (filter === 'active') displayed = displayed.filter(j => j.is_active);
    if (filter === 'scheduled') displayed = displayed.filter(j => j.schedule);
    if (filter === 'webhook') displayed = displayed.filter(j => j.webhook_token);

    if (sort === 'name') displayed.sort((a, b) => a.name.localeCompare(b.name));
    else if (sort === 'updated') displayed.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
    else displayed.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());

    if (loading) {
        return (
            <div>
                <div className="flex items-center justify-between mb-6">
                    <Skeleton className="h-8 w-24" />
                    <Skeleton className="h-9 w-28 rounded-lg" />
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                    {[1, 2, 3, 4, 5, 6].map(i => <Skeleton key={i} className="h-44 rounded-xl" />)}
                </div>
            </div>
        );
    }

    return (
        <div>
            {toast && <Toast message={toast.message} type={toast.type} onDone={() => setToast(null)} />}
            {showCreate && <CreateJobWizard onCreated={loadJobs} onClose={() => setShowCreate(false)} />}

            <SectionHeader title="Jobs" subtitle={`${jobs.length} configured`}
                action={
                    <button onClick={() => setShowCreate(true)} className="btn btn-primary">
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2.5}><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
                        New Job
                    </button>
                }
            />

            {/* Search + Filters */}
            <div className="flex flex-col md:flex-row items-start md:items-center gap-3 mb-5">
                {/* Search */}
                <div className="relative flex-1 max-w-sm">
                    <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" /></svg>
                    <input
                        className="input" placeholder="Search jobs..."
                        style={{ paddingLeft: '2.5rem' }}
                        value={query} onChange={e => setQuery(e.target.value)}
                    />
                </div>

                {/* Filter chips */}
                <div className="flex items-center gap-1.5">
                    {(['all', 'active', 'scheduled', 'webhook'] as Filter[]).map(f => (
                        <button
                            key={f}
                            onClick={() => setFilter(f)}
                            className={`px-3 py-1.5 text-xs font-medium rounded-full transition-all ${filter === f
                                ? 'bg-blue-500/15 text-blue-400 border border-blue-500/30'
                                : 'text-zinc-500 border hover:text-zinc-300'
                                }`}
                            style={{ borderColor: filter === f ? undefined : 'var(--border)' }}
                        >
                            {f.charAt(0).toUpperCase() + f.slice(1)}
                        </button>
                    ))}
                </div>

                {/* Sort */}
                <select
                    value={sort} onChange={e => setSort(e.target.value as Sort)}
                    className="input text-xs py-1.5 w-auto"
                    style={{ maxWidth: 140 }}
                >
                    <option value="created">Newest first</option>
                    <option value="name">A → Z</option>
                    <option value="updated">Last updated</option>
                </select>
            </div>

            {/* Jobs grid */}
            {displayed.length === 0 ? (
                <EmptyState
                    title={query ? 'No matching jobs' : 'No jobs yet'}
                    description={query ? 'Try a different search term' : 'Create your first job to start running containers on Orbex'}
                    action={!query ? <button onClick={() => setShowCreate(true)} className="btn btn-primary btn-sm">Create First Job</button> : undefined}
                />
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                    {displayed.map((job, idx) => {
                        const runs = jobRuns[job.id] || [];
                        const lastRun = runs[0];
                        const activeRun = runs.find(r => r.status === 'running' || r.status === 'paused');
                        const runCount = runs.length;

                        return (
                            <div
                                key={job.id}
                                className="card-interactive p-5 animate-slide-up"
                                style={{ animationDelay: `${idx * 40}ms`, opacity: 0 }}
                            >
                                {/* Active run banner */}
                                {activeRun && (
                                    <div className="flex items-center gap-2 mb-3 p-2 rounded-lg bg-blue-500/5 border border-blue-500/15">
                                        <StatusBadge status={activeRun.status} />
                                        <span className="text-[11px] text-zinc-500 flex-1 truncate font-mono">{activeRun.id.slice(0, 8)}</span>
                                        {activeRun.status === 'running' && (
                                            <button onClick={(e) => { e.preventDefault(); handleQuickAction(activeRun.id, 'pause'); }} className="btn-ghost btn-sm p-1" title="Pause">
                                                <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 24 24"><rect x="6" y="4" width="4" height="16" rx="1" /><rect x="14" y="4" width="4" height="16" rx="1" /></svg>
                                            </button>
                                        )}
                                        {activeRun.status === 'paused' && (
                                            <button onClick={(e) => { e.preventDefault(); handleQuickAction(activeRun.id, 'resume'); }} className="btn-ghost btn-sm p-1" title="Resume">
                                                <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z" /></svg>
                                            </button>
                                        )}
                                        <button onClick={(e) => { e.preventDefault(); handleQuickAction(activeRun.id, 'kill'); }} className="btn-danger btn-sm p-1" title="Kill">
                                            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2.5}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                                        </button>
                                    </div>
                                )}

                                {/* Header */}
                                <div className="flex items-start justify-between mb-2">
                                    <a href={`/dashboard/jobs/${job.id}`} className="flex-1 min-w-0">
                                        <h3 className="text-[14px] font-semibold text-zinc-100 hover:text-blue-400 transition-colors truncate">
                                            {job.name}
                                        </h3>
                                        <p className="text-[11px] text-zinc-500 font-mono mt-0.5 truncate">{job.image}</p>
                                    </a>
                                    <span className={`text-[10px] font-semibold uppercase tracking-wider px-2 py-0.5 rounded-full shrink-0 ${job.is_active
                                        ? 'text-lime-400 bg-lime-dim border border-lime-500/20'
                                        : 'text-zinc-500 border'
                                        }`} style={{ borderColor: job.is_active ? undefined : 'var(--border)' }}>
                                        {job.is_active ? 'Active' : 'Paused'}
                                    </span>
                                </div>

                                {/* Meta row */}
                                <div className="flex items-center gap-2 text-[11px] text-zinc-500 mt-3 flex-wrap">
                                    {job.schedule && (
                                        <span className="flex items-center gap-1 px-1.5 py-0.5 rounded bg-blue-500/8 text-blue-400 border border-blue-500/10">
                                            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                                            {job.schedule}
                                        </span>
                                    )}
                                    {job.webhook_token && (
                                        <span className="flex items-center gap-1 px-1.5 py-0.5 rounded bg-orange-500/8 text-orange-400 border border-orange-500/10">⚡ webhook</span>
                                    )}
                                    {lastRun && !activeRun && (
                                        <span className="flex items-center gap-1">
                                            <span className={`w-1.5 h-1.5 rounded-full ${lastRun.status === 'succeeded' ? 'bg-lime-400' : 'bg-orange-400'}`} />
                                            {lastRun.status}
                                        </span>
                                    )}
                                    <span className="text-zinc-600">{runCount} runs</span>
                                </div>

                                {/* Quick actions */}
                                <div className="flex items-center gap-1.5 mt-3 pt-3 border-t" style={{ borderColor: 'var(--border)' }}>
                                    <button
                                        onClick={(e) => { e.preventDefault(); handleQuickRun(job.id); }}
                                        className="btn-lime btn-sm flex-1 flex items-center justify-center gap-1.5"
                                    >
                                        <svg className="w-3 h-3 shrink-0" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z" /></svg>
                                        <span>Run</span>
                                    </button>
                                    <a href={`/dashboard/jobs/${job.id}`} className="btn-ghost btn-sm flex-1 justify-center text-center no-underline">
                                        Details →
                                    </a>
                                </div>
                            </div>
                        );
                    })}
                </div>
            )}
        </div>
    );
}
