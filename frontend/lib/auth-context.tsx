'use client';

import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { apiMe, clearTokens, getToken, setTokens, type AuthTokens, type User } from './api';

interface AuthContextType { user: User | null; isAuthenticated: boolean; isLoading: boolean; login: (tokens: AuthTokens) => void; logout: () => void; }
const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
	const [user, setUser] = useState<User | null>(null);
	const [isLoading, setIsLoading] = useState(true);
	useEffect(() => { let mounted = true; (async () => { if (!getToken()) { if (mounted) setIsLoading(false); return; } try { const profile = await apiMe(); if (mounted) setUser(profile); } catch { clearTokens(); } finally { if (mounted) setIsLoading(false); } })(); return () => { mounted = false; }; }, []);
	const value = useMemo(() => ({ user, isAuthenticated: Boolean(user), isLoading, login: (tokens: AuthTokens) => { setTokens(tokens); setUser(tokens.user); setIsLoading(false); }, logout: () => { clearTokens(); setUser(null); setIsLoading(false); } }), [user, isLoading]);
	return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
export function useAuth() { const context = useContext(AuthContext); if (!context) throw new Error('useAuth must be used within an AuthProvider'); return context; }