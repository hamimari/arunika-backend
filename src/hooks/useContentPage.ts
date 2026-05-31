import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { message } from 'antd';

interface ContentApiModule {
  list: (params: { search?: string; page?: number; per_page?: number }) => Promise<{
    data: unknown[];
    total: number;
  }>;
  create: (data: unknown) => Promise<unknown>;
  update: (id: string, data: unknown) => Promise<unknown>;
  delete: (id: string) => Promise<unknown>;
  toggleVisibility: (id: string, hidden: boolean) => Promise<unknown>;
}

export function useContentPage(key: string, api: ContentApiModule) {
  const qc = useQueryClient();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(20);
  const [modalOpen, setModalOpen] = useState(false);
  const [editItem, setEditItem] = useState<unknown>(null);

  const query = useQuery({
    queryKey: [key, search, page, perPage],
    queryFn: () => api.list({ search, page, per_page: perPage }),
  });

  const invalidate = () => qc.invalidateQueries({ queryKey: [key] });

  const createMutation = useMutation({
    mutationFn: (data: unknown) => api.create(data),
    onSuccess: () => { invalidate(); message.success('Created'); setModalOpen(false); },
    onError: () => message.error('Failed to create'),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: unknown }) => api.update(id, data),
    onSuccess: () => { invalidate(); message.success('Updated'); setModalOpen(false); },
    onError: () => message.error('Failed to update'),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(id),
    onSuccess: () => { invalidate(); message.success('Deleted'); },
    onError: () => message.error('Failed to delete'),
  });

  const visibilityMutation = useMutation({
    mutationFn: ({ id, hidden }: { id: string; hidden: boolean }) =>
      api.toggleVisibility(id, hidden),
    onSuccess: () => invalidate(),
    onError: () => message.error('Failed to update visibility'),
  });

  return {
    data: query.data?.data as unknown[],
    total: query.data?.total ?? 0,
    loading: query.isLoading,
    search, setSearch,
    page, perPage,
    setPage, setPerPage,
    onPageChange: (p: number, ps: number) => { setPage(p); setPerPage(ps); },
    modalOpen, setModalOpen,
    editItem, setEditItem,
    onAdd: () => { setEditItem(null); setModalOpen(true); },
    onEdit: (item: unknown) => { setEditItem(item); setModalOpen(true); },
    onDelete: (id: string) => deleteMutation.mutate(id),
    onToggleVisibility: (id: string, hidden: boolean) =>
      visibilityMutation.mutate({ id, hidden }),
    onSave: (data: unknown) => {
      const item = editItem as { id?: string } | null;
      if (item?.id) {
        updateMutation.mutate({ id: item.id, data });
      } else {
        createMutation.mutate(data);
      }
    },
    saving: createMutation.isPending || updateMutation.isPending,
  };
}
