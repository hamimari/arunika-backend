import api from './client';

export const usersApi = {
  list: (params: { search?: string; page?: number; per_page?: number }) =>
    api.get('/admin/users', { params }).then((r) => r.data),
  get: (id: string) => api.get(`/admin/users/${id}`).then((r) => r.data.data),
  updatePermission: (id: string, action: 'grant' | 'revoke', duration_days?: number) =>
    api.patch(`/admin/users/${id}/permission`, { action, duration_days }).then((r) => r.data),
};

export const paymentsApi = {
  list: (params: { status?: string; search?: string; page?: number; per_page?: number }) =>
    api.get('/admin/payments', { params }).then((r) => r.data),
  get: (id: string) => api.get(`/admin/payments/${id}`).then((r) => r.data.data),
};

export const campaignsApi = {
  dispatch: (payload: {
    title: string;
    body: string;
    channel: string;
    segment: string;
  }) => api.post('/admin/campaigns', payload).then((r) => r.data.data),
};

export interface PremiumPackage {
  id: string;
  name: string;
  subtitle: string;
  price_idr: number;
  type: 'content' | 'subscription';
  badge_label: string;
  is_best_value: boolean;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export type PremiumPackageInput = Omit<PremiumPackage, 'id' | 'created_at' | 'updated_at'>;

export const premiumPackagesApi = {
  list: (): Promise<{ data: PremiumPackage[] }> =>
    api.get('/admin/premium/packs').then((r) => r.data),
  create: (data: PremiumPackageInput): Promise<{ data: PremiumPackage }> =>
    api.post('/admin/premium/packs', data).then((r) => r.data),
  update: (id: string, data: PremiumPackageInput): Promise<{ data: PremiumPackage }> =>
    api.put(`/admin/premium/packs/${id}`, data).then((r) => r.data),
  remove: (id: string): Promise<void> =>
    api.delete(`/admin/premium/packs/${id}`).then(() => undefined),
  toggleVisibility: (id: string, isActive: boolean): Promise<{ data: PremiumPackage }> =>
    api.patch(`/admin/premium/packs/${id}/visibility`, { is_active: isActive }).then((r) => r.data),
};
