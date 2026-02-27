'use client';

import { useEffect, useState } from 'react';
import { api, Job, JobRun } from '@/lib/api';
import { StatCard, StatusBadge, timeAgo, EmptyState } from '@/components/ui';

export default function OverviewPage() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [recentRuns, setRecentRuns] = useState<(JobRun & { jobName: string })[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const jobList = await api.listJobs();
        setJobs(jobList);

        // Fetch runs for each job and merge
        const allRuns: (JobRun & { jobName: string })[] = [];
        for (const job of jobList.slice(0, 10)) {
          try {
            const runs = await api.listRuns(job.id);
            for (const run of runs) {
              allRuns.push({ ...run, jobName: job.name });
            }
          } catch { /* skip */ }
        }
        allRuns.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
        setRecentRuns(allRuns.slice(0, 15));
      } catch (err) {
        console.error('Failed to load overview:', err);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const activeJobs = jobs.filter(j => j.is_active).length;
  const scheduledJobs = jobs.filter(j => j.schedule).length;
  const runningRuns = recentRuns.filter(r => r.status === 'running' || r.status === 'paused').length;
  const recentSucceeded = recentRuns.filter(r => r.status === 'succeeded').length;
  const recentFailed = recentRuns.filter(r => r.status === 'failed').length;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="w-8 h-8 border-2 border-violet-400 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Overview</h1>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <StatCard label="Total Jobs" value={jobs.length} sub={`${scheduledJobs} scheduled`} />
        <StatCard label="Active" value={activeJobs} />
        <StatCard label="Running Now" value={runningRuns} />
        <StatCard label="Recent" value={`${recentSucceeded}/${recentSucceeded + recentFailed}`} sub="success rate" />
      </div>

      {/* Recent runs */}
      <h2 className="text-lg font-semibold mb-4">Recent Runs</h2>
      {recentRuns.length === 0 ? (
        <EmptyState title="No runs yet" description="Trigger a job run to see activity here." />
      ) : (
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="text-xs text-zinc-500 uppercase tracking-wider border-b border-zinc-800">
                <th className="text-left p-4">Job</th>
                <th className="text-left p-4">Status</th>
                <th className="text-left p-4">Duration</th>
                <th className="text-left p-4">When</th>
              </tr>
            </thead>
            <tbody>
              {recentRuns.map(run => (
                <tr key={run.id} className="border-b border-zinc-800/50 hover:bg-zinc-800/30 transition-colors">
                  <td className="p-4">
                    <a href={`/dashboard/runs/${run.id}`} className="text-sm font-medium text-zinc-200 hover:text-violet-400 transition-colors">
                      {run.jobName}
                    </a>
                    <p className="text-xs text-zinc-600 font-mono mt-0.5">{run.id.slice(0, 8)}</p>
                  </td>
                  <td className="p-4"><StatusBadge status={run.status} /></td>
                  <td className="p-4 text-sm text-zinc-400">
                    {run.duration_ms ? `${(run.duration_ms / 1000).toFixed(1)}s` : '—'}
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
