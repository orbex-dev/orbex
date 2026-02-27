import { JobRun } from '@/lib/api';

const statusConfig: Record<string, { color: string; bg: string; dot: string }> = {
    pending: { color: 'text-yellow-400', bg: 'bg-yellow-400/10', dot: 'bg-yellow-400' },
    running: { color: 'text-blue-400', bg: 'bg-blue-400/10', dot: 'bg-blue-400 animate-pulse' },
    succeeded: { color: 'text-emerald-400', bg: 'bg-emerald-400/10', dot: 'bg-emerald-400' },
    failed: { color: 'text-red-400', bg: 'bg-red-400/10', dot: 'bg-red-400' },
    paused: { color: 'text-orange-400', bg: 'bg-orange-400/10', dot: 'bg-orange-400' },
    cancelled: { color: 'text-zinc-400', bg: 'bg-zinc-400/10', dot: 'bg-zinc-400' },
};

export function StatusBadge({ status }: { status: JobRun['status'] }) {
    const cfg = statusConfig[status] || statusConfig.pending;
    return (
        <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${cfg.color} ${cfg.bg}`}>
            <span className={`w-1.5 h-1.5 rounded-full ${cfg.dot}`} />
            {status}
        </span>
    );
}

export function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
    return (
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5">
            <p className="text-xs text-zinc-500 uppercase tracking-wider font-medium">{label}</p>
            <p className="text-3xl font-bold mt-2 bg-gradient-to-r from-zinc-100 to-zinc-300 bg-clip-text text-transparent">
                {value}
            </p>
            {sub && <p className="text-xs text-zinc-500 mt-1">{sub}</p>}
        </div>
    );
}

export function EmptyState({ title, description, action }: { title: string; description: string; action?: React.ReactNode }) {
    return (
        <div className="text-center py-16">
            <div className="w-16 h-16 mx-auto bg-zinc-800 rounded-2xl flex items-center justify-center mb-4">
                <svg className="w-8 h-8 text-zinc-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                </svg>
            </div>
            <h3 className="text-lg font-medium text-zinc-300">{title}</h3>
            <p className="text-sm text-zinc-500 mt-1 max-w-sm mx-auto">{description}</p>
            {action && <div className="mt-4">{action}</div>}
        </div>
    );
}

export function formatDuration(ms: number): string {
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    const mins = Math.floor(ms / 60000);
    const secs = Math.floor((ms % 60000) / 1000);
    return `${mins}m ${secs}s`;
}

export function timeAgo(date: string): string {
    const seconds = Math.floor((Date.now() - new Date(date).getTime()) / 1000);
    if (seconds < 60) return 'just now';
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
    return `${Math.floor(seconds / 86400)}d ago`;
}
