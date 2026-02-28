'use client';

import { useState, useEffect, createContext, useContext, useCallback } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { auth, User } from '@/lib/api';

interface AuthContextType {
    user: User | null;
    loading: boolean;
    logout: () => Promise<void>;
    refresh: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType>({
    user: null,
    loading: true,
    logout: async () => { },
    refresh: async () => { },
});

export function useAuth() {
    return useContext(AuthContext);
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
    const [user, setUser] = useState<User | null>(null);
    const [loading, setLoading] = useState(true);
    const router = useRouter();
    const pathname = usePathname();

    const checkSession = useCallback(async () => {
        try {
            const me = await auth.getMe();
            setUser(me);
        } catch {
            setUser(null);
            // Only redirect if on a protected route
            if (pathname.startsWith('/dashboard')) {
                router.replace('/login');
            }
        } finally {
            setLoading(false);
        }
    }, [pathname, router]);

    useEffect(() => {
        checkSession();
    }, [checkSession]);

    const logout = useCallback(async () => {
        try {
            await auth.logout();
        } catch {
            // ignore
        }
        setUser(null);
        router.replace('/login');
    }, [router]);

    const refresh = useCallback(async () => {
        try {
            const me = await auth.getMe();
            setUser(me);
        } catch {
            setUser(null);
        }
    }, []);

    return (
        <AuthContext.Provider value={{ user, loading, logout, refresh }}>
            {children}
        </AuthContext.Provider>
    );
}
