import { Modal, Form, Input, Switch, InputNumber, Tag, message } from 'antd';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import ContentTable from '../../components/ContentTable';
import { bannersApi } from '../../api/content';
import { useContentPage } from '../../hooks/useContentPage';
import type { ColumnsType } from 'antd/es/table';

interface Banner {
  id: string;
  title: string;
  image_url: string;
  link_url?: string;
  description?: string;
  is_active: boolean;
  hidden?: boolean;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
}

export default function BannersPage() {
  const ctx = useContentPage('banners', bannersApi);
  const [form] = Form.useForm();
  const qc = useQueryClient();

  const toggleActive = useMutation({
    mutationFn: ({ id, is_active }: { id: string; is_active: boolean }) =>
      bannersApi.toggleActive(id, is_active),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['content', 'banners'] });
      message.success('Banner active status updated');
    },
  });

  const tableColumns: ColumnsType<Banner> = [
    { title: 'Title', dataIndex: 'title', key: 'title' },
    {
      title: 'Image',
      dataIndex: 'image_url',
      key: 'image_url',
      render: (v: string) =>
        v ? (
          <img src={v} alt="banner" style={{ height: 40, objectFit: 'cover', borderRadius: 4 }} />
        ) : (
          '-'
        ),
    },
    { title: 'Link URL', dataIndex: 'link_url', key: 'link_url', render: (v) => v || '-' },
    {
      title: 'Active',
      dataIndex: 'is_active',
      key: 'is_active',
      render: (v: boolean, record: Banner) => (
        <Tag
          color={v ? 'green' : 'default'}
          style={{ cursor: 'pointer' }}
          onClick={() => toggleActive.mutate({ id: record.id, is_active: !v })}
        >
          {v ? 'Active' : 'Inactive'}
        </Tag>
      ),
    },
  ];

  const handleSave = async () => {
    const values = await form.validateFields();
    ctx.onSave(values);
  };

  return (
    <>
      <h2>Banners</h2>
      <ContentTable<Banner>
        data={ctx.data as Banner[]}
        total={ctx.total}
        page={ctx.page}
        perPage={ctx.perPage}
        loading={ctx.loading}
        columns={tableColumns}
        onSearch={ctx.setSearch}
        onPageChange={ctx.onPageChange}
        onAdd={() => {
          form.resetFields();
          ctx.onAdd();
        }}
        onEdit={(item) => {
          form.setFieldsValue(item);
          ctx.onEdit(item);
        }}
        onDelete={ctx.onDelete}
        onToggleVisibility={ctx.onToggleVisibility}
      />
      <Modal
        title={ctx.editItem ? 'Edit Banner' : 'New Banner'}
        open={ctx.modalOpen}
        onOk={handleSave}
        onCancel={() => ctx.setModalOpen(false)}
        confirmLoading={ctx.saving}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="title" label="Title" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="image_url" label="Image URL" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="link_url" label="Link URL">
            <Input />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input />
          </Form.Item>
          <Form.Item name="sort_order" label="Sort Order">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="is_active" label="Active" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
