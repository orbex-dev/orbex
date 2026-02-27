'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { api, Job, JobRun } from '@/lib/api';
import { StatusBadge, timeAgo, formatDuration } from '@/components/ui';

export default function JobDetailPage() {
    const params = useParams();
    const jobId = params.id as string;
    const [job, setJob] = useState<Job | null>(null);
    const [runs, setRuns] = useState<JobRun[]>([]);
    const [loading, setLoading] = useState(true);

    async function loadData() {
        try {
            const [jobData, runsData] = await Promise.all([
                api.getJob(jobId),
                api.listRuns(jobId),
            ]);
            setJob(jobData);
            setRuns(runsData);
        } catch (err) {
            console.error('Failed to load job:', err);
        } finally {
            setLoading(false);
        }
    }

    useEffect(() => { loadData(); }, [jobId]);

    async function handleTrigger() {
        try {
            await api.triggerRun(jobId);
            setTimeout(loadData, 1000);
        } catch (err) {
            alert(`Failed to trigger run: ${err}`);
        }
    }

    async function handleGenerateWebhook() {
        try {
            const result = await api.generateWebhook(jobId);
            alert(`Webhook URL: ${window.location.origin}${result.trigger_url}\n\nToken: ${result.webhook_token}`);
            loadData();
        } catch (err) {
            alert(`Failed to generate webhook: ${err}`);
        }
    }

    async function handleDelete() {
        if (!confirm('Are you sure you want to delete this job?')) return;
        try {
            await api.deleteJob(jobId);
            window.location.href = '/jobs';
        } catch (err) {
            alert(`Failed to delete: ${err}`);
        }
    }

    if (loading || !job) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="w-8 h-8 border-2 border-violet-400 border-t-transparent rounded-full animate-spin" />
            </div>
        );
    }

    return (
        <div>
            {/* Header */}
            <div className="flex items-start justify-between mb-6">
                <div>
                    <a href="/jobs" className="text-xs text-zinc-500 hover:text-zinc-400 transition-colors">← Back to Jobs</a>
                    <h1 className="text-2xl font-bold mt-1">{job.name}</h1>
                    <p className="text-sm text-zinc-500 font-mono">{job.image}</p>
                </div>
                <div className="flex gap-2">
                    <button onClick={handleTrigger} className="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm font-medium rounded-lg transition-colors">
                        ▶ Run Now
                    </button>
                    <button onClick={handleGenerateWebhook} className="px-3 py-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-sm rounded-lg transition-colors">
                        🔗 Webhook
                    </button>
                    <button onClick={handleDelete} className="px-3 py-2 bg-zinc-800 hover:bg-red-900/50 text-zinc-400 hover:text-red-400 text-sm rounded-lg transition-colors">
                        🗑
                    </button>
                </div>
            </div>

            {/* Job config */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-8">
                {[
                    { label: 'Memory', value: `${job.memory_mb}MB` },
                    { label: 'CPU', value: `${job.cpu_millicores / 1000} cores` },
                    { label: 'Timeout', value: `${job.timeout_seconds}s` },
                    { label: 'Schedule', value: job.schedule || 'Manual' },
                ].map(item => (
                    <div key={item.label} className="bg-zinc-900 border border-zinc-800 rounded-lg p-3">
                        <p className="text-xs text-zinc-500">{item.label}</p>
                        <p className="text-sm font-medium text-zinc-200 mt-0.5">{item.value}</p>
                    </div>
                ))}
            </div>

            {job.command && job.command.length > 0 && (
                <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4 mb-6">
                    <p className="text-xs text-zinc-500 mb-2">Command</p>
                    <code className="text-sm text-cyan-400 font-mono">{job.command.join(' ')}</code>
                </div>
            )}

            {/* Runs */}
            <h2 className="text-lg font-semibold mb-4">Run History</h2>
            {runs.length === 0 ? (
                <div className="text-center py-8 text-zinc-500 text-sm">No runs yet. Click &quot;Run Now&quot; to execute.</div>
            ) : (
                <div className="bg-zinc-900 border border-zinc-800 rounded-xl overflow-hidden">
                    <table className="w-full">
                        <thead>
                            <tr className="text-xs text-zinc-500 uppercase tracking-wider border-b border-zinc-800">
                                <th className="text-left p-4">ID</th>
                                <th className="text-left p-4">Status</th>
                                <th className="text-left p-4">Duration</th>
                                <th className="text-left p-4">Exit Code</th>
                                <th className="text-left p-4">When</th>
                            </tr>
                        </thead>
                        <tbody>
                            {runs.map(run => (
                                <tr key={run.id} className="border-b border-zinc-800/50 hover:bg-zinc-800/30 transition-colors">
                                    <td className="p-4">
                                        <a href={`/runs/${run.id}`} className="text-sm font-mono text-zinc-300 hover:text-violet-400 transition-colors">
                                            {run.id.slice(0, 8)}
                                        </a>
                                    </td>
                                    <td className="p-4"><StatusBadge status={run.status} /></td>
                                    <td className="p-4 text-sm text-zinc-400">
                                        {run.duration_ms ? formatDuration(run.duration_ms) : '—'}
                                    </td>
                                    <td className="p-4 text-sm text-zinc-400 font-mono">
                                        {run.exit_code !== undefined ? run.exit_code : '—'}
                                    </td>
                                    <td className="p-4 text-sm text-zinc-500">{timeAgo(run.created_at)}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
}
