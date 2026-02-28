'use client';

import { useEffect, useState } from 'react';
import { api, Job, JobRun } from '@/lib/api';
import { StatusBadge, StatCard, SectionHeader, Skeleton, Toast, timeAgo, formatDuration } from '@/components/ui';

export default function DashboardOverview() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [runs, setRuns] = useState<(JobRun & { _jobName?: string })[]>([]);
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  async function load() {
    try {
      const jobsData = await api.listJobs();
      setJobs(jobsData);
      const allRuns: (JobRun & { _jobName?: string })[] = [];
      for (const job of jobsData.slice(0, 15)) {
        const jobRuns = await api.listRuns(job.id);
        allRuns.push(...jobRuns.map(r => ({ ...r, _jobName: job.name })));
      }
      allRuns.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
      setRuns(allRuns.slice(0, 20));
    } catch (err) {
      console.error('Failed to load:', err);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  const activeJobs = jobs.filter(j => j.is_active).length;
  const scheduledJobs = jobs.filter(j => j.schedule).length;
  const runningRuns = runs.filter(r => r.status === 'running').length;
  const totalRuns = runs.length;
  const successRuns = runs.filter(r => r.status === 'succeeded').length;
  const successRate = totalRuns > 0 ? Math.round((successRuns / totalRuns) * 100) : 0;

  // Quick actions on runs
  async function handleRunAction(runId: string, action: 'pause' | 'resume' | 'kill') {
    try {
      if (action === 'pause') await api.pauseRun(runId);
      else if (action === 'resume') await api.resumeRun(runId);
      else await api.killRun(runId);
      setToast({ message: `Run ${action}d`, type: 'success' });
      setTimeout(load, 500);
    } catch (err) {
      setToast({ message: `Failed: ${err}`, type: 'error' });
    }
  }

  if (loading) {
    return (
      <div>
        <Skeleton className="h-8 w-48 mb-2" />
        <Skeleton className="h-4 w-64 mb-8" />
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          {[1, 2, 3, 4].map(i => <Skeleton key={i} className="h-32 rounded-xl" />)}
        </div>
        <Skeleton className="h-6 w-32 mb-4" />
        {[1, 2, 3, 4, 5].map(i => <Skeleton key={i} className="h-16 rounded-xl mb-2" />)}
      </div>
    );
  }

  return (
    <div>
      {toast && <Toast message={toast.message} type={toast.type} onDone={() => setToast(null)} />}

      {/* Header */}
      <div className="mb-8 animate-slide-up">
        <h1 className="text-2xl font-bold text-zinc-100 tracking-tight">Dashboard</h1>
        <p className="text-sm text-zinc-500 mt-1">Monitor your jobs and runs in real-time</p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3 mb-8">
        <StatCard label="Total Jobs" value={jobs.length} subtitle={`${scheduledJobs} scheduled`} accent="electric"
          icon={<svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 14.15v4.25c0 1.094-.787 2.036-1.872 2.18-2.087.277-4.216.42-6.378.42s-4.291-.143-6.378-.42c-1.085-.144-1.872-1.086-1.872-2.18v-4.25m16.5 0a2.18 2.18 0 00.75-1.661V8.706c0-1.081-.768-2.015-1.837-2.175a48.114 48.114 0 00-3.413-.387m4.5 8.006c-.194.165-.42.295-.673.38A23.978 23.978 0 0112 15.75c-2.648 0-5.195-.429-7.577-1.22a2.016 2.016 0 01-.673-.38m0 0A2.18 2.18 0 013 12.489V8.706c0-1.081.768-2.015 1.837-2.175a48.111 48.111 0 013.413-.387m7.5 0V5.25A2.25 2.25 0 0013.5 3h-3a2.25 2.25 0 00-2.25 2.25v.894m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>} />
        <StatCard label="Active" value={activeJobs} subtitle="enabled" accent="lime"
          icon={<svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>} />
        <StatCard label="Running" value={runningRuns} subtitle="right now" accent="electric"
          icon={<svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.348a1.125 1.125 0 010 1.971l-11.54 6.347a1.125 1.125 0 01-1.667-.985V5.653z" /></svg>} />
        <StatCard label="Success Rate" value={`${successRate}%`} subtitle={`${successRuns}/${totalRuns} runs`} accent="lime"
          icon={<svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" /></svg>} />
      </div>

      {/* Activity Feed */}
      <SectionHeader title="Activity" subtitle="Recent job executions"
        action={<a href="/dashboard/jobs" className="text-xs text-blue-400 hover:text-blue-300 transition-colors font-medium">View all jobs →</a>}
      />

      {runs.length === 0 ? (
        <div className="card p-12 text-center animate-fade-in">
          <div className="w-12 h-12 rounded-xl bg-zinc-800/50 border border-zinc-700/30 flex items-center justify-center mx-auto mb-3">
            <svg className="w-6 h-6 text-zinc-600" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
          </div>
          <p className="text-sm text-zinc-400 font-medium">No activity yet</p>
          <p className="text-xs text-zinc-600 mt-1">Create a job and trigger a run to see activity here</p>
        </div>
      ) : (
        <div className="space-y-2">
          {runs.map((run, idx) => {
            const isRunActive = run.status === 'running' || run.status === 'paused';
            return (
              <div
                key={run.id}
                className={`card p-4 flex items-center gap-4 animate-slide-up`}
                style={{ animationDelay: `${idx * 30}ms`, opacity: 0 }}
              >
                {/* Status dot */}
                <StatusBadge status={run.status} />

                {/* Job + run info */}
                <div className="flex-1 min-w-0">
                  <a href={`/dashboard/runs/${run.id}`} className="text-sm font-medium text-zinc-200 hover:text-blue-400 transition-colors">
                    {run._jobName || run.job_id.slice(0, 8)}
                  </a>
                  <div className="flex items-center gap-3 mt-0.5 text-[11px] text-zinc-500">
                    <span className="font-mono">{run.id.slice(0, 8)}</span>
                    {run.duration_ms && <span>· {formatDuration(run.duration_ms)}</span>}
                    {run.exit_code !== undefined && run.exit_code !== null && (
                      <span>· exit <span className={run.exit_code === 0 ? 'text-lime-400' : 'text-orange-400'}>{run.exit_code}</span></span>
                    )}
                  </div>
                </div>

                {/* Quick actions for active runs */}
                {isRunActive && (
                  <div className="flex items-center gap-1">
                    {run.status === 'running' && (
                      <button onClick={() => handleRunAction(run.id, 'pause')} className="btn-ghost btn-sm" title="Pause">
                        <svg className="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><rect x="6" y="4" width="4" height="16" rx="1" /><rect x="14" y="4" width="4" height="16" rx="1" /></svg>
                      </button>
                    )}
                    {run.status === 'paused' && (
                      <button onClick={() => handleRunAction(run.id, 'resume')} className="btn-ghost btn-sm" title="Resume">
                        <svg className="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z" /></svg>
                      </button>
                    )}
                    <button onClick={() => handleRunAction(run.id, 'kill')} className="btn-danger btn-sm" title="Kill">
                      <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                    </button>
                  </div>
                )}

                {/* Time */}
                <span className="text-xs text-zinc-500 shrink-0">{timeAgo(run.created_at)}</span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
