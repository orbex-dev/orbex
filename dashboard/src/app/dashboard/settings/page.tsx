'use client';

import { useState, useEffect } from 'react';
import { getStoredApiKey, setApiKey } from '@/lib/api';

export default function SettingsPage() {
    const [key, setKey] = useState('');
    const [saved, setSaved] = useState(false);

    useEffect(() => {
        setKey(getStoredApiKey());
    }, []);

    function handleSave() {
        setApiKey(key);
        setSaved(true);
        setTimeout(() => setSaved(false), 2000);
    }

    return (
        <div>
            <h1 className="text-2xl font-bold mb-6">Settings</h1>

            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 max-w-xl">
                <h2 className="text-lg font-medium mb-4">API Key</h2>
                <p className="text-sm text-zinc-500 mb-4">
                    Enter your Orbex API key to connect the dashboard to your account.
                    Generate one via <code className="text-violet-400">POST /api/v1/auth/api-keys</code>.
                </p>
                <div className="flex gap-2">
                    <input
                        type="password"
                        value={key}
                        onChange={e => setKey(e.target.value)}
                        className="flex-1 px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-200 font-mono focus:border-violet-500 focus:outline-none"
                        placeholder="obx_..."
                    />
                    <button
                        onClick={handleSave}
                        className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${saved
                                ? 'bg-emerald-600 text-white'
                                : 'bg-violet-600 hover:bg-violet-500 text-white'
                            }`}
                    >
                        {saved ? '✓ Saved' : 'Save'}
                    </button>
                </div>
            </div>

            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 max-w-xl mt-6">
                <h2 className="text-lg font-medium mb-4">API Endpoint</h2>
                <p className="text-sm text-zinc-500 mb-2">
                    The dashboard connects to:
                </p>
                <code className="text-sm text-cyan-400 font-mono">
                    {process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1'}
                </code>
                <p className="text-xs text-zinc-600 mt-2">
                    Set <code className="text-zinc-400">NEXT_PUBLIC_API_URL</code> env var to change.
                </p>
            </div>
        </div>
    );
}
