'use client';

import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiMe, clearTokens, getStoredUser, getToken, setStoredUser, setTokens, type AuthTokens, type User } from './api';
import { connectWebSocket, disconnectWebSocket } from './ws';

interface AuthContextType { user: User | null; isAuthenticated: boolean; isLoading: boolean; login: (tokens: AuthTokens) => void; logout: () => void; }
const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
	const [user, setUser] = useState<User | null>(() => getStoredUser());
	const [isLoading, setIsLoading] = useState<boolean>(() => {
		if (typeof window === 'undefined') return true;
		const token = getToken();
		const stored = getStoredUser();
		return !token || !stored;
	});
	const queryClient = useQueryClient();

	useEffect(() => {
		let mounted = true;
		(async () => {
			const token = getToken();
			if (!token) {
				if (mounted) {
					setUser(null);
					setIsLoading(false);
				}
				return;
			}
			try {
				const profile = await apiMe();
				if (mounted) {
					setUser(profile);
					setStoredUser(profile);
					connectWebSocket(token);
				}
			} catch (err: any) {
				// Only clear tokens if auth explicitly returns 401
				if (err?.status === 401) {
					clearTokens();
					if (mounted) setUser(null);
				}
			} finally {
				if (mounted) setIsLoading(false);
			}
		})();
		return () => { mounted = false; };
	}, []);

	const value = useMemo(() => ({
		user,
		isAuthenticated: Boolean(user),
		isLoading,
		login: (tokens: AuthTokens) => {
			setTokens(tokens, tokens.user);
			setUser(tokens.user);
			setIsLoading(false);
			connectWebSocket(tokens.access_token);
		},
		logout: () => {
			clearTokens();
			disconnectWebSocket();
			queryClient.clear();
			setUser(null);
			setIsLoading(false);
		},
	}), [user, isLoading, queryClient]);

	return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
export function useAuth() { const context = useContext(AuthContext); if (!context) throw new Error('useAuth must be used within an AuthProvider'); return context; }