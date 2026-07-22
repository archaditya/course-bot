'use client';

import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { getToken, setTokens, clearTokens, type AuthTokens } from './api';

interface AuthContextType {
  user: { id: string; email: string } | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (tokens: AuthTokens) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<{ id: string; email: string } | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const token = getToken();
    if (token) {
      const storedUser = localStorage.getItem('user');
      if (storedUser) {
        try {
          setUser(JSON.parse(storedUser));
        } catch {
          setUser(null);
        }
      }
    }
    setIsLoading(false);
  }, []);

  const login = (tokens: AuthTokens) => {
    setTokens(tokens);
    setUser(tokens.user);
    localStorage.setItem('user', JSON.stringify(tokens.user));
    setIsLoading(false);
  };

  const logout = () => {
    clearTokens();
    setUser(null);
    localStorage.removeItem('user');
    setIsLoading(false);
  };

  return (
    <AuthContext.Provider value={{ user, isAuthenticated: !!user, isLoading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
