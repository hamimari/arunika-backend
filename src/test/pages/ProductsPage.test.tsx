import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import MockAdapter from 'axios-mock-adapter';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import api from '../../api/client';
import ProductsPage from '../../pages/products/ProductsPage';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ProductsPage />
    </QueryClientProvider>
  );
}

describe('ProductsPage', () => {
  it('renders the product list', async () => {
    mock.onGet('/admin/products').reply(200, {
      data: [
        {
          id: 'prod-1',
          feature_id: 'feat-1',
          price_idr: 29000,
          is_active: true,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      ],
    });

    renderPage();

    expect(await screen.findByText('Rp 29.000')).toBeInTheDocument();
    expect(screen.getAllByText('Active').length).toBeGreaterThan(0);
  });

  it('shows an empty table when there are no products', async () => {
    mock.onGet('/admin/products').reply(200, { data: [] });

    renderPage();

    expect(await screen.findByText('Products')).toBeInTheDocument();
    expect(await screen.findByText('No data', { selector: 'div' })).toBeInTheDocument();
  });
});
