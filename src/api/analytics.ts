import api from './client';

const base = '/admin/analytics';

export const analyticsApi = {
  getDAU: (days = 30) => api.get(`${base}/dau`, { params: { days } }).then((r) => r.data.data),
  getNewUsers: (days = 30) => api.get(`${base}/new-users`, { params: { days } }).then((r) => r.data.data),
  getPopularFeatures: () => api.get(`${base}/popular-features`).then((r) => r.data.data),
  getPayments: (from?: string, to?: string) =>
    api.get(`${base}/payments`, { params: { from, to } }).then((r) => r.data.data),
  getSubscriptionStats: () =>
    api.get(`${base}/subscription-stats`).then((r) => r.data),
};
