import { Modal, Form, Input, Image } from 'antd';
import ContentTable from '../../components/ContentTable';
import { arCardCategoriesApi } from '../../api/content';
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
  {
    title: 'Image',
    dataIndex: 'image_url',
    key: 'image',
    width: 72,
    render: (url: string) =>
      url ? <Image src={url} alt="" width={40} height={40} style={{ objectFit: 'cover', borderRadius: 6 }} /> : '—',
  },
  { title: 'Name', dataIndex: 'name', key: 'name' },
  { title: 'Image URL', dataIndex: 'image_url', key: 'image_url', ellipsis: true },
];

export default function ArCardCategoriesPage() {
  const ctx = useContentPage('ar-card-categories', arCardCategoriesApi);
  const [form] = Form.useForm();

  const handleSave = async () => {
    const values = await form.validateFields();
    ctx.onSave(values);
  };

  return (
    <>
      <h2>AR Card Categories</h2>
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
            <Input placeholder="https://…" />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, curr) => prev.image_url !== curr.image_url}>
            {({ getFieldValue }) =>
              getFieldValue('image_url') ? (
                <Image
                  src={getFieldValue('image_url')}
                  alt="Category image preview"
                  width={96}
                  height={96}
                  style={{ objectFit: 'cover', borderRadius: 8 }}
                />
              ) : null
            }
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
