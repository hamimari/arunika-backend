import axios from 'axios';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080',
  headers: { 'Content-Type': 'application/json' },
});

// Attach admin JWT to every request.
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('admin_access_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

function logoutAndRedirect() {
  localStorage.removeItem('admin_access_token');
  localStorage.removeItem('admin_refresh_token');
  localStorage.removeItem('admin_id');
  window.location.href = '/login';
}

// The access token is short-lived (15 min). Rather than logging the admin
// out on every 401 — which happens routinely as tokens expire mid-session —
// try the refresh-token flow once and transparently retry the original
// request. Concurrent 401s while a refresh is already in flight share the
// same refresh promise instead of each firing their own refresh call.
let refreshPromise: Promise<string> | null = null;

async function refreshAccessToken(): Promise<string> {
  if (!refreshPromise) {
    refreshPromise = (async () => {
      // Lazy import avoids a circular-import cycle with auth.ts (which
      // imports `api` from this file).
      const { authApi } = await import('./auth');
      const adminId = localStorage.getItem('admin_id');
      const refreshToken = localStorage.getItem('admin_refresh_token');
      if (!adminId || !refreshToken) {
        throw new Error('No refresh token available');
      }
      const { access_token } = await authApi.refresh(adminId, refreshToken);
      localStorage.setItem('admin_access_token', access_token);
      return access_token;
    })().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

api.interceptors.response.use(
  (res) => res,
  async (err) => {
    const originalRequest = err.config;
    const isAuthEndpoint =
      originalRequest?.url?.includes('/admin/auth/login') ||
      originalRequest?.url?.includes('/admin/auth/refresh');

    if (err.response?.status === 401 && !isAuthEndpoint && !originalRequest._retried) {
      originalRequest._retried = true;
      try {
        const accessToken = await refreshAccessToken();
        originalRequest.headers = originalRequest.headers ?? {};
        originalRequest.headers.Authorization = `Bearer ${accessToken}`;
        return api(originalRequest);
      } catch {
        logoutAndRedirect();
        return Promise.reject(err);
      }
    }

    if (err.response?.status === 401) {
      logoutAndRedirect();
    }
    return Promise.reject(err);
  }
);

export default api;
