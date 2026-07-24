'use client';

import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiMe, clearTokens, getToken, setTokens, type AuthTokens, type User } from './api';
import { connectWebSocket, disconnectWebSocket } from './ws';

interface AuthContextType { user: User | null; isAuthenticated: boolean; isLoading: boolean; login: (tokens: AuthTokens) => void; logout: () => void; }
const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
	const [user, setUser] = useState<User | null>(null);
	const [isLoading, setIsLoading] = useState(true);
	const queryClient = useQueryClient();

	useEffect(() => {
		let mounted = true;
		(async () => {
			const token = getToken();
			if (!token) { if (mounted) setIsLoading(false); return; }
			try {
				const profile = await apiMe();
				if (mounted) { setUser(profile); connectWebSocket(token); }
			} catch {
				clearTokens();
			} finally {
				if (mounted) setIsLoading(false);
			}
		})();
		return () => { mounted = false; };
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const value = useMemo(() => ({
		user,
		isAuthenticated: Boolean(user),
		isLoading,
		login: (tokens: AuthTokens) => {
			setTokens(tokens);
			setUser(tokens.user);
			setIsLoading(false);
			connectWebSocket(tokens.access_token);
		},
		logout: () => {
			clearTokens();
			disconnectWebSocket();
			// Wipe every cached query (projects, conversations, chunks…) so the
			// next person to sign in on this browser/tab never sees a flash of
			// the previous user's data.
			queryClient.clear();
			setUser(null);
			setIsLoading(false);
		},
	}), [user, isLoading, queryClient]);

	return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
export function useAuth() { const context = useContext(AuthContext); if (!context) throw new Error('useAuth must be used within an AuthProvider'); return context; }