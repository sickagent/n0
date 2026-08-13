import { createContext } from 'react';
import type { AuthSession, LoginPayload, RegisterPayload } from '../types';

export type AuthContextValue = {
  session: AuthSession | null;
  isAuthenticated: boolean;
  isReady: boolean;
  login: (payload: LoginPayload) => Promise<void>;
  register: (payload: RegisterPayload) => Promise<void>;
  logout: () => void;
};

export const AuthContext = createContext<AuthContextValue | null>(null);
