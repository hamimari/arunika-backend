import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import MockAdapter from 'axios-mock-adapter';
import api from '../../api/client';
import { analyticsApi } from '../../api/analytics';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

describe('analyticsApi', () => {
  it('getDAU calls /admin/analytics/dau with days param', async () => {
    mock.onGet('/admin/analytics/dau').reply(200, { data: [] });
    await analyticsApi.getDAU(7);
    expect(mock.history.get[0].url).toBe('/admin/analytics/dau');
    expect(mock.history.get[0].params).toMatchObject({ days: 7 });
  });

  it('getNewUsers calls /admin/analytics/new-users', async () => {
    mock.onGet('/admin/analytics/new-users').reply(200, { data: [] });
    await analyticsApi.getNewUsers(30);
    expect(mock.history.get[0].url).toBe('/admin/analytics/new-users');
  });

  it('getPopularFeatures calls /admin/analytics/popular-features', async () => {
    mock.onGet('/admin/analytics/popular-features').reply(200, { data: [] });
    await analyticsApi.getPopularFeatures();
    expect(mock.history.get[0].url).toBe('/admin/analytics/popular-features');
  });

  it('getPayments calls /admin/analytics/payments with date range', async () => {
    mock.onGet('/admin/analytics/payments').reply(200, { data: [] });
    await analyticsApi.getPayments('2026-01-01', '2026-01-31');
    const params = mock.history.get[0].params;
    expect(params).toMatchObject({ from: '2026-01-01', to: '2026-01-31' });
  });

  it('getSubscriptionStats calls /admin/analytics/subscription-stats', async () => {
    const stats = { total: 10, premium: 3, free: 7 };
    mock.onGet('/admin/analytics/subscription-stats').reply(200, { data: stats });
    const res = await analyticsApi.getSubscriptionStats();
    expect(mock.history.get[0].url).toBe('/admin/analytics/subscription-stats');
    expect(res.data).toMatchObject(stats);
  });
});
