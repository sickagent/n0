import { useMemo, useState } from 'react';
import type { PropsWithChildren } from 'react';
import { authApi } from '../api/auth';
import { getStoredSession, setStoredSession } from './session';
import type { AuthSession } from '../types';

import { AuthContext, type AuthContextValue } from './context';

export function AuthProvider({ children }: PropsWithChildren) {
  const [session, setSession] = useState<AuthSession | null>(() => getStoredSession());

  const value = useMemo<AuthContextValue>(() => ({
    session,
    isAuthenticated: !!session?.token,
    isReady: true,
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
  }), [session]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
