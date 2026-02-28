'use client';

import { useState } from 'react';
import { api, auth } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { Toast, SectionHeader, Dialog } from '@/components/ui';

export default function SettingsPage() {
    const { user } = useAuth();
    const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);
    const [showKeyDialog, setShowKeyDialog] = useState(false);
    const [newKey, setNewKey] = useState('');
    const [keyEmail, setKeyEmail] = useState('');
    const [keyPassword, setKeyPassword] = useState('');
    const [generating, setGenerating] = useState(false);
    const [currentPassword, setCurrentPassword] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [resetting, setResetting] = useState(false);

    async function handleGenerateKey() {
        setGenerating(true);
        try {
            const result = await api.generateApiKey(keyEmail, keyPassword);
            setNewKey(result.key);
            setToast({ message: 'API key generated', type: 'success' });
        } catch (err) {
            setToast({ message: `Failed: ${err instanceof Error ? err.message : err}`, type: 'error' });
        } finally {
            setGenerating(false);
        }
    }

    async function handlePasswordReset() {
        if (!currentPassword || !newPassword) return;
        if (newPassword.length < 8) {
            setToast({ message: 'New password must be at least 8 characters', type: 'error' });
            return;
        }
        setResetting(true);
        try {
            await auth.changePassword(currentPassword, newPassword);
            setToast({ message: 'Password updated successfully', type: 'success' });
            setCurrentPassword('');
            setNewPassword('');
        } catch (err) {
            setToast({ message: `Failed: ${err instanceof Error ? err.message : err}`, type: 'error' });
        } finally {
            setResetting(false);
        }
    }

    const apiEndpoint = typeof window !== 'undefined' ? `${window.location.protocol}//${window.location.hostname}:8080/api/v1` : 'http://localhost:8080/api/v1';

    return (
        <div>
            {toast && <Toast message={toast.message} type={toast.type} onDone={() => setToast(null)} />}

            {showKeyDialog && (
                <Dialog onClose={() => { setShowKeyDialog(false); setNewKey(''); setKeyEmail(''); setKeyPassword(''); }}>
                    <div className="p-6">
                        <h3 className="text-lg font-bold text-zinc-100 mb-1">Generate API Key</h3>
                        <p className="text-xs text-zinc-500 mb-4">API keys are for CLI and programmatic access. Confirm your credentials to generate one.</p>
                        {!newKey ? (
                            <div className="space-y-3">
                                <div>
                                    <label className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1 block">Email</label>
                                    <input className="input" type="email" value={keyEmail} onChange={e => setKeyEmail(e.target.value)} placeholder="you@example.com" />
                                </div>
                                <div>
                                    <label className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1 block">Password</label>
                                    <input className="input" type="password" value={keyPassword} onChange={e => setKeyPassword(e.target.value)} placeholder="••••••••" />
                                </div>
                            </div>
                        ) : (
                            <div>
                                <p className="text-xs text-orange-400 mb-2">⚠ Copy this key now — it won&apos;t be shown again.</p>
                                <div className="flex gap-2">
                                    <code className="flex-1 text-xs text-lime-400 font-mono p-3 rounded-lg overflow-x-auto" style={{ background: 'var(--surface-2)', border: '1px solid var(--border)' }}>
                                        {newKey}
                                    </code>
                                    <button onClick={() => { navigator.clipboard.writeText(newKey); setToast({ message: 'Copied', type: 'success' }); }} className="btn-ghost btn-sm">Copy</button>
                                </div>
                            </div>
                        )}
                    </div>
                    <div className="border-t px-6 py-4 flex justify-end gap-2" style={{ borderColor: 'var(--border)' }}>
                        <button onClick={() => { setShowKeyDialog(false); setNewKey(''); setKeyEmail(''); setKeyPassword(''); }} className="btn-ghost btn-sm">
                            {newKey ? 'Done' : 'Cancel'}
                        </button>
                        {!newKey && (
                            <button onClick={handleGenerateKey} disabled={generating || !keyEmail || !keyPassword} className="btn btn-primary btn-sm">
                                {generating ? 'Generating...' : 'Generate Key'}
                            </button>
                        )}
                    </div>
                </Dialog>
            )}

            <div className="mb-8 animate-slide-up">
                <h1 className="text-2xl font-bold text-zinc-100 tracking-tight">Settings</h1>
                <p className="text-sm text-zinc-500 mt-1">Manage your account and API access</p>
            </div>

            <div className="space-y-6 max-w-2xl">
                {/* Account */}
                <div className="card p-5 animate-slide-up delay-1">
                    <div className="flex items-center gap-3 mb-4">
                        <div className="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center">
                            <svg className="w-4 h-4 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z" /></svg>
                        </div>
                        <div>
                            <h3 className="text-sm font-semibold text-zinc-200">Account</h3>
                            <p className="text-xs text-zinc-500">Signed in via session cookie (httpOnly)</p>
                        </div>
                    </div>
                    <div className="space-y-2">
                        <div className="flex justify-between items-center py-1.5 border-b" style={{ borderColor: 'var(--border)' }}>
                            <span className="text-xs text-zinc-500">Email</span>
                            <span className="text-xs text-zinc-300 font-mono">{user?.email || '—'}</span>
                        </div>
                    </div>
                </div>

                {/* Password Reset */}
                <div className="card p-5 animate-slide-up delay-2">
                    <div className="flex items-center gap-3 mb-4">
                        <div className="w-8 h-8 rounded-lg bg-orange-500/10 flex items-center justify-center">
                            <svg className="w-4 h-4 text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" /></svg>
                        </div>
                        <div>
                            <h3 className="text-sm font-semibold text-zinc-200">Change Password</h3>
                            <p className="text-xs text-zinc-500">Update your account password</p>
                        </div>
                    </div>
                    <div className="space-y-3">
                        <div>
                            <label className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1 block">Current Password</label>
                            <input className="input" type="password" value={currentPassword} onChange={e => setCurrentPassword(e.target.value)} placeholder="••••••••" />
                        </div>
                        <div>
                            <label className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1 block">New Password</label>
                            <input className="input" type="password" value={newPassword} onChange={e => setNewPassword(e.target.value)} placeholder="Min. 8 characters" />
                        </div>
                        <button onClick={handlePasswordReset} disabled={resetting || !currentPassword || !newPassword} className="btn btn-primary btn-sm">
                            {resetting ? 'Updating...' : 'Update Password'}
                        </button>
                    </div>
                </div>

                {/* API Keys */}
                <div className="card p-5 animate-slide-up delay-2">
                    <div className="flex items-center gap-3 mb-4">
                        <div className="w-8 h-8 rounded-lg bg-violet-500/10 flex items-center justify-center">
                            <svg className="w-4 h-4 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" /></svg>
                        </div>
                        <div className="flex-1">
                            <h3 className="text-sm font-semibold text-zinc-200">API Keys</h3>
                            <p className="text-xs text-zinc-500">For CLI and programmatic access (not used by the dashboard)</p>
                        </div>
                        <button onClick={() => setShowKeyDialog(true)} className="btn btn-primary btn-sm">Generate Key</button>
                    </div>
                    <p className="text-xs text-zinc-500">Use API keys with <code className="text-blue-400">Authorization: Bearer obx_...</code> in your scripts, CI/CD, or the Orbex CLI.</p>
                </div>

                {/* API Endpoint */}
                <div className="card p-5 animate-slide-up delay-3">
                    <div className="flex items-center gap-3 mb-4">
                        <div className="w-8 h-8 rounded-lg bg-orange-500/10 flex items-center justify-center">
                            <svg className="w-4 h-4 text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" /></svg>
                        </div>
                        <div>
                            <h3 className="text-sm font-semibold text-zinc-200">API Endpoint</h3>
                            <p className="text-xs text-zinc-500">Backend server URL</p>
                        </div>
                    </div>
                    <code className="block text-sm text-blue-400 font-mono p-3 rounded-lg" style={{ background: 'var(--surface-2)', border: '1px solid var(--border)' }}>
                        {apiEndpoint}
                    </code>
                </div>

                {/* System Info */}
                <div className="card p-5 animate-slide-up delay-4">
                    <div className="flex items-center gap-3 mb-4">
                        <div className="w-8 h-8 rounded-lg bg-zinc-500/10 flex items-center justify-center">
                            <svg className="w-4 h-4 text-zinc-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" /></svg>
                        </div>
                        <div>
                            <h3 className="text-sm font-semibold text-zinc-200">System</h3>
                        </div>
                    </div>
                    <div className="space-y-2">
                        {[
                            ['Version', 'v0.1.0-beta'],
                            ['Dashboard', 'Next.js 16'],
                            ['Backend', 'Go + PostgreSQL'],
                            ['Runtime', 'Docker'],
                            ['Auth', 'Session cookies + API keys'],
                        ].map(([label, value]) => (
                            <div key={label} className="flex justify-between items-center py-1.5 border-b" style={{ borderColor: 'var(--border)' }}>
                                <span className="text-xs text-zinc-500">{label}</span>
                                <span className="text-xs text-zinc-300 font-mono">{value}</span>
                            </div>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    );
}
