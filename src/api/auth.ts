import axios from 'axios';
import api from './client';

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  admin_id: string;
}

export interface RefreshResponse {
  access_token: string;
}

// Uses a bare axios call (not the shared `api` instance) so this request
// never passes through client.ts's own 401 interceptor — that interceptor
// calls back into this function, and reusing `api` here would recurse.
const baseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

export const authApi = {
  login: async (email: string, password: string): Promise<LoginResponse> => {
    const { data } = await api.post('/admin/auth/login', { email, password });
    return data;
  },

  logout: async (): Promise<void> => {
    await api.post('/admin/auth/logout');
  },

  refresh: async (adminId: string, refreshToken: string): Promise<RefreshResponse> => {
    const { data } = await axios.post(`${baseURL}/admin/auth/refresh`, {
      admin_id: adminId,
      refresh_token: refreshToken,
    });
    return data;
  },
};
