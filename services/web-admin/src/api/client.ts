import axios from 'axios';
import { getAccessToken, setStoredSession } from '../auth/session';

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '',
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error?.response?.status === 401) {
      setStoredSession(null);
    }
    const message = error?.response?.data || error?.message || 'Request failed';
    return Promise.reject(new Error(typeof message === 'string' ? message : JSON.stringify(message)));
  }
);
