'use client';

import { usePathname } from 'next/navigation';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useState, useEffect, useRef, useCallback } from 'react';
import { api, Job, searchJobs } from '@/lib/api';
import { AuthProvider, useAuth } from '@/lib/auth';

// ─── Icons ─────────────────────────────────────────────────
const icons = {
    overview: <svg className="w-[18px] h-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.75}><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z" /></svg>,
    jobs: <svg className="w-[18px] h-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.75}><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 14.15v4.25c0 1.094-.787 2.036-1.872 2.18-2.087.277-4.216.42-6.378.42s-4.291-.143-6.378-.42c-1.085-.144-1.872-1.086-1.872-2.18v-4.25m16.5 0a2.18 2.18 0 00.75-1.661V8.706c0-1.081-.768-2.015-1.837-2.175a48.114 48.114 0 00-3.413-.387m4.5 8.006c-.194.165-.42.295-.673.38A23.978 23.978 0 0112 15.75c-2.648 0-5.195-.429-7.577-1.22a2.016 2.016 0 01-.673-.38m0 0A2.18 2.18 0 013 12.489V8.706c0-1.081.768-2.015 1.837-2.175a48.111 48.111 0 013.413-.387m7.5 0V5.25A2.25 2.25 0 0013.5 3h-3a2.25 2.25 0 00-2.25 2.25v.894m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>,
    settings: <svg className="w-[18px] h-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.75}><path strokeLinecap="round" strokeLinejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z" /><path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>,
    search: <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" /></svg>,
    collapse: <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" /></svg>,
    logout: <svg className="w-[18px] h-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.75}><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15m3 0l3-3m0 0l-3-3m3 3H9" /></svg>,
};

const navItems = [
    { name: 'Overview', href: '/dashboard', icon: icons.overview },
    { name: 'Jobs', href: '/dashboard/jobs', icon: icons.jobs },
];

const bottomItems = [
    { name: 'Settings', href: '/dashboard/settings', icon: icons.settings },
];

// ─── Command Palette ───────────────────────────────────────
function CommandPalette({ onClose }: { onClose: () => void }) {
    const [query, setQuery] = useState('');
    const [jobs, setJobs] = useState<Job[]>([]);
    const [selected, setSelected] = useState(0);
    const inputRef = useRef<HTMLInputElement>(null);
    const router = useRouter();

    useEffect(() => {
        inputRef.current?.focus();
        api.listJobs().then(setJobs).catch(() => { });
    }, []);

    const filtered = searchJobs(jobs, query);
    const results = [
        // Navigation commands
        ...(!query ? [
            { type: 'nav' as const, label: 'Go to Overview', href: '/dashboard' },
            { type: 'nav' as const, label: 'Go to Jobs', href: '/dashboard/jobs' },
            { type: 'nav' as const, label: 'Go to Settings', href: '/dashboard/settings' },
        ] : []),
        // Job results
        ...filtered.slice(0, 8).map(j => ({
            type: 'job' as const,
            label: j.name,
            href: `/dashboard/jobs/${j.id}`,
            meta: j.image,
        })),
    ];

    function handleSelect(idx: number) {
        const item = results[idx];
        if (item) {
            router.push(item.href);
            onClose();
        }
    }

    function handleKeyDown(e: React.KeyboardEvent) {
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            setSelected(s => Math.min(s + 1, results.length - 1));
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            setSelected(s => Math.max(s - 1, 0));
        } else if (e.key === 'Enter') {
            e.preventDefault();
            handleSelect(selected);
        } else if (e.key === 'Escape') {
            onClose();
        }
    }

    return (
        <div className="cmd-backdrop" onClick={onClose}>
            <div className="fixed top-[20%] left-1/2 -translate-x-1/2 w-full max-w-lg z-[101]" onClick={e => e.stopPropagation()}>
                <div className="card animate-scale-in overflow-hidden" style={{ background: 'var(--surface-1)' }}>
                    {/* Input */}
                    <div className="flex items-center gap-3 px-4 py-3 border-b" style={{ borderColor: 'var(--border)' }}>
                        <span className="text-zinc-500">{icons.search}</span>
                        <input
                            ref={inputRef}
                            value={query}
                            onChange={e => { setQuery(e.target.value); setSelected(0); }}
                            onKeyDown={handleKeyDown}
                            placeholder="Search jobs, navigate..."
                            className="flex-1 bg-transparent text-sm text-zinc-200 placeholder-zinc-600 outline-none"
                        />
                        <kbd className="text-[10px] text-zinc-600 bg-zinc-800 px-1.5 py-0.5 rounded font-mono">esc</kbd>
                    </div>
                    {/* Results */}
                    <div className="max-h-72 overflow-y-auto py-1">
                        {results.length === 0 ? (
                            <div className="px-4 py-6 text-center text-xs text-zinc-500">No results found</div>
                        ) : (
                            results.map((item, i) => (
                                <button
                                    key={i}
                                    onClick={() => handleSelect(i)}
                                    onMouseEnter={() => setSelected(i)}
                                    className={`w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors ${i === selected ? 'bg-zinc-800/80' : 'hover:bg-zinc-800/50'
                                        }`}
                                >
                                    <span className="text-zinc-500">
                                        {item.type === 'nav' ? (
                                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" /></svg>
                                        ) : (
                                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M21 7.5l-2.25-1.313M21 7.5v2.25m0-2.25l-2.25 1.313M3 7.5l2.25-1.313M3 7.5l2.25 1.313M3 7.5v2.25m9 3l2.25-1.313M12 12.75l-2.25-1.313M12 12.75V15m0 6.75l2.25-1.313M12 21.75V19.5m0 2.25l-2.25-1.313m0-16.875L12 2.25l2.25 1.313M21 14.25v2.25l-2.25 1.313m-13.5 0L3 16.5v-2.25" /></svg>
                                        )}
                                    </span>
                                    <div className="flex-1 min-w-0">
                                        <span className="text-sm text-zinc-200 truncate block">{item.label}</span>
                                        {'meta' in item && item.meta && (
                                            <span className="text-[11px] text-zinc-500 font-mono truncate block">{item.meta}</span>
                                        )}
                                    </div>
                                    {i === selected && (
                                        <kbd className="text-[10px] text-zinc-600 bg-zinc-800 px-1.5 py-0.5 rounded font-mono">↵</kbd>
                                    )}
                                </button>
                            ))
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
}

// ─── Dashboard Shell ───────────────────────────────────────
function DashboardShell({ children }: { children: React.ReactNode }) {
    const pathname = usePathname();
    const { user, loading, logout } = useAuth();
    const [collapsed, setCollapsed] = useState(false);
    const [cmdOpen, setCmdOpen] = useState(false);

    const isActive = (href: string) => {
        if (href === '/dashboard') return pathname === '/dashboard';
        return pathname.startsWith(href);
    };

    // ⌘K keyboard shortcut
    useEffect(() => {
        function handleKey(e: KeyboardEvent) {
            if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
                e.preventDefault();
                setCmdOpen(o => !o);
            }
        }
        window.addEventListener('keydown', handleKey);
        return () => window.removeEventListener('keydown', handleKey);
    }, []);

    // Remember collapsed state
    useEffect(() => {
        const saved = localStorage.getItem('sidebar_collapsed');
        if (saved === 'true') setCollapsed(true);
    }, []);
    const toggleCollapse = useCallback(() => {
        setCollapsed(c => {
            localStorage.setItem('sidebar_collapsed', String(!c));
            return !c;
        });
    }, []);

    // Auth loading state
    if (loading) {
        return (
            <div className="min-h-screen flex items-center justify-center" style={{ background: 'var(--surface-0)' }}>
                <div className="flex items-center gap-3">
                    <svg className="w-5 h-5 animate-spin text-blue-400" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                    </svg>
                    <span className="text-sm text-zinc-500">Loading...</span>
                </div>
            </div>
        );
    }

    // Not authenticated — useAuth already redirects, but show nothing while redirecting
    if (!user) {
        return null;
    }

    const sidebarW = collapsed ? 'w-[64px]' : 'w-[240px]';
    const mainMl = collapsed ? 'ml-[64px]' : 'ml-[240px]';

    return (
        <div className="flex min-h-screen" style={{ background: 'var(--surface-0)' }}>
            {/* Command Palette */}
            {cmdOpen && <CommandPalette onClose={() => setCmdOpen(false)} />}

            {/* Sidebar */}
            <aside className={`${sidebarW} border-r fixed h-full flex flex-col transition-all duration-200`} style={{ borderColor: 'var(--border)', background: 'var(--surface-0)' }}>
                {/* Brand + collapse */}
                <div className={`px-3 py-4 flex items-center border-b ${collapsed ? 'justify-center' : 'justify-between'}`} style={{ borderColor: 'var(--border)' }}>
                    <Link href="/" className="flex items-center gap-2.5 group">
                        <div className="w-8 h-8 rounded-lg bg-blue-600 flex items-center justify-center shrink-0 group-hover:bg-blue-500 transition-colors">
                            <span className="text-white text-sm font-bold">O</span>
                        </div>
                        {!collapsed && (
                            <span className="text-[15px] font-bold text-zinc-100 tracking-tight">orbex</span>
                        )}
                    </Link>
                    {!collapsed && (
                        <button onClick={toggleCollapse} className="text-zinc-500 hover:text-zinc-300 transition-colors p-1" title="Collapse sidebar">
                            {icons.collapse}
                        </button>
                    )}
                </div>

                {/* Search trigger */}
                {!collapsed && (
                    <div className="px-3 py-3">
                        <button
                            onClick={() => setCmdOpen(true)}
                            className="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-[12px] text-zinc-500 hover:text-zinc-400 transition-colors"
                            style={{ background: 'var(--surface-2)', border: '1px solid var(--border)' }}
                        >
                            {icons.search}
                            <span className="flex-1 text-left">Search...</span>
                            <kbd className="text-[10px] text-zinc-600 bg-zinc-800 px-1 py-0.5 rounded font-mono">⌘K</kbd>
                        </button>
                    </div>
                )}
                {collapsed && (
                    <div className="px-3 py-2 space-y-1.5">
                        <button
                            onClick={toggleCollapse}
                            className="w-full flex justify-center p-2 rounded-lg text-zinc-500 hover:text-zinc-300 transition-colors"
                            title="Expand sidebar"
                        >
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" /></svg>
                        </button>
                        <button
                            onClick={() => setCmdOpen(true)}
                            className="w-full flex justify-center p-2 rounded-lg text-zinc-500 hover:text-zinc-300 transition-colors"
                            style={{ background: 'var(--surface-2)', border: '1px solid var(--border)' }}
                            title="Search (⌘K)"
                        >
                            {icons.search}
                        </button>
                    </div>
                )}

                {/* Nav */}
                <nav className="flex-1 px-2 py-1 space-y-0.5">
                    {!collapsed && (
                        <p className="text-[10px] font-semibold text-zinc-600 uppercase tracking-[0.1em] px-3 mb-1.5 mt-1">Platform</p>
                    )}
                    {navItems.map((item) => {
                        const active = isActive(item.href);
                        return (
                            <Link
                                key={item.href}
                                href={item.href}
                                title={item.name}
                                className={`flex items-center gap-3 rounded-lg text-[13px] font-medium transition-all duration-150 ${collapsed ? 'justify-center p-2.5' : 'px-3 py-2'
                                    } ${active
                                        ? 'bg-blue-500/10 text-blue-400'
                                        : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/60'
                                    }`}
                            >
                                <span className={active ? 'text-blue-400' : 'text-zinc-500'}>{item.icon}</span>
                                {!collapsed && item.name}
                                {!collapsed && active && <span className="ml-auto w-1.5 h-1.5 rounded-full bg-blue-400" />}
                            </Link>
                        );
                    })}
                </nav>

                {/* Bottom */}
                <div className="px-2 pb-3 space-y-0.5">
                    <div className="border-t mb-2" style={{ borderColor: 'var(--border)' }} />
                    {bottomItems.map((item) => {
                        const active = isActive(item.href);
                        return (
                            <Link
                                key={item.href}
                                href={item.href}
                                title={item.name}
                                className={`flex items-center gap-3 rounded-lg text-[13px] font-medium transition-all duration-150 ${collapsed ? 'justify-center p-2.5' : 'px-3 py-2'
                                    } ${active
                                        ? 'bg-blue-500/10 text-blue-400'
                                        : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/60'
                                    }`}
                            >
                                <span className={active ? 'text-blue-400' : 'text-zinc-500'}>{item.icon}</span>
                                {!collapsed && item.name}
                            </Link>
                        );
                    })}
                    {/* Logout */}
                    <button
                        onClick={logout}
                        title="Logout"
                        className={`flex items-center gap-3 rounded-lg text-[13px] font-medium transition-all duration-150 w-full text-zinc-400 hover:text-red-400 hover:bg-red-500/5 ${collapsed ? 'justify-center p-2.5' : 'px-3 py-2'
                            }`}
                    >
                        <span className="text-zinc-500">{icons.logout}</span>
                        {!collapsed && 'Logout'}
                    </button>
                    {!collapsed && (
                        <div className="px-3 pt-1">
                            <p className="text-[10px] text-zinc-600 truncate">{user.email}</p>
                            <span className="text-[10px] text-zinc-600 font-mono">v0.1.0-beta</span>
                        </div>
                    )}
                </div>
            </aside>

            {/* Main content — centered with mx-auto */}
            <main className={`flex-1 ${mainMl} bg-grid min-h-screen transition-all duration-200`}>
                <div className="p-8 max-w-6xl mx-auto">
                    {children}
                </div>
            </main>
        </div>
    );
}

// ─── Dashboard Layout (with AuthProvider) ──────────────────
export default function DashboardLayout({ children }: { children: React.ReactNode }) {
    return (
        <AuthProvider>
            <DashboardShell>{children}</DashboardShell>
        </AuthProvider>
    );
}
