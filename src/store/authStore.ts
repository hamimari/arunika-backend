import { create } from 'zustand';
import { authApi } from '../api/auth';

interface AuthState {
  accessToken: string | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  initFromStorage: () => void;
}

// Read synchronously at store-creation time (not in a useEffect) so the very
// first render already reflects a token that's already in localStorage.
// Deferring this to an effect meant ProtectedRoute's first render always saw
// isAuthenticated=false and bounced to /login before the effect could run —
// i.e. every hard page reload looked like a forced logout, independent of
// actual token expiry.
const storedToken = localStorage.getItem('admin_access_token');

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: storedToken,
  isAuthenticated: !!storedToken,

  initFromStorage: () => {
    const token = localStorage.getItem('admin_access_token');
    set({ accessToken: token, isAuthenticated: !!token });
  },

  login: async (email, password) => {
    const res = await authApi.login(email, password);
    localStorage.setItem('admin_access_token', res.access_token);
    localStorage.setItem('admin_refresh_token', res.refresh_token);
    localStorage.setItem('admin_id', res.admin_id);
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
