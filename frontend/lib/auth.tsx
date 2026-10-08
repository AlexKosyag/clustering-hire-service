'use client';

import { createContext, ReactNode, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { api, storage } from './api';

export interface AuthUser {
  id: string;
  email: string;
  role: 'CANDIDATE' | 'EMPLOYER';
  isEmailVerified?: boolean;
  createdAt?: string;
}

interface AuthContextValue {
  user: AuthUser | null;
  ready: boolean;
  login: (email: string, password: string) => Promise<AuthUser>;
  register: (email: string, password: string, role: 'CANDIDATE' | 'EMPLOYER') => Promise<AuthUser>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue>(null as unknown as AuthContextValue);

export const useAuth = () => useContext(AuthContext);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    setUser(storage.user<AuthUser>());
    setReady(true);
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const { data } = await api.post('/auth/login', { email, password });
    storage.save(data);
    setUser(data.user);
    return data.user as AuthUser;
  }, []);

  const register = useCallback(async (email: string, password: string, role: 'CANDIDATE' | 'EMPLOYER') => {
    const { data } = await api.post('/auth/register', { email, password, role });
    storage.save(data);
    setUser(data.user);
    return data.user as AuthUser;
  }, []);

  const logout = useCallback(() => {
    storage.clear();
    setUser(null);
  }, []);

  const value = useMemo(() => ({ user, ready, login, register, logout }), [user, ready, login, register, logout]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
