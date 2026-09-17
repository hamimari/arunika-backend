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

export type CampaignChannel = 'push' | 'email' | 'both';
export type CampaignSegment = 'all_devices' | 'all' | 'subscribers';
export type CampaignLinkType = 'none' | 'ar_card' | 'dongeng';

export interface CampaignInput {
  title: string;
  body: string;
  channel: CampaignChannel;
  segment: CampaignSegment;
  image_url?: string;
  link_type: CampaignLinkType;
  link_id?: string;
}

export interface Campaign {
  id: string;
  title: string;
  body: string;
  image_url: string;
  channel: CampaignChannel;
  segment: CampaignSegment;
  link_type: CampaignLinkType;
  link_id: string;
  status: 'SENDING' | 'COMPLETED' | 'FAILED';
  sent: number;
  failed: number;
  error: string;
  created_at: string;
  completed_at?: string;
}

export const campaignsApi = {
  dispatch: (payload: CampaignInput): Promise<Campaign> =>
    api.post('/admin/campaigns', payload).then((r) => r.data.data),
  list: (params: { page?: number; per_page?: number }): Promise<{ data: Campaign[]; total: number }> =>
    api.get('/admin/campaigns', { params }).then((r) => r.data),
};

export interface FeatureFlag {
  key: string;
  name: string;
  description: string;
  is_enabled: boolean;
  updated_at: string;
}

export const featureFlagsApi = {
  list: (): Promise<{ data: FeatureFlag[] }> => api.get('/admin/feature-flags').then((r) => r.data),
  toggle: (key: string, isEnabled: boolean): Promise<{ data: FeatureFlag }> =>
    api.patch(`/admin/feature-flags/${key}`, { is_enabled: isEnabled }).then((r) => r.data),
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
  duration_days: number | null;
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

export interface Product {
  id: string;
  feature_id: string;
  feature_code: 'AR_CARD' | 'DONGENG' | '';
  display_name: string;
  content_id: string;
  price_idr: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateProductInput {
  feature_code: 'AR_CARD' | 'DONGENG';
  price_idr: number;
  ar_card_id?: string;
  dongeng_id?: string;
}

export const productsApi = {
  list: (): Promise<{ data: Product[] }> => api.get('/admin/products').then((r) => r.data),
  create: (data: CreateProductInput): Promise<{ data: Product }> =>
    api.post('/admin/products', data).then((r) => r.data),
  update: (id: string, priceIdr: number): Promise<{ data: Product }> =>
    api.put(`/admin/products/${id}`, { price_idr: priceIdr }).then((r) => r.data),
  remove: (id: string): Promise<void> =>
    api.delete(`/admin/products/${id}`).then(() => undefined),
  toggleActive: (id: string, isActive: boolean): Promise<void> =>
    api.patch(`/admin/products/${id}/active`, { is_active: isActive }).then(() => undefined),
};

export interface PremiumPackageItem {
  package_id: string;
  product_id: string;
  created_at: string;
}

export const packageItemsApi = {
  list: (packageId: string): Promise<{ data: PremiumPackageItem[] }> =>
    api.get(`/admin/premium/packs/${packageId}/items`).then((r) => r.data),
  add: (packageId: string, productId: string): Promise<void> =>
    api.post(`/admin/premium/packs/${packageId}/items`, { product_id: productId }).then(() => undefined),
  remove: (packageId: string, productId: string): Promise<void> =>
    api.delete(`/admin/premium/packs/${packageId}/items/${productId}`).then(() => undefined),
};

export interface Order {
  id: string;
  user_id: string;
  user_name: string;
  user_email: string;
  user_phone: string;
  product_id: string | null;
  product_name: string | null;
  package_id: string | null;
  package_name: string | null;
  amount_idr: number;
  status: 'PENDING' | 'PAID' | 'FAILED' | 'EXPIRED';
  created_at: string;
  updated_at: string;
}

export const ordersApi = {
  list: (params: { status?: string; search?: string; page?: number; per_page?: number }): Promise<{
    data: Order[];
    total: number;
  }> => api.get('/admin/orders', { params }).then((r) => r.data),
  sync: (id: string): Promise<{ data: Order }> =>
    api.post(`/admin/orders/${id}/sync`).then((r) => r.data),
};
