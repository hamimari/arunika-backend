import { create } from 'zustand';
import { authApi } from '../api/auth';

interface AuthState {
  accessToken: string | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  initFromStorage: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  isAuthenticated: false,

  initFromStorage: () => {
    const token = localStorage.getItem('admin_access_token');
    set({ accessToken: token, isAuthenticated: !!token });
  },

  login: async (email, password) => {
    const res = await authApi.login(email, password);
    localStorage.setItem('admin_access_token', res.access_token);
    localStorage.setItem('admin_refresh_token', res.refresh_token);
    set({ accessToken: res.access_token, isAuthenticated: true });
  },

  logout: async () => {
    try {
      await authApi.logout();
    } catch {
      // best effort
    }
    localStorage.removeItem('admin_access_token');
    localStorage.removeItem('admin_refresh_token');
    localStorage.removeItem('admin_id');
    set({ accessToken: null, isAuthenticated: false });
  },
}));
