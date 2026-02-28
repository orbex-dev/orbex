'use client';

// ─── Status Badge ──────────────────────────────────────────
const statusConfig: Record<string, { bg: string; text: string; dot: string; pulse?: boolean }> = {
    pending: { bg: 'bg-timeout-dim border-yellow-500/20', text: 'text-yellow-400', dot: 'bg-yellow-400' },
    running: { bg: 'bg-electric-dim border-blue-500/20', text: 'text-blue-400', dot: 'bg-blue-400', pulse: true },
    succeeded: { bg: 'bg-lime-dim border-lime-500/20', text: 'text-lime-400', dot: 'bg-lime-400' },
    failed: { bg: 'bg-coral-dim border-orange-500/20', text: 'text-orange-400', dot: 'bg-orange-400' },
    timed_out: { bg: 'bg-timeout-dim border-yellow-500/20', text: 'text-yellow-500', dot: 'bg-yellow-500' },
    paused: { bg: 'bg-violet-500/10 border-violet-500/20', text: 'text-violet-400', dot: 'bg-violet-400' },
    cancelled: { bg: 'bg-zinc-500/10 border-zinc-500/20', text: 'text-zinc-400', dot: 'bg-zinc-500' },
};

export function StatusBadge({ status }: { status: string }) {
    // Detect timeout from error message or explicit status
    const displayStatus = status === 'failed' ? status : status;
    const config = statusConfig[displayStatus] || statusConfig.cancelled;
    return (
        <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-semibold uppercase tracking-wider border ${config.bg} ${config.text}`}>
            <span className={`w-1.5 h-1.5 rounded-full ${config.dot} ${config.pulse ? 'pulse-dot' : ''}`} />
            {displayStatus.replace('_', ' ')}
        </span>
    );
}

// ─── Stat Card ─────────────────────────────────────────────
export function StatCard({ label, value, subtitle, icon, accent = 'electric' }: {
    label: string;
    value: string | number;
    subtitle?: string;
    icon: React.ReactNode;
    accent?: 'electric' | 'lime' | 'coral' | 'timeout';
}) {
    const accentMap = {
        electric: { bg: 'bg-electric-dim', border: 'border-blue-500/15', icon: 'text-blue-400 bg-blue-500/10', glow: 'glow-electric' },
        lime: { bg: 'bg-lime-dim', border: 'border-lime-500/15', icon: 'text-lime-400 bg-lime-500/10', glow: 'glow-lime' },
        coral: { bg: 'bg-coral-dim', border: 'border-orange-500/15', icon: 'text-orange-400 bg-orange-500/10', glow: 'glow-coral' },
        timeout: { bg: 'bg-timeout-dim', border: 'border-yellow-500/15', icon: 'text-yellow-400 bg-yellow-500/10', glow: '' },
    };
    const a = accentMap[accent];
    return (
        <div className={`card ${a.glow} p-5 animate-slide-up`}>
            <div className="flex items-start justify-between mb-3">
                <p className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">{label}</p>
                <div className={`w-8 h-8 rounded-lg ${a.icon} flex items-center justify-center`}>
                    {icon}
                </div>
            </div>
            <p className="text-3xl font-bold text-zinc-100 tracking-tight">{value}</p>
            {subtitle && <p className="text-[12px] text-zinc-500 mt-1">{subtitle}</p>}
        </div>
    );
}

// ─── Section Header ────────────────────────────────────────
export function SectionHeader({ title, subtitle, action }: {
    title: string;
    subtitle?: string;
    action?: React.ReactNode;
}) {
    return (
        <div className="flex items-center justify-between mb-4">
            <div>
                <h2 className="text-lg font-bold text-zinc-100 tracking-tight">{title}</h2>
                {subtitle && <p className="text-xs text-zinc-500 mt-0.5">{subtitle}</p>}
            </div>
            {action}
        </div>
    );
}

// ─── Empty State ───────────────────────────────────────────
export function EmptyState({ title, description, icon, action }: {
    title: string;
    description: string;
    icon?: React.ReactNode;
    action?: React.ReactNode;
}) {
    return (
        <div className="flex flex-col items-center justify-center py-16 animate-fade-in">
            <div className="w-14 h-14 rounded-2xl bg-zinc-800/50 border border-zinc-700/30 flex items-center justify-center mb-4">
                {icon || (
                    <svg className="w-6 h-6 text-zinc-600" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m6 4.125l2.25 2.25m0 0l2.25 2.25M12 13.875l2.25-2.25M12 13.875l-2.25 2.25M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
                    </svg>
                )}
            </div>
            <h3 className="text-sm font-semibold text-zinc-300 mb-1">{title}</h3>
            <p className="text-xs text-zinc-500 text-center max-w-xs mb-4">{description}</p>
            {action}
        </div>
    );
}

// ─── Skeleton ──────────────────────────────────────────────
export function Skeleton({ className }: { className?: string }) {
    return <div className={`skeleton ${className || 'h-4 w-full'}`} />;
}

// ─── Toast (reusable) ──────────────────────────────────────
import { useEffect } from 'react';

export function Toast({ message, type, onDone }: {
    message: string;
    type: 'success' | 'error';
    onDone: () => void;
}) {
    useEffect(() => {
        const t = setTimeout(onDone, 3000);
        return () => clearTimeout(t);
    }, [onDone]);

    return (
        <div className={`toast ${type === 'success' ? 'toast-success' : 'toast-error'}`}>
            {type === 'success' ? '✓ ' : '✕ '}{message}
        </div>
    );
}

// ─── Dialog ────────────────────────────────────────────────
export function Dialog({ children, onClose }: { children: React.ReactNode; onClose: () => void }) {
    return (
        <div className="dialog-overlay">
            <div className="dialog-backdrop" onClick={onClose} />
            <div className="dialog-content animate-scale-in">
                {children}
            </div>
        </div>
    );
}

// ─── Format Helpers ────────────────────────────────────────
export function formatDuration(ms: number): string {
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    const min = Math.floor(ms / 60000);
    const sec = Math.floor((ms % 60000) / 1000);
    return `${min}m ${sec}s`;
}

export function timeAgo(dateStr: string): string {
    const diff = Date.now() - new Date(dateStr).getTime();
    const minutes = Math.floor(diff / 60000);
    if (minutes < 1) return 'just now';
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
}
