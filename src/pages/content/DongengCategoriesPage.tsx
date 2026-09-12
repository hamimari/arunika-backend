import { Modal, Form, Input } from 'antd';
import ContentTable from '../../components/ContentTable';
import { dongengCategoriesApi } from '../../api/content';
import { useContentPage } from '../../hooks/useContentPage';
import type { ColumnsType } from 'antd/es/table';

interface Category {
  id: string;
  name: string;
  image_url: string;
  hidden?: boolean;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
}

const tableColumns: ColumnsType<Category> = [
  { title: 'Name', dataIndex: 'name', key: 'name' },
  { title: 'Image URL', dataIndex: 'image_url', key: 'image_url', ellipsis: true },
];

export default function DongengCategoriesPage() {
  const ctx = useContentPage('dongeng-categories', dongengCategoriesApi);
  const [form] = Form.useForm();

  const handleSave = async () => {
    const values = await form.validateFields();
    ctx.onSave(values);
  };

  return (
    <>
      <h2>Dongeng Categories</h2>
      <ContentTable<Category>
        data={ctx.data as Category[]}
        total={ctx.total}
        page={ctx.page}
        perPage={ctx.perPage}
        loading={ctx.loading}
        columns={tableColumns}
        onSearch={ctx.setSearch}
        onPageChange={ctx.onPageChange}
        onAdd={() => { form.resetFields(); ctx.onAdd(); }}
        onEdit={(item) => { form.setFieldsValue(item); ctx.onEdit(item); }}
        onDelete={ctx.onDelete}
        onToggleVisibility={ctx.onToggleVisibility}
      />
      <Modal
        title={ctx.editItem ? 'Edit Category' : 'New Category'}
        open={ctx.modalOpen}
        onOk={handleSave}
        onCancel={() => ctx.setModalOpen(false)}
        confirmLoading={ctx.saving}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="image_url" label="Image URL" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
