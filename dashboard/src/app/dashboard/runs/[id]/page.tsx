'use client';

import { useEffect, useState, useRef } from 'react';
import { useParams } from 'next/navigation';
import { api, JobRun } from '@/lib/api';
import { StatusBadge, Toast, Skeleton, formatDuration, timeAgo } from '@/components/ui';

export default function RunDetailPage() {
    const params = useParams();
    const runId = params.id as string;
    const [run, setRun] = useState<JobRun | null>(null);
    const [logs, setLogs] = useState('');
    const [loading, setLoading] = useState(true);
    const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);
    const [logSearch, setLogSearch] = useState('');
    const [autoScroll, setAutoScroll] = useState(true);
    const logRef = useRef<HTMLPreElement>(null);

    async function loadRun() {
        try {
            const [runData, logsData] = await Promise.all([
                api.getRun(runId),
                api.getRunLogs(runId),
            ]);
            setRun(runData);
            setLogs(logsData.logs || '');
        } catch (err) { console.error(err); }
        finally { setLoading(false); }
    }

    useEffect(() => { loadRun(); }, [runId]);

    // Auto-refresh for active runs
    useEffect(() => {
        if (!run) return;
        if (run.status !== 'running' && run.status !== 'paused') return;
        const interval = setInterval(loadRun, 3000);
        return () => clearInterval(interval);
    }, [run?.status]);

    // Auto-scroll logs
    useEffect(() => {
        if (autoScroll && logRef.current) {
            logRef.current.scrollTop = logRef.current.scrollHeight;
        }
    }, [logs, autoScroll]);

    async function handleAction(action: 'pause' | 'resume' | 'kill') {
        try {
            if (action === 'pause') await api.pauseRun(runId);
            else if (action === 'resume') await api.resumeRun(runId);
            else await api.killRun(runId);
            setToast({ message: `Run ${action}d`, type: 'success' });
            setTimeout(loadRun, 500);
        } catch (err) { setToast({ message: `Failed: ${err}`, type: 'error' }); }
    }

    function downloadLogs() {
        const blob = new Blob([logs], { type: 'text/plain' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `run-${runId.slice(0, 8)}.log`;
        a.click();
        URL.revokeObjectURL(url);
    }

    if (loading || !run) {
        return (
            <div>
                <Skeleton className="h-4 w-24 mb-2" />
                <Skeleton className="h-8 w-48 mb-6" />
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
                    {[1, 2, 3, 4].map(i => <Skeleton key={i} className="h-20 rounded-xl" />)}
                </div>
                <Skeleton className="h-64 rounded-xl" />
            </div>
        );
    }

    const isActive = run.status === 'running' || run.status === 'paused';
    const isTimedOut = run.error_message?.includes('timeout');
    const displayStatus = isTimedOut ? 'timed_out' : run.status;

    // Log lines with search highlighting
    const logLines = logs.split('\n');
    const filteredLines = logSearch
        ? logLines.filter(l => l.toLowerCase().includes(logSearch.toLowerCase()))
        : logLines;

    // Timeout progress
    let timeoutProgress = 0;
    if (run.status === 'running' && run.started_at) {
        const elapsed = (Date.now() - new Date(run.started_at).getTime()) / 1000;
        // We don't have timeout_seconds on run — estimate from job
        timeoutProgress = Math.min(elapsed / 3600, 1); // default 1h assumption
    }

    return (
        <div>
            {toast && <Toast message={toast.message} type={toast.type} onDone={() => setToast(null)} />}

            {/* Header */}
            <div className="flex items-start justify-between mb-6 animate-slide-up">
                <div>
                    <a href={`/dashboard/jobs/${run.job_id}`} className="inline-flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-400 transition-colors mb-1">
                        <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                        Back to Job
                    </a>
                    <h1 className="text-2xl font-bold text-zinc-100 tracking-tight">Run <span className="font-mono text-zinc-400">{runId.slice(0, 8)}</span></h1>
                </div>
                {isActive && (
                    <div className="flex gap-2">
                        {run.status === 'running' && <button onClick={() => handleAction('pause')} className="btn-ghost btn-sm">⏸ Pause</button>}
                        {run.status === 'paused' && <button onClick={() => handleAction('resume')} className="btn btn-primary btn-sm">▶ Resume</button>}
                        <button onClick={() => handleAction('kill')} className="btn-danger btn-sm">Kill</button>
                    </div>
                )}
            </div>

            {/* Status cards */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6 animate-slide-up delay-1">
                <div className="card p-4">
                    <p className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">Status</p>
                    <StatusBadge status={displayStatus} />
                </div>
                <div className="card p-4">
                    <p className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">Duration</p>
                    <p className="text-lg font-bold text-zinc-200 font-mono">{run.duration_ms ? formatDuration(run.duration_ms) : isActive ? 'Running...' : '—'}</p>
                </div>
                <div className="card p-4">
                    <p className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">Exit Code</p>
                    {run.exit_code !== undefined && run.exit_code !== null ? (
                        <span className={`text-lg font-bold font-mono px-2 py-0.5 rounded ${run.exit_code === 0 ? 'text-lime-400 bg-lime-dim' : 'text-orange-400 bg-coral-dim'}`}>{run.exit_code}</span>
                    ) : <p className="text-lg text-zinc-600">—</p>}
                </div>
                <div className="card p-4">
                    <p className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">Started</p>
                    <p className="text-sm text-zinc-300">{run.started_at ? timeAgo(run.started_at) : 'Pending'}</p>
                </div>
            </div>

            {/* Timeout progress (for running jobs) */}
            {run.status === 'running' && (
                <div className="card p-3 mb-4 animate-slide-up delay-2">
                    <div className="flex items-center justify-between text-[11px] text-zinc-500 mb-1">
                        <span>Timeout progress</span>
                        <span className="font-mono">{Math.round(timeoutProgress * 100)}%</span>
                    </div>
                    <div className="w-full h-1.5 rounded-full" style={{ background: 'var(--surface-3)' }}>
                        <div className={`h-full rounded-full transition-all duration-1000 ${timeoutProgress > 0.8 ? 'bg-orange-400' : 'bg-blue-400'}`} style={{ width: `${timeoutProgress * 100}%` }} />
                    </div>
                </div>
            )}

            {/* Error callout */}
            {run.error_message && (
                <div className={`card p-4 mb-4 border-l-2 animate-slide-up delay-2 ${isTimedOut ? 'border-l-yellow-500' : 'border-l-orange-500'}`}>
                    <p className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1">
                        {isTimedOut ? '⏱ Timed Out' : '✕ Error'}
                    </p>
                    <p className="text-sm text-orange-300 font-mono">{run.error_message}</p>
                </div>
            )}

            {/* Container info */}
            {run.container_id && (
                <div className="card p-3 mb-6 animate-slide-up delay-3">
                    <span className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">Container </span>
                    <span className="text-xs text-zinc-400 font-mono">{run.container_id}</span>
                </div>
            )}

            {/* Logs */}
            <div className="animate-slide-up delay-3">
                <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-3">
                        <h2 className="text-lg font-bold text-zinc-100 tracking-tight">Logs</h2>
                        {isActive && (
                            <span className="flex items-center gap-1.5 text-xs text-blue-400">
                                <span className="w-2 h-2 rounded-full bg-blue-400 pulse-dot" />
                                Live
                            </span>
                        )}
                    </div>
                    <div className="flex items-center gap-2">
                        {/* Search */}
                        <div className="relative">
                            <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" /></svg>
                            <input
                                className="input input-mono text-xs py-1 pl-8 w-40"
                                placeholder="Filter logs..."
                                value={logSearch} onChange={e => setLogSearch(e.target.value)}
                            />
                        </div>
                        <button onClick={() => navigator.clipboard.writeText(logs)} className="btn-ghost btn-sm">Copy</button>
                        <button onClick={downloadLogs} className="btn-ghost btn-sm">⬇ Download</button>
                        {isActive && (
                            <button onClick={() => setAutoScroll(!autoScroll)} className={`btn-sm ${autoScroll ? 'btn-ghost' : 'btn-ghost'}`}>
                                {autoScroll ? '⬇ Auto' : '⏸ Scroll'}
                            </button>
                        )}
                    </div>
                </div>

                <div className="card overflow-hidden" style={{ background: '#0a0a0c' }}>
                    <pre
                        ref={logRef}
                        className="font-mono text-xs text-zinc-400 p-4 overflow-auto max-h-[500px] leading-relaxed"
                        style={{ tabSize: 2 }}
                    >
                        {filteredLines.length === 0 ? (
                            <span className="text-zinc-600">{logs ? 'No matching lines' : 'Waiting for output...'}</span>
                        ) : (
                            filteredLines.map((line, i) => (
                                <div key={i} className="flex hover:bg-white/[0.02] group">
                                    <span className="select-none w-10 text-right pr-3 text-zinc-700 shrink-0">{i + 1}</span>
                                    <span className="flex-1 whitespace-pre-wrap break-all">
                                        {logSearch ? highlightMatch(line, logSearch) : line}
                                    </span>
                                    <button
                                        onClick={() => navigator.clipboard.writeText(line)}
                                        className="opacity-0 group-hover:opacity-100 text-zinc-600 hover:text-zinc-400 text-[10px] px-1 shrink-0"
                                    >copy</button>
                                </div>
                            ))
                        )}
                    </pre>
                </div>
            </div>
        </div>
    );
}

function highlightMatch(text: string, search: string): React.ReactNode {
    if (!search) return text;
    const parts = text.split(new RegExp(`(${search.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi'));
    return parts.map((part, i) =>
        part.toLowerCase() === search.toLowerCase()
            ? <mark key={i} className="bg-yellow-500/30 text-yellow-200 rounded px-0.5">{part}</mark>
            : part
    );
}
