'use client';

import { useEffect, useState, useRef } from 'react';
import { useParams } from 'next/navigation';
import { api, JobRun } from '@/lib/api';
import { StatusBadge, formatDuration } from '@/components/ui';

export default function RunDetailPage() {
    const params = useParams();
    const runId = params.id as string;
    const [run, setRun] = useState<JobRun | null>(null);
    const [logs, setLogs] = useState<string>('');
    const [loading, setLoading] = useState(true);
    const logRef = useRef<HTMLPreElement>(null);
    const intervalRef = useRef<NodeJS.Timeout | null>(null);

    async function loadRun() {
        try {
            const [runData, logsData] = await Promise.all([
                api.getRun(runId),
                api.getRunLogs(runId).catch(() => ({ logs: '' })),
            ]);
            setRun(runData);
            setLogs(logsData.logs || '');

            // Stop polling when run is finished
            if (['succeeded', 'failed', 'cancelled'].includes(runData.status)) {
                if (intervalRef.current) clearInterval(intervalRef.current);
            }
        } catch (err) {
            console.error('Failed to load run:', err);
        } finally {
            setLoading(false);
        }
    }

    useEffect(() => {
        loadRun();
        // Poll every 2s for live updates
        intervalRef.current = setInterval(loadRun, 2000);
        return () => {
            if (intervalRef.current) clearInterval(intervalRef.current);
        };
    }, [runId]);

    useEffect(() => {
        if (logRef.current) {
            logRef.current.scrollTop = logRef.current.scrollHeight;
        }
    }, [logs]);

    async function handlePause() {
        try {
            await api.pauseRun(runId);
            loadRun();
        } catch (err) { alert(`Failed: ${err}`); }
    }

    async function handleResume() {
        try {
            await api.resumeRun(runId);
            loadRun();
        } catch (err) { alert(`Failed: ${err}`); }
    }

    async function handleKill() {
        if (!confirm('Kill this run?')) return;
        try {
            await api.killRun(runId);
            loadRun();
        } catch (err) { alert(`Failed: ${err}`); }
    }

    if (loading || !run) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="w-8 h-8 border-2 border-violet-400 border-t-transparent rounded-full animate-spin" />
            </div>
        );
    }

    const isActive = run.status === 'running' || run.status === 'paused';

    return (
        <div>
            {/* Header */}
            <div className="flex items-start justify-between mb-6">
                <div>
                    <a href={`/dashboard/jobs/${run.job_id}`} className="text-xs text-zinc-500 hover:text-zinc-400 transition-colors">← Back to Job</a>
                    <h1 className="text-2xl font-bold mt-1">
                        Run <span className="font-mono text-zinc-400">{run.id.slice(0, 8)}</span>
                    </h1>
                </div>
                {isActive && (
                    <div className="flex gap-2">
                        {run.status === 'running' && (
                            <button onClick={handlePause} className="px-3 py-2 bg-orange-600/20 hover:bg-orange-600/30 text-orange-400 text-sm rounded-lg transition-colors border border-orange-600/30">
                                ⏸ Pause
                            </button>
                        )}
                        {run.status === 'paused' && (
                            <button onClick={handleResume} className="px-3 py-2 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 text-sm rounded-lg transition-colors border border-blue-600/30">
                                ▶ Resume
                            </button>
                        )}
                        <button onClick={handleKill} className="px-3 py-2 bg-red-600/20 hover:bg-red-600/30 text-red-400 text-sm rounded-lg transition-colors border border-red-600/30">
                            ✕ Kill
                        </button>
                    </div>
                )}
            </div>

            {/* Status + info */}
            <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-6">
                <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-3">
                    <p className="text-xs text-zinc-500">Status</p>
                    <div className="mt-1"><StatusBadge status={run.status} /></div>
                </div>
                <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-3">
                    <p className="text-xs text-zinc-500">Duration</p>
                    <p className="text-sm font-medium text-zinc-200 mt-1">
                        {run.duration_ms ? formatDuration(run.duration_ms) : isActive ? '...' : '—'}
                    </p>
                </div>
                <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-3">
                    <p className="text-xs text-zinc-500">Exit Code</p>
                    <p className={`text-sm font-mono mt-1 ${run.exit_code === 0 ? 'text-emerald-400' : run.exit_code ? 'text-red-400' : 'text-zinc-400'}`}>
                        {run.exit_code !== undefined ? run.exit_code : '—'}
                    </p>
                </div>
                <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-3">
                    <p className="text-xs text-zinc-500">Started</p>
                    <p className="text-xs text-zinc-200 mt-1">
                        {run.started_at ? new Date(run.started_at).toLocaleTimeString() : '—'}
                    </p>
                </div>
                <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-3">
                    <p className="text-xs text-zinc-500">Container</p>
                    <p className="text-xs text-zinc-400 font-mono mt-1 truncate">
                        {run.container_id ? run.container_id.slice(0, 12) : '—'}
                    </p>
                </div>
            </div>

            {/* Error message */}
            {run.error_message && (
                <div className="bg-red-950/30 border border-red-800/30 rounded-lg p-4 mb-6">
                    <p className="text-xs text-red-400 font-medium mb-1">Error</p>
                    <p className="text-sm text-red-300">{run.error_message}</p>
                </div>
            )}

            {/* Logs */}
            <div className="mb-6">
                <div className="flex items-center justify-between mb-3">
                    <h2 className="text-lg font-semibold">Logs</h2>
                    {isActive && (
                        <span className="text-xs text-blue-400 flex items-center gap-1">
                            <span className="w-2 h-2 bg-blue-400 rounded-full animate-pulse" />
                            Live
                        </span>
                    )}
                </div>
                <pre
                    ref={logRef}
                    className="log-viewer bg-zinc-900 border border-zinc-800 rounded-xl p-4 max-h-[500px] overflow-auto text-zinc-300 whitespace-pre-wrap"
                >
                    {logs || (isActive ? 'Waiting for output...' : 'No logs captured.')}
                </pre>
            </div>
        </div>
    );
}
