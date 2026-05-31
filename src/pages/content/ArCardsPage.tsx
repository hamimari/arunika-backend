import { Modal, Form, Input, Select } from 'antd';
import ContentTable from '../../components/ContentTable';
import { arCardsApi } from '../../api/content';
import { useContentPage } from '../../hooks/useContentPage';
import type { ColumnsType } from 'antd/es/table';

interface ArCard {
  id: string;
  title: string;
  type: string;
  file_url: string;
  short_code: string;
  hidden?: boolean;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
}

const tableColumns: ColumnsType<ArCard> = [
  { title: 'Title', dataIndex: 'title', key: 'title' },
  { title: 'Type', dataIndex: 'type', key: 'type' },
  { title: 'Short Code', dataIndex: 'short_code', key: 'short_code' },
];

export default function ArCardsPage() {
  const ctx = useContentPage('ar-cards', arCardsApi);
  const [form] = Form.useForm();

  const handleSave = async () => {
    const values = await form.validateFields();
    ctx.onSave(values);
  };

  return (
    <>
      <h2>AR Cards</h2>
      <ContentTable<ArCard>
        data={ctx.data as ArCard[]}
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
        title={ctx.editItem ? 'Edit AR Card' : 'New AR Card'}
        open={ctx.modalOpen}
        onOk={handleSave}
        onCancel={() => ctx.setModalOpen(false)}
        confirmLoading={ctx.saving}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="title" label="Title" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="type" label="Type" rules={[{ required: true }]}>
            <Select options={[
              { value: 'alphabet', label: 'Alphabet' },
              { value: 'number', label: 'Number' },
              { value: 'animal', label: 'Animal' },
            ]} />
          </Form.Item>
          <Form.Item name="file_url" label="File URL" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="sound_url" label="Sound URL">
            <Input />
          </Form.Item>
          <Form.Item name="short_code" label="Short Code">
            <Input />
          </Form.Item>
          <Form.Item name="image_url" label="Image URL">
            <Input />
          </Form.Item>
          <Form.Item name="printable_img" label="Printable Image URL">
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
