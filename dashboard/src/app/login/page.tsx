'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { auth } from '@/lib/api';

export default function LoginPage() {
    const router = useRouter();
    const [mode, setMode] = useState<'login' | 'register'>('login');
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');

    async function handleSubmit(e: React.FormEvent) {
        e.preventDefault();
        setError('');
        setLoading(true);
        try {
            if (mode === 'register') {
                await auth.register(email, password);
            }
            // Login sets httpOnly cookie automatically
            await auth.login(email, password);
            router.push('/dashboard');
        } catch (err: unknown) {
            setError(err instanceof Error ? err.message : String(err));
        } finally {
            setLoading(false);
        }
    }

    return (
        <div className="min-h-screen flex items-center justify-center bg-grid" style={{ background: 'var(--surface-0)' }}>
            <div className="w-full max-w-md mx-4 animate-scale-in">
                {/* Logo */}
                <div className="flex items-center justify-center gap-3 mb-8">
                    <div className="w-12 h-12 rounded-2xl bg-blue-600 flex items-center justify-center shadow-lg shadow-blue-600/20">
                        <span className="text-white text-xl font-bold">O</span>
                    </div>
                    <span className="text-2xl font-bold text-zinc-100 tracking-tight">orbex</span>
                </div>

                {/* Card */}
                <div className="card p-8">
                    <h2 className="text-xl font-bold text-zinc-100 text-center mb-1">
                        {mode === 'login' ? 'Welcome back' : 'Create account'}
                    </h2>
                    <p className="text-sm text-zinc-500 text-center mb-6">
                        {mode === 'login' ? 'Enter your credentials to continue' : 'Get started with Orbex'}
                    </p>

                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div>
                            <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">Email</label>
                            <input
                                type="email" required autoFocus
                                className="input" placeholder="you@example.com"
                                value={email} onChange={e => setEmail(e.target.value)}
                            />
                        </div>
                        <div>
                            <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">Password</label>
                            <input
                                type="password" required minLength={8}
                                className="input" placeholder="••••••••"
                                value={password} onChange={e => setPassword(e.target.value)}
                            />
                        </div>

                        {error && (
                            <div className="p-3 rounded-lg text-sm" style={{ background: 'rgba(239, 68, 68, 0.08)', border: '1px solid rgba(239, 68, 68, 0.2)', color: '#ef4444' }}>
                                {error}
                            </div>
                        )}

                        <button type="submit" disabled={loading} className="btn btn-primary w-full justify-center text-sm py-2.5">
                            {loading ? (
                                <span className="flex items-center gap-2">
                                    <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>
                                    {mode === 'login' ? 'Signing in...' : 'Creating account...'}
                                </span>
                            ) : (
                                mode === 'login' ? 'Sign In' : 'Create Account'
                            )}
                        </button>
                    </form>

                    <div className="mt-6 text-center">
                        <button
                            onClick={() => { setMode(mode === 'login' ? 'register' : 'login'); setError(''); }}
                            className="text-sm text-zinc-500 hover:text-zinc-300 transition-colors"
                        >
                            {mode === 'login' ? "Don't have an account? " : 'Already have an account? '}
                            <span className="text-blue-400 font-medium">{mode === 'login' ? 'Register' : 'Sign In'}</span>
                        </button>
                    </div>
                </div>

                <p className="text-center text-[11px] text-zinc-600 mt-6">
                    Orbex v0.1.0-beta · Run anything. Know everything.
                </p>
            </div>
        </div>
    );
}
