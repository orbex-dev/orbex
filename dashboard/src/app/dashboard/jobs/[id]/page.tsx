'use client';

import { useEffect, useState, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { api, Job, JobRun, UpdateJobRequest, TriggerRunRequest } from '@/lib/api';
import { StatusBadge, SectionHeader, Skeleton, Toast, Dialog, timeAgo, formatDuration } from '@/components/ui';

// ─── Inline Editable Field ─────────────────────────────────
function EditableField({ label, value, field, type = 'text', onSave, mono = false }: {
    label: string; value: string; field: string; type?: string; mono?: boolean;
    onSave: (field: string, value: string) => Promise<void>;
}) {
    const [editing, setEditing] = useState(false);
    const [val, setVal] = useState(value);
    const [saving, setSaving] = useState(false);

    async function save() {
        if (val === value) { setEditing(false); return; }
        setSaving(true);
        await onSave(field, val);
        setSaving(false);
        setEditing(false);
    }

    return (
        <div className="card p-3 group">
            <p className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1">{label}</p>
            {editing ? (
                <div className="flex items-center gap-2">
                    <input
                        type={type} value={val} onChange={e => setVal(e.target.value)}
                        className={`input text-sm py-1.5 ${mono ? 'input-mono' : ''}`}
                        autoFocus onKeyDown={e => e.key === 'Enter' && save()}
                    />
                    <button onClick={save} disabled={saving} className="btn btn-primary btn-sm">{saving ? '...' : '✓'}</button>
                    <button onClick={() => { setEditing(false); setVal(value); }} className="btn-ghost btn-sm">✕</button>
                </div>
            ) : (
                <div className="flex items-center justify-between">
                    <p className={`text-sm font-medium text-zinc-200 ${mono ? 'font-mono' : ''}`}>{value}</p>
                    <button
                        onClick={() => setEditing(true)}
                        className="opacity-0 group-hover:opacity-100 text-zinc-500 hover:text-zinc-300 transition-all text-xs"
                    >edit</button>
                </div>
            )}
        </div>
    );
}

// ─── Run With Overrides Dialog ─────────────────────────────
function RunOverridesDialog({ job, onRun, onClose }: {
    job: Job; onRun: (overrides: TriggerRunRequest) => void; onClose: () => void;
}) {
    const [timeout, setTimeout_] = useState(String(job.timeout_seconds));
    const [command, setCommand] = useState(job.command?.join(' ') || '');
    const [envPairs, setEnvPairs] = useState<Array<{ key: string; value: string }>>(
        Object.entries(job.env || {}).map(([key, value]) => ({ key, value }))
    );

    function addEnvPair() { setEnvPairs([...envPairs, { key: '', value: '' }]); }
    function removeEnvPair(idx: number) { setEnvPairs(envPairs.filter((_, i) => i !== idx)); }

    function handleRun() {
        const overrides: TriggerRunRequest = {};
        const t = parseInt(timeout);
        if (t !== job.timeout_seconds) overrides.timeout_seconds = t;
        if (command !== (job.command?.join(' ') || '')) overrides.command = command ? command.split(' ') : undefined;
        const env: Record<string, string> = {};
        envPairs.forEach(p => { if (p.key) env[p.key] = p.value; });
        if (Object.keys(env).length > 0) overrides.env = env;
        onRun(overrides);
    }

    return (
        <Dialog onClose={onClose}>
            <div className="p-6">
                <h3 className="text-lg font-bold text-zinc-100 mb-1">Run with Overrides</h3>
                <p className="text-xs text-zinc-500 mb-5">Override job config for this single run. Changes won&apos;t affect the job definition.</p>

                <div className="space-y-4">
                    <div>
                        <label className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1 block">Timeout (seconds)</label>
                        <input className="input input-mono" type="number" value={timeout} onChange={e => setTimeout_(e.target.value)} />
                    </div>
                    <div>
                        <label className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1 block">Command override</label>
                        <input className="input input-mono" placeholder="echo hello" value={command} onChange={e => setCommand(e.target.value)} />
                    </div>
                    <div>
                        <div className="flex items-center justify-between mb-1">
                            <label className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">Environment Variables</label>
                            <button onClick={addEnvPair} className="text-xs text-blue-400 hover:text-blue-300">+ Add</button>
                        </div>
                        <div className="space-y-1.5">
                            {envPairs.map((pair, i) => (
                                <div key={i} className="flex items-center gap-2">
                                    <input className="input input-mono text-xs flex-1" placeholder="KEY" value={pair.key}
                                        onChange={e => { const p = [...envPairs]; p[i].key = e.target.value; setEnvPairs(p); }} />
                                    <input className="input input-mono text-xs flex-1" placeholder="value" value={pair.value}
                                        onChange={e => { const p = [...envPairs]; p[i].value = e.target.value; setEnvPairs(p); }} />
                                    <button onClick={() => removeEnvPair(i)} className="text-zinc-500 hover:text-red-400 text-xs">✕</button>
                                </div>
                            ))}
                            {envPairs.length === 0 && <p className="text-xs text-zinc-600">No env vars</p>}
                        </div>
                    </div>
                </div>
            </div>
            <div className="border-t px-6 py-4 flex justify-end gap-2" style={{ borderColor: 'var(--border)' }}>
                <button onClick={onClose} className="btn-ghost btn-sm">Cancel</button>
                <button onClick={handleRun} className="btn btn-primary btn-sm">Run with Overrides</button>
            </div>
        </Dialog>
    );
}

// ─── Page ──────────────────────────────────────────────────
export default function JobDetailPage() {
    const params = useParams();
    const router = useRouter();
    const jobId = params.id as string;
    const [job, setJob] = useState<Job | null>(null);
    const [runs, setRuns] = useState<JobRun[]>([]);
    const [loading, setLoading] = useState(true);
    const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);
    const [showOverrides, setShowOverrides] = useState(false);
    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
    const [webhookData, setWebhookData] = useState<{ token: string; triggerUrl: string } | null>(null);

    async function loadData() {
        try {
            const [jobData, runsData] = await Promise.all([
                api.getJob(jobId), api.listRuns(jobId),
            ]);
            setJob(jobData);
            setRuns(runsData);
        } catch (err) { console.error(err); }
        finally { setLoading(false); }
    }

    useEffect(() => { loadData(); }, [jobId]);

    // Inline edit save
    const handleFieldSave = useCallback(async (field: string, value: string) => {
        const update: UpdateJobRequest = {};
        if (field === 'name') update.name = value;
        if (field === 'image') update.image = value;
        if (field === 'command') update.command = value ? value.split(' ') : [];
        if (field === 'memory_mb') update.memory_mb = parseInt(value);
        if (field === 'cpu_millicores') update.cpu_millicores = parseInt(value);
        if (field === 'timeout_seconds') update.timeout_seconds = parseInt(value);
        if (field === 'schedule') update.schedule = value;
        try {
            await api.updateJob(jobId, update);
            setToast({ message: 'Updated', type: 'success' });
            loadData();
        } catch (err) {
            setToast({ message: `Failed: ${err}`, type: 'error' });
        }
    }, [jobId]);

    async function handleTrigger() {
        try {
            await api.triggerRun(jobId);
            setToast({ message: 'Run triggered', type: 'success' });
            setTimeout(loadData, 1000);
        } catch (err) { setToast({ message: `Failed: ${err}`, type: 'error' }); }
    }

    async function handleTriggerWithOverrides(overrides: TriggerRunRequest) {
        try {
            await api.triggerRunWithOverrides(jobId, overrides);
            setToast({ message: 'Run triggered with overrides', type: 'success' });
            setShowOverrides(false);
            setTimeout(loadData, 1000);
        } catch (err) { setToast({ message: `Failed: ${err}`, type: 'error' }); }
    }

    async function handleToggleActive() {
        try {
            await api.updateJob(jobId, { is_active: !job!.is_active });
            setToast({ message: job!.is_active ? 'Job paused' : 'Job activated', type: 'success' });
            loadData();
        } catch (err) { setToast({ message: `Failed: ${err}`, type: 'error' }); }
    }

    async function handleGenerateWebhook() {
        try {
            const result = await api.generateWebhook(jobId);
            setWebhookData({ token: result.webhook_token, triggerUrl: result.trigger_url });
            loadData();
        } catch (err) { setToast({ message: `Failed: ${err}`, type: 'error' }); }
    }

    async function handleDelete() {
        try {
            await api.deleteJob(jobId);
            router.push('/dashboard/jobs');
        } catch (err) {
            setShowDeleteConfirm(false);
            setToast({ message: `Failed: ${err}`, type: 'error' });
        }
    }

    async function handleRunAction(runId: string, action: 'pause' | 'resume' | 'kill') {
        try {
            if (action === 'pause') await api.pauseRun(runId);
            else if (action === 'resume') await api.resumeRun(runId);
            else await api.killRun(runId);
            setToast({ message: `Run ${action}d`, type: 'success' });
            setTimeout(loadData, 500);
        } catch (err) { setToast({ message: `Failed: ${err}`, type: 'error' }); }
    }

    if (loading || !job) {
        return (
            <div>
                <Skeleton className="h-4 w-24 mb-2" />
                <Skeleton className="h-8 w-48 mb-6" />
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-8">
                    {[1, 2, 3, 4].map(i => <Skeleton key={i} className="h-20 rounded-xl" />)}
                </div>
            </div>
        );
    }

    const activeRun = runs.find(r => r.status === 'running' || r.status === 'paused');
    const fullWebhookUrl = typeof window !== 'undefined' ? `${window.location.protocol}//${window.location.hostname}:8080/api/v1/webhooks/${job.webhook_token}/trigger` : '';
    const curlCommand = `curl -X POST http://localhost:8080/api/v1/jobs/${job.id}/run \\\n  -H "Authorization: Bearer YOUR_API_KEY"`;

    return (
        <div>
            {toast && <Toast message={toast.message} type={toast.type} onDone={() => setToast(null)} />}
            {showOverrides && <RunOverridesDialog job={job} onRun={handleTriggerWithOverrides} onClose={() => setShowOverrides(false)} />}
            {showDeleteConfirm && (
                <Dialog onClose={() => setShowDeleteConfirm(false)}>
                    <div className="p-6">
                        <h3 className="text-lg font-bold text-zinc-100 mb-2">Delete Job</h3>
                        <p className="text-sm text-zinc-400">Delete <strong className="text-zinc-200">{job.name}</strong> and all runs? This cannot be undone.</p>
                    </div>
                    <div className="border-t px-6 py-4 flex justify-end gap-2" style={{ borderColor: 'var(--border)' }}>
                        <button onClick={() => setShowDeleteConfirm(false)} className="btn-ghost btn-sm">Cancel</button>
                        <button onClick={handleDelete} className="btn btn-sm" style={{ background: '#ef4444', color: 'white' }}>Delete Job</button>
                    </div>
                </Dialog>
            )}
            {webhookData && (
                <Dialog onClose={() => setWebhookData(null)}>
                    <div className="p-6">
                        <h3 className="text-lg font-bold text-zinc-100 mb-1">Webhook Generated</h3>
                        <p className="text-xs text-zinc-500 mb-4">Use this URL to trigger the job externally — no API key needed.</p>
                        <label className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1 block">Trigger URL</label>
                        <div className="flex gap-2 mb-3">
                            <code className="flex-1 text-xs text-blue-400 font-mono p-2 rounded-lg overflow-x-auto" style={{ background: 'var(--surface-2)', border: '1px solid var(--border)' }}>
                                {typeof window !== 'undefined' ? `${window.location.protocol}//${window.location.hostname}:8080${webhookData.triggerUrl}` : webhookData.triggerUrl}
                            </code>
                            <button onClick={() => navigator.clipboard.writeText(`${window.location.protocol}//${window.location.hostname}:8080${webhookData.triggerUrl}`)} className="btn-ghost btn-sm">Copy</button>
                        </div>
                        <label className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1 block">Token</label>
                        <code className="block text-xs text-violet-400 font-mono p-2 rounded-lg" style={{ background: 'var(--surface-2)', border: '1px solid var(--border)' }}>{webhookData.token}</code>
                    </div>
                    <div className="border-t px-6 py-4 flex justify-end" style={{ borderColor: 'var(--border)' }}>
                        <button onClick={() => setWebhookData(null)} className="btn btn-primary btn-sm">Done</button>
                    </div>
                </Dialog>
            )}

            {/* Header */}
            <div className="flex items-start justify-between mb-6 animate-slide-up">
                <div>
                    <a href="/dashboard/jobs" className="inline-flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-400 transition-colors mb-1">
                        <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                        Jobs
                    </a>
                    <h1 className="text-2xl font-bold text-zinc-100 tracking-tight">{job.name}</h1>
                    <p className="text-sm text-zinc-500 font-mono mt-0.5">{job.image}</p>
                </div>
                <div className="flex items-center gap-2">
                    <button onClick={handleToggleActive} className={`btn-sm ${job.is_active ? 'btn-ghost' : 'btn-lime'}`}>
                        {job.is_active ? 'Pause Job' : 'Activate'}
                    </button>
                    <button onClick={() => setShowDeleteConfirm(true)} className="btn-danger btn-sm">Delete</button>
                </div>
            </div>

            {/* Active run banner */}
            {activeRun && (
                <div className="card p-4 mb-6 flex items-center gap-3 animate-slide-up delay-1 border-l-2" style={{ borderLeftColor: activeRun.status === 'running' ? 'var(--electric)' : '#8b5cf6' }}>
                    <StatusBadge status={activeRun.status} />
                    <div className="flex-1">
                        <a href={`/dashboard/runs/${activeRun.id}`} className="text-sm text-zinc-200 hover:text-blue-400 transition-colors font-medium">
                            Active run <span className="font-mono text-zinc-500">{activeRun.id.slice(0, 8)}</span>
                        </a>
                        {activeRun.started_at && (
                            <p className="text-[11px] text-zinc-500 mt-0.5">Started {timeAgo(activeRun.started_at)}</p>
                        )}
                    </div>
                    <div className="flex gap-1">
                        {activeRun.status === 'running' && (
                            <button onClick={() => handleRunAction(activeRun.id, 'pause')} className="btn-ghost btn-sm">⏸ Pause</button>
                        )}
                        {activeRun.status === 'paused' && (
                            <button onClick={() => handleRunAction(activeRun.id, 'resume')} className="btn-ghost btn-sm">▶ Resume</button>
                        )}
                        <button onClick={() => handleRunAction(activeRun.id, 'kill')} className="btn-danger btn-sm">Kill</button>
                    </div>
                </div>
            )}

            {/* Config (inline editable) */}
            <SectionHeader title="Configuration" subtitle="Click any value to edit" />
            <div className="grid grid-cols-2 md:grid-cols-4 gap-2 mb-8 animate-slide-up delay-2">
                <EditableField label="Memory" value={`${job.memory_mb}`} field="memory_mb" type="number" onSave={handleFieldSave} />
                <EditableField label="CPU (millicores)" value={`${job.cpu_millicores}`} field="cpu_millicores" type="number" onSave={handleFieldSave} />
                <EditableField label="Timeout (sec)" value={`${job.timeout_seconds}`} field="timeout_seconds" type="number" onSave={handleFieldSave} />
                <EditableField label="Schedule" value={job.schedule || 'None'} field="schedule" onSave={handleFieldSave} mono />
            </div>

            <div className="mb-8 animate-slide-up delay-3">
                <EditableField label="Container Command" value={job.command && job.command.length > 0 ? job.command.join(' ') : ''} field="command" onSave={handleFieldSave} mono />
                {(!job.command || job.command.length === 0) && (
                    <p className="text-xs text-zinc-500 mt-1 ml-1">Using image default entrypoint</p>
                )}
            </div>

            {/* Triggers — unified view */}
            <SectionHeader title="Triggers" subtitle="All ways to invoke this job" />
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mb-8 animate-slide-up delay-3">
                {/* Manual */}
                <div className="card p-4">
                    <div className="flex items-center gap-2 mb-2">
                        <div className="w-7 h-7 rounded-lg bg-lime-dim flex items-center justify-center">
                            <svg className="w-3.5 h-3.5 text-lime-400" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z" /></svg>
                        </div>
                        <span className="text-sm font-semibold text-zinc-200">Manual</span>
                    </div>
                    <div className="flex gap-2">
                        <button onClick={handleTrigger} className="btn-lime btn-sm flex-1 justify-center">Run Now</button>
                        <button onClick={() => setShowOverrides(true)} className="btn-ghost btn-sm flex-1 justify-center">With Overrides</button>
                    </div>
                </div>
                {/* Cron */}
                <div className="card p-4">
                    <div className="flex items-center gap-2 mb-2">
                        <div className="w-7 h-7 rounded-lg bg-electric-dim flex items-center justify-center">
                            <svg className="w-3.5 h-3.5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                        </div>
                        <span className="text-sm font-semibold text-zinc-200">Schedule</span>
                        {job.schedule && <span className="text-xs text-blue-400 font-mono ml-auto">{job.schedule}</span>}
                    </div>
                    <p className="text-xs text-zinc-500">{job.schedule ? 'Runs automatically on schedule' : 'No schedule configured — edit above to add'}</p>
                </div>
                {/* Webhook */}
                <div className="card p-4">
                    <div className="flex items-center gap-2 mb-2">
                        <div className="w-7 h-7 rounded-lg bg-coral-dim flex items-center justify-center">
                            <svg className="w-3.5 h-3.5 text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m9.86-2.556a4.5 4.5 0 00-1.242-7.244l4.5-4.5a4.5 4.5 0 016.364 6.364l-1.757 1.757" /></svg>
                        </div>
                        <span className="text-sm font-semibold text-zinc-200">Webhook</span>
                    </div>
                    {job.webhook_token ? (
                        <div>
                            <code className="block text-[10px] text-orange-400/70 font-mono truncate mb-2">{fullWebhookUrl}</code>
                            <div className="flex gap-2">
                                <button onClick={() => navigator.clipboard.writeText(fullWebhookUrl)} className="btn-ghost btn-sm flex-1 justify-center">Copy URL</button>
                                <button onClick={handleGenerateWebhook} className="btn-ghost btn-sm flex-1 justify-center">Regenerate</button>
                            </div>
                        </div>
                    ) : (
                        <button onClick={handleGenerateWebhook} className="btn-ghost btn-sm w-full justify-center">Generate Webhook</button>
                    )}
                </div>
                {/* API */}
                <div className="card p-4">
                    <div className="flex items-center gap-2 mb-2">
                        <div className="w-7 h-7 rounded-lg bg-violet-500/10 flex items-center justify-center">
                            <svg className="w-3.5 h-3.5 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M17.25 6.75L22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3l-4.5 16.5" /></svg>
                        </div>
                        <span className="text-sm font-semibold text-zinc-200">API</span>
                    </div>
                    <pre className="text-[10px] text-zinc-400 font-mono p-2 rounded-lg overflow-x-auto" style={{ background: 'var(--surface-2)' }}>{curlCommand}</pre>
                </div>
            </div>

            {/* Run History */}
            <SectionHeader title="Run History" subtitle={`${runs.length} runs`} />
            {runs.length === 0 ? (
                <div className="card p-8 text-center">
                    <p className="text-sm text-zinc-400">No runs yet. Click &quot;Run Now&quot; to execute.</p>
                </div>
            ) : (
                <div className="card overflow-hidden">
                    <table className="w-full">
                        <thead>
                            <tr className="border-b" style={{ borderColor: 'var(--border)' }}>
                                <th className="text-left p-3 text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">Run</th>
                                <th className="text-left p-3 text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">Status</th>
                                <th className="text-left p-3 text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">Duration</th>
                                <th className="text-left p-3 text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">Exit</th>
                                <th className="text-left p-3 text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">When</th>
                                <th className="text-right p-3 text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {runs.map(run => {
                                const isActive = run.status === 'running' || run.status === 'paused';
                                return (
                                    <tr key={run.id} className="border-b hover:bg-white/[0.02] transition-colors" style={{ borderColor: 'var(--border)' }}>
                                        <td className="p-3">
                                            <a href={`/dashboard/runs/${run.id}`} className="text-sm font-mono text-zinc-300 hover:text-blue-400 transition-colors">{run.id.slice(0, 8)}</a>
                                        </td>
                                        <td className="p-3"><StatusBadge status={run.status} /></td>
                                        <td className="p-3 text-sm text-zinc-400 font-mono">{run.duration_ms ? formatDuration(run.duration_ms) : '—'}</td>
                                        <td className="p-3">
                                            {run.exit_code !== undefined && run.exit_code !== null ? (
                                                <span className={`text-sm font-mono px-1.5 py-0.5 rounded ${run.exit_code === 0 ? 'text-lime-400 bg-lime-dim' : 'text-orange-400 bg-coral-dim'}`}>{run.exit_code}</span>
                                            ) : <span className="text-sm text-zinc-600">—</span>}
                                        </td>
                                        <td className="p-3 text-xs text-zinc-500">{timeAgo(run.created_at)}</td>
                                        <td className="p-3 text-right">
                                            {isActive && (
                                                <div className="flex justify-end gap-1">
                                                    {run.status === 'running' && <button onClick={() => handleRunAction(run.id, 'pause')} className="btn-ghost btn-sm">⏸</button>}
                                                    {run.status === 'paused' && <button onClick={() => handleRunAction(run.id, 'resume')} className="btn-ghost btn-sm">▶</button>}
                                                    <button onClick={() => handleRunAction(run.id, 'kill')} className="btn-danger btn-sm">✕</button>
                                                </div>
                                            )}
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
}
