import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import api from '../../api/client';
import FairyTalesPage from '../../pages/content/FairyTalesPage';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <FairyTalesPage />
    </QueryClientProvider>
  );
}

describe('FairyTalesPage', () => {
  it('renaming a dongeng with no category sends null categories, not empty strings', async () => {
    mock.onGet('/admin/content/fairy-tales').reply(200, {
      data: [
        {
          id: 'd-1',
          title: 'Kancil',
          image_url: 'https://example.com/kancil.png',
          audio_url: '',
          age_start: 3,
          age_end: 6,
          is_free: true,
          // shape returned before the backend fix
          category_id: '',
        },
      ],
      total: 1,
    });
    mock.onGet('/admin/content/categories').reply(200, { data: [], total: 0 });
    mock.onGet('/admin/content/dongeng-categories').reply(200, { data: [], total: 0 });
    mock.onPut('/admin/content/fairy-tales/d-1').reply(200, { data: { id: 'd-1' } });
    const user = userEvent.setup();

    renderPage();

    const row = (await screen.findByText('Kancil')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: /edit/i }));
    const title = await screen.findByDisplayValue('Kancil');
    await user.clear(title);
    await user.type(title, 'Kancil Cerdik');
    await user.click(screen.getByRole('button', { name: 'OK' }));

    await waitFor(() => expect(mock.history.put.length).toBe(1));
    const body = JSON.parse(mock.history.put[0].data);
    expect(body.title).toBe('Kancil Cerdik');
    expect(body.category_id).toBeNull();
    expect(body.dongeng_category_id).toBeNull();
    expect(body.dongeng_sub_category_id).toBeNull();
  });
});
