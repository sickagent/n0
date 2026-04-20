import type { AuthSession } from '../types';

const SESSION_KEY = 'n0.auth.session';

export function getStoredSession(): AuthSession | null {
  const raw = window.localStorage.getItem(SESSION_KEY);
  if (!raw) {
    return null;
  }

  try {
    return JSON.parse(raw) as AuthSession;
  } catch {
    window.localStorage.removeItem(SESSION_KEY);
    return null;
  }
}

export function setStoredSession(session: AuthSession | null) {
  if (!session) {
    window.localStorage.removeItem(SESSION_KEY);
    return;
  }
  window.localStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

export function getAccessToken(): string | null {
  return getStoredSession()?.token ?? null;
}
