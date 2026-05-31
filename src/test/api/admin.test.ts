import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import MockAdapter from 'axios-mock-adapter';
import api from '../../api/client';
import { usersApi, paymentsApi, campaignsApi } from '../../api/admin';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

describe('usersApi', () => {
  describe('list', () => {
    it('calls GET /admin/users with params', async () => {
      mock.onGet('/admin/users').reply(200, { data: [], total: 0, page: 1, per_page: 20 });
      const res = await usersApi.list({ page: 1, per_page: 20 });
      expect(res.total).toBe(0);
      expect(mock.history.get[0].url).toBe('/admin/users');
    });

    it('passes search param', async () => {
      mock.onGet('/admin/users').reply(200, { data: [], total: 0 });
      await usersApi.list({ search: 'budi' });
      expect(mock.history.get[0].params).toMatchObject({ search: 'budi' });
    });
  });

  describe('get', () => {
    it('calls GET /admin/users/:id', async () => {
      const user = { id: 'abc', name: 'Budi' };
      mock.onGet('/admin/users/abc').reply(200, { data: { user, subscription: null } });
      const res = await usersApi.get('abc');
      expect(res.user.name).toBe('Budi');
    });
  });

  describe('updatePermission', () => {
    it('sends duration_days (not days) when granting premium', async () => {
      mock.onPatch('/admin/users/abc/permission').reply(200, { message: 'ok' });
      await usersApi.updatePermission('abc', 'grant', 30);
      const body = JSON.parse(mock.history.patch[0].data as string);
      // Must send duration_days, NOT days
      expect(body).toHaveProperty('duration_days', 30);
      expect(body).not.toHaveProperty('days');
      expect(body.action).toBe('grant');
    });

    it('sends action=revoke without duration_days', async () => {
      mock.onPatch('/admin/users/abc/permission').reply(200, { message: 'ok' });
      await usersApi.updatePermission('abc', 'revoke');
      const body = JSON.parse(mock.history.patch[0].data as string);
      expect(body.action).toBe('revoke');
    });
  });
});

describe('paymentsApi', () => {
  it('calls GET /admin/payments', async () => {
    mock.onGet('/admin/payments').reply(200, { data: [], total: 0, page: 1, per_page: 20 });
    await paymentsApi.list({});
    expect(mock.history.get[0].url).toBe('/admin/payments');
  });

  it('passes status filter param', async () => {
    mock.onGet('/admin/payments').reply(200, { data: [], total: 0 });
    await paymentsApi.list({ status: 'settlement' });
    expect(mock.history.get[0].params).toMatchObject({ status: 'settlement' });
  });

  it('passes search param', async () => {
    mock.onGet('/admin/payments').reply(200, { data: [], total: 0 });
    await paymentsApi.list({ search: 'sub-001' });
    expect(mock.history.get[0].params).toMatchObject({ search: 'sub-001' });
  });

  it('calls GET /admin/payments/:id', async () => {
    mock.onGet('/admin/payments/abc-123').reply(200, { data: { id: 'abc-123' } });
    const res = await paymentsApi.get('abc-123');
    expect(res.id).toBe('abc-123');
    expect(mock.history.get[0].url).toBe('/admin/payments/abc-123');
  });
});

describe('campaignsApi', () => {
  it('POSTs to /admin/campaigns with correct shape', async () => {
    mock.onPost('/admin/campaigns').reply(200, { data: { sent: 100 } });
    const payload = { title: 'Promo', body: 'Msg', channel: 'push', segment: 'all' };
    await campaignsApi.dispatch(payload);
    const body = JSON.parse(mock.history.post[0].data as string);
    expect(body).toMatchObject(payload);
  });
});
