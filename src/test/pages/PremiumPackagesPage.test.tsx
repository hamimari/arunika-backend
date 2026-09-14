import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import api from '../../api/client';
import PremiumPackagesPage from '../../pages/packages/PremiumPackagesPage';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <PremiumPackagesPage />
    </QueryClientProvider>
  );
}

const samplePack = {
  id: 'pkg-1',
  name: 'Paket Hutan',
  subtitle: '8 Hewan Hutan + 2 Dongeng',
  price_idr: 29000,
  type: 'content',
  badge_label: '',
  is_best_value: false,
  is_active: true,
  sort_order: 1,
  duration_days: null,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

describe('PremiumPackagesPage', () => {
  it('renders the package list', async () => {
    mock.onGet('/admin/premium/packs').reply(200, { data: [samplePack] });

    renderPage();

    expect(await screen.findByText('Paket Hutan')).toBeInTheDocument();
    expect(screen.getByText('Rp 29.000')).toBeInTheDocument();
  });

  it('shows an error alert when the list fails to load', async () => {
    mock.onGet('/admin/premium/packs').reply(500);

    renderPage();

    expect(await screen.findByText('Failed to load packages')).toBeInTheDocument();
  });

  it('creates a new package via the Add Package modal', async () => {
    mock.onGet('/admin/premium/packs').reply(200, { data: [] });
    mock.onPost('/admin/premium/packs').reply(201, {
      data: { ...samplePack, id: 'new-pkg', name: 'Paket Baru' },
    });

    const user = userEvent.setup();
    renderPage();

    await screen.findByRole('button', { name: /add package/i });
    await user.click(screen.getByRole('button', { name: /add package/i }));

    const modal = await screen.findByRole('dialog');
    await user.type(within(modal).getByLabelText('Name'), 'Paket Baru');
    await user.type(within(modal).getByLabelText('Subtitle'), 'Deskripsi baru');
    await user.type(within(modal).getByLabelText('Price (IDR)'), '15000');

    // Ant Design Select: open the dropdown then pick "Content".
    await user.click(within(modal).getByLabelText('Type'));
    const contentOption = await screen.findByTitle('Content');
    await user.click(contentOption);

    await user.click(within(modal).getByRole('button', { name: 'Create' }));

    await waitFor(() => expect(mock.history.post.length).toBe(1));
    const body = JSON.parse(mock.history.post[0].data as string);
    expect(body).toMatchObject({
      name: 'Paket Baru',
      subtitle: 'Deskripsi baru',
      price_idr: 15000,
      type: 'content',
    });
  });
});
