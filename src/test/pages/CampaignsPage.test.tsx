import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import api from '../../api/client';
import CampaignsPage from '../../pages/campaigns/CampaignsPage';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <CampaignsPage />
    </QueryClientProvider>
  );
}

describe('CampaignsPage', () => {
  it('shows campaign history', async () => {
    mock.onGet('/admin/campaigns').reply(200, {
      data: [
        {
          id: 'c-1',
          title: 'Kartu AR baru!',
          body: 'Harimau Sumatera sudah rilis',
          image_url: '',
          channel: 'both',
          segment: 'all',
          link_type: 'ar_card',
          link_id: 'card-1',
          status: 'COMPLETED',
          sent: 120,
          failed: 3,
          error: '',
          created_at: '2026-09-01T00:00:00Z',
        },
      ],
      total: 1,
    });

    renderPage();

    expect(await screen.findByText('Kartu AR baru!')).toBeInTheDocument();
    expect(screen.getByText('COMPLETED')).toBeInTheDocument();
    expect(screen.getByText('3 failed')).toBeInTheDocument();
  });

  it('sends a device-wide push campaign after confirmation', async () => {
    mock.onGet('/admin/campaigns').reply(200, { data: [], total: 0 });
    mock.onPost('/admin/campaigns').reply(202, {
      data: { id: 'c-2', status: 'SENDING' },
    });
    const user = userEvent.setup();

    renderPage();

    await user.type(screen.getByPlaceholderText(/Kartu AR baru/), 'Dongeng baru!');
    await user.type(screen.getByPlaceholderText('Short message shown under the title'), 'Yuk dengarkan');
    await user.click(screen.getByRole('button', { name: 'Send Campaign' }));
    await user.click(await screen.findByRole('button', { name: 'Yes, Send' }));

    await waitFor(() => expect(mock.history.post.length).toBe(1));
    expect(JSON.parse(mock.history.post[0].data)).toEqual({
      title: 'Dongeng baru!',
      body: 'Yuk dengarkan',
      segment: 'all_devices',
      channel: 'push',
      link_type: 'none',
    });
  });
});
