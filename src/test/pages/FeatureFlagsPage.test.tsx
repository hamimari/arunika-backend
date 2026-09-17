import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import api from '../../api/client';
import FeatureFlagsPage from '../../pages/settings/FeatureFlagsPage';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <FeatureFlagsPage />
    </QueryClientProvider>
  );
}

const flags = [
  {
    key: 'printable_cards',
    name: 'Kartu Printable',
    description: 'Printable AR card PDF download section on the home screen.',
    is_enabled: true,
    updated_at: '2026-09-01T00:00:00Z',
  },
  {
    key: 'qr_scan',
    name: 'Scan QR Kartu AR',
    description: 'Scan tab in the bottom navigation.',
    is_enabled: false,
    updated_at: '2026-09-01T00:00:00Z',
  },
];

describe('FeatureFlagsPage', () => {
  it('lists flags with their visibility', async () => {
    mock.onGet('/admin/feature-flags').reply(200, { data: flags });

    renderPage();

    expect(await screen.findByText('Kartu Printable')).toBeInTheDocument();
    expect(screen.getByText('Scan QR Kartu AR')).toBeInTheDocument();
    expect(screen.getByText('Visible')).toBeInTheDocument();
    expect(screen.getByText('Hidden')).toBeInTheDocument();
  });

  it('hides a feature when its switch is turned off', async () => {
    mock.onGet('/admin/feature-flags').reply(200, { data: flags });
    mock
      .onPatch('/admin/feature-flags/printable_cards')
      .reply(200, { data: { ...flags[0], is_enabled: false } });
    const user = userEvent.setup();

    renderPage();

    await user.click(await screen.findByRole('switch', { name: 'Toggle Kartu Printable' }));

    await waitFor(() => expect(mock.history.patch.length).toBe(1));
    expect(JSON.parse(mock.history.patch[0].data)).toEqual({ is_enabled: false });
  });
});
