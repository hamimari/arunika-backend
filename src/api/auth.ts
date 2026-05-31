import api from './client';

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
}

export const authApi = {
  login: async (email: string, password: string): Promise<LoginResponse> => {
    const { data } = await api.post('/admin/auth/login', { email, password });
    return data;
  },

  refresh: async (adminId: string, refreshToken: string): Promise<{ access_token: string }> => {
    const { data } = await api.post('/admin/auth/refresh', {
      admin_id: adminId,
      refresh_token: refreshToken,
    });
    return data;
  },

  logout: async (): Promise<void> => {
    await api.post('/admin/auth/logout');
  },
};
