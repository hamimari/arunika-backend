import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import MockAdapter from 'axios-mock-adapter';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import api from '../../api/client';
import DongengCategoriesPage from '../../pages/content/DongengCategoriesPage';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <DongengCategoriesPage />
    </QueryClientProvider>
  );
}

describe('DongengCategoriesPage', () => {
  it('renders the category list', async () => {
    mock.onGet('/admin/content/dongeng-categories').reply(200, {
      data: [
        { id: 'cat-1', name: 'Fairy Tales', image_url: 'https://example.com/fairy.png' },
        { id: 'cat-2', name: 'Islamic', image_url: 'https://example.com/islamic.png' },
      ],
      total: 2,
    });

    renderPage();

    expect(await screen.findByText('Fairy Tales')).toBeInTheDocument();
    expect(screen.getByText('Islamic')).toBeInTheDocument();
  });

  it('shows an empty table when there are no categories', async () => {
    mock.onGet('/admin/content/dongeng-categories').reply(200, { data: [], total: 0 });

    renderPage();

    expect(await screen.findByText('Dongeng Categories')).toBeInTheDocument();
    expect(await screen.findByText('No data', { selector: 'div' })).toBeInTheDocument();
  });
});
