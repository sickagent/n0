import { api } from './client';
import type { AuthSession, LoginPayload, RegisterPayload } from '../types';

type LoginResponse = AuthSession;
type RegisterResponse = { user_id: string };

export const authApi = {
  login: async (payload: LoginPayload) => {
    const { data } = await api.post<LoginResponse>('/v1/auth/login', payload);
    return data;
  },

  register: async (payload: RegisterPayload) => {
    const { data } = await api.post<RegisterResponse>('/v1/auth/register', payload);
    return data;
  },
};
