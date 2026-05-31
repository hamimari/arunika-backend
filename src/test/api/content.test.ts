import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import MockAdapter from 'axios-mock-adapter';
import api from '../../api/client';
import { fairyTalesApi, bannersApi, arCardsApi } from '../../api/content';

const mock = new MockAdapter(api);

beforeEach(() => mock.reset());
afterEach(() => mock.reset());

describe('contentApi factory (fairyTalesApi)', () => {
  it('list calls GET /admin/content/fairy-tales', async () => {
    mock.onGet('/admin/content/fairy-tales').reply(200, { data: [], total: 0 });
    await fairyTalesApi.list({});
    expect(mock.history.get[0].url).toBe('/admin/content/fairy-tales');
  });

  it('get calls GET /admin/content/fairy-tales/:id', async () => {
    mock.onGet('/admin/content/fairy-tales/123').reply(200, { data: { id: '123' } });
    const res = await fairyTalesApi.get('123');
    expect(res.id).toBe('123');
  });

  it('create calls POST /admin/content/fairy-tales', async () => {
    mock.onPost('/admin/content/fairy-tales').reply(201, { data: { id: 'new' } });
    const res = await fairyTalesApi.create({ title: 'Test' });
    expect(res.id).toBe('new');
  });

  it('update calls PUT /admin/content/fairy-tales/:id', async () => {
    mock.onPut('/admin/content/fairy-tales/123').reply(200, { data: { id: '123', title: 'Updated' } });
    const res = await fairyTalesApi.update('123', { title: 'Updated' });
    expect(res.title).toBe('Updated');
  });

  it('delete calls DELETE /admin/content/fairy-tales/:id', async () => {
    mock.onDelete('/admin/content/fairy-tales/123').reply(204);
    await fairyTalesApi.delete('123');
    expect(mock.history.delete[0].url).toBe('/admin/content/fairy-tales/123');
  });

  it('toggleVisibility sends hidden flag', async () => {
    mock.onPatch('/admin/content/fairy-tales/123/visibility').reply(200, {});
    await fairyTalesApi.toggleVisibility('123', true);
    const body = JSON.parse(mock.history.patch[0].data as string);
    expect(body).toEqual({ hidden: true });
  });
});

describe('bannersApi', () => {
  it('inherits all contentApi methods', async () => {
    mock.onGet('/admin/content/banners').reply(200, { data: [], total: 0 });
    await bannersApi.list({});
    expect(mock.history.get[0].url).toBe('/admin/content/banners');
  });

  it('toggleActive sends is_active flag', async () => {
    mock.onPatch('/admin/content/banners/b1/active').reply(200, {});
    await bannersApi.toggleActive('b1', false);
    const body = JSON.parse(mock.history.patch[0].data as string);
    expect(body).toEqual({ is_active: false });
    expect(mock.history.patch[0].url).toBe('/admin/content/banners/b1/active');
  });

  it('toggleVisibility uses /visibility endpoint not /active', async () => {
    mock.onPatch('/admin/content/banners/b1/visibility').reply(200, {});
    await bannersApi.toggleVisibility('b1', true);
    expect(mock.history.patch[0].url).toBe('/admin/content/banners/b1/visibility');
  });
});

describe('arCardsApi', () => {
  it('uses ar-cards resource slug', async () => {
    mock.onGet('/admin/content/ar-cards').reply(200, { data: [], total: 0 });
    await arCardsApi.list({});
    expect(mock.history.get[0].url).toBe('/admin/content/ar-cards');
  });
});
