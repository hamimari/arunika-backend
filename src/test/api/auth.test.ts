import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import MockAdapter from 'axios-mock-adapter';
import api from '../../api/client';
import { authApi } from '../../api/auth';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

describe('authApi', () => {
  describe('login', () => {
    it('POSTs credentials and returns tokens', async () => {
      mock.onPost('/admin/auth/login').reply(200, {
        access_token: 'access123',
        refresh_token: 'refresh456',
      });
      const res = await authApi.login('admin@arunika.id', 'admin123');
      expect(res.access_token).toBe('access123');
      expect(res.refresh_token).toBe('refresh456');
      const body = JSON.parse(mock.history.post[0].data as string);
      expect(body).toEqual({ email: 'admin@arunika.id', password: 'admin123' });
    });

    it('rejects on invalid credentials (401)', async () => {
      mock.onPost('/admin/auth/login').reply(401, { error: 'invalid credentials' });
      await expect(authApi.login('bad@email.com', 'wrong')).rejects.toThrow();
    });
  });

  describe('refresh', () => {
    it('POSTs admin_id and refresh_token', async () => {
      mock.onPost('/admin/auth/refresh').reply(200, { access_token: 'newtoken' });
      const res = await authApi.refresh('admin-uuid', 'refresh456');
      expect(res.access_token).toBe('newtoken');
      const body = JSON.parse(mock.history.post[0].data as string);
      expect(body).toEqual({ admin_id: 'admin-uuid', refresh_token: 'refresh456' });
    });
  });

  describe('logout', () => {
    it('POSTs to /admin/auth/logout', async () => {
      mock.onPost('/admin/auth/logout').reply(200, {});
      await authApi.logout();
      expect(mock.history.post[0].url).toBe('/admin/auth/logout');
    });
  });
});
