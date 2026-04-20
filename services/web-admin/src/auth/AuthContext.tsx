import { createContext, useContext, useEffect, useMemo, useState } from 'react';
import type { PropsWithChildren } from 'react';
import { authApi } from '../api/auth';
import { getStoredSession, setStoredSession } from './session';
import type { AuthSession, LoginPayload, RegisterPayload } from '../types';

type AuthContextValue = {
  session: AuthSession | null;
  isAuthenticated: boolean;
  isReady: boolean;
  login: (payload: LoginPayload) => Promise<void>;
  register: (payload: RegisterPayload) => Promise<void>;
  logout: () => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const [session, setSession] = useState<AuthSession | null>(null);
  const [isReady, setIsReady] = useState(false);

  useEffect(() => {
    setSession(getStoredSession());
    setIsReady(true);
  }, []);

  const value = useMemo<AuthContextValue>(() => ({
    session,
    isAuthenticated: !!session?.token,
    isReady,
    login: async (payload) => {
      const nextSession = await authApi.login(payload);
      setStoredSession(nextSession);
      setSession(nextSession);
    },
    register: async (payload) => {
      await authApi.register(payload);
      const nextSession = await authApi.login({
        email: payload.email,
        password: payload.password,
      });
      setStoredSession(nextSession);
      setSession(nextSession);
    },
    logout: () => {
      setStoredSession(null);
      setSession(null);
    },
  }), [isReady, session]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}
