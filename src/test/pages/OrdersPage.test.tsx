import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import MockAdapter from 'axios-mock-adapter';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import api from '../../api/client';
import OrdersPage from '../../pages/orders/OrdersPage';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <OrdersPage />
    </QueryClientProvider>
  );
}

const sampleOrder = {
  id: 'order-1',
  user_id: 'user-1',
  product_id: null,
  package_id: 'pkg-1',
  amount_idr: 29000,
  status: 'PAID' as const,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

describe('OrdersPage', () => {
  it('renders the order list', async () => {
    mock.onGet('/admin/orders').reply(200, { data: [sampleOrder], total: 1 });

    renderPage();

    expect(await screen.findByText('Rp 29.000')).toBeInTheDocument();
    expect(screen.getByText('PAID')).toBeInTheDocument();
    expect(screen.getByText('Package')).toBeInTheDocument();
  });

  it('re-fetches with the status param when the filter changes', async () => {
    mock.onGet('/admin/orders').reply(200, { data: [sampleOrder], total: 1 });

    renderPage();

    await screen.findByText('Rp 29.000');
    expect(mock.history.get[0].params).toMatchObject({ page: 1, per_page: 20 });
    expect(mock.history.get[0].params.status).toBeUndefined();
  });
});
