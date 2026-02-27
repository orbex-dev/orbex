'use client';

import { useEffect, useState } from 'react';
import { api, Job, CreateJobRequest } from '@/lib/api';
import { EmptyState, timeAgo } from '@/components/ui';

export default function JobsPage() {
    const [jobs, setJobs] = useState<Job[]>([]);
    const [loading, setLoading] = useState(true);
    const [showCreate, setShowCreate] = useState(false);
    const [formData, setFormData] = useState<CreateJobRequest>({
        name: '', image: '', command: [], timeout_seconds: 3600,
    });

    async function loadJobs() {
        try {
            setJobs(await api.listJobs());
        } catch (err) {
            console.error('Failed to load jobs:', err);
        } finally {
            setLoading(false);
        }
    }

    useEffect(() => { loadJobs(); }, []);

    async function handleCreate(e: React.FormEvent) {
        e.preventDefault();
        try {
            const data = { ...formData };
            if (typeof data.command === 'string') {
                data.command = (data.command as unknown as string).split(' ').filter(Boolean);
            }
            await api.createJob(data);
            setShowCreate(false);
            setFormData({ name: '', image: '', command: [], timeout_seconds: 3600 });
            loadJobs();
        } catch (err) {
            alert(`Failed to create job: ${err}`);
        }
    }

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="w-8 h-8 border-2 border-violet-400 border-t-transparent rounded-full animate-spin" />
            </div>
        );
    }

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Jobs</h1>
                <button
                    onClick={() => setShowCreate(!showCreate)}
                    className="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm font-medium rounded-lg transition-colors"
                >
                    + New Job
                </button>
            </div>

            {/* Create form */}
            {showCreate && (
                <form onSubmit={handleCreate} className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 mb-6 space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-xs text-zinc-400 mb-1">Name</label>
                            <input
                                type="text" required value={formData.name}
                                onChange={e => setFormData({ ...formData, name: e.target.value })}
                                className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-200 focus:border-violet-500 focus:outline-none"
                                placeholder="my-etl-job"
                            />
                        </div>
                        <div>
                            <label className="block text-xs text-zinc-400 mb-1">Docker Image</label>
                            <input
                                type="text" required value={formData.image}
                                onChange={e => setFormData({ ...formData, image: e.target.value })}
                                className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-200 focus:border-violet-500 focus:outline-none"
                                placeholder="python:3.12"
                            />
                        </div>
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-xs text-zinc-400 mb-1">Command</label>
                            <input
                                type="text"
                                value={Array.isArray(formData.command) ? formData.command.join(' ') : formData.command}
                                onChange={e => setFormData({ ...formData, command: e.target.value as unknown as string[] })}
                                className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-200 focus:border-violet-500 focus:outline-none"
                                placeholder="python run.py (optional)"
                            />
                        </div>
                        <div>
                            <label className="block text-xs text-zinc-400 mb-1">Timeout (seconds)</label>
                            <input
                                type="number"
                                value={formData.timeout_seconds}
                                onChange={e => setFormData({ ...formData, timeout_seconds: parseInt(e.target.value) || 3600 })}
                                className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-200 focus:border-violet-500 focus:outline-none"
                            />
                        </div>
                    </div>
                    <div>
                        <label className="block text-xs text-zinc-400 mb-1">Cron Schedule (optional)</label>
                        <input
                            type="text"
                            value={formData.schedule || ''}
                            onChange={e => setFormData({ ...formData, schedule: e.target.value || undefined })}
                            className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-200 focus:border-violet-500 focus:outline-none"
                            placeholder="*/5 * * * * (every 5 minutes)"
                        />
                    </div>
                    <div className="flex gap-2">
                        <button type="submit" className="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm rounded-lg transition-colors">
                            Create Job
                        </button>
                        <button type="button" onClick={() => setShowCreate(false)} className="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-sm rounded-lg transition-colors">
                            Cancel
                        </button>
                    </div>
                </form>
            )}

            {/* Jobs list */}
            {jobs.length === 0 ? (
                <EmptyState
                    title="No jobs yet"
                    description="Create your first job to get started."
                    action={
                        <button onClick={() => setShowCreate(true)} className="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm rounded-lg transition-colors">
                            Create Job
                        </button>
                    }
                />
            ) : (
                <div className="space-y-3">
                    {jobs.map(job => (
                        <a key={job.id} href={`/jobs/${job.id}`}
                            className="flex items-center justify-between bg-zinc-900 border border-zinc-800 rounded-xl p-5 hover:border-zinc-700 transition-colors group"
                        >
                            <div>
                                <h3 className="font-medium text-zinc-200 group-hover:text-violet-400 transition-colors">{job.name}</h3>
                                <div className="flex items-center gap-3 mt-1">
                                    <span className="text-xs text-zinc-500 font-mono">{job.image}</span>
                                    {job.schedule && (
                                        <span className="text-xs px-2 py-0.5 rounded-full bg-cyan-400/10 text-cyan-400">
                                            ⏰ {job.schedule}
                                        </span>
                                    )}
                                    {job.webhook_token && (
                                        <span className="text-xs px-2 py-0.5 rounded-full bg-violet-400/10 text-violet-400">
                                            🔗 webhook
                                        </span>
                                    )}
                                </div>
                            </div>
                            <div className="text-right">
                                <p className="text-xs text-zinc-500">{timeAgo(job.created_at)}</p>
                                <p className="text-xs text-zinc-600 font-mono mt-0.5">{job.id.slice(0, 8)}</p>
                            </div>
                        </a>
                    ))}
                </div>
            )}
        </div>
    );
}
