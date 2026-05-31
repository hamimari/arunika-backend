import { Modal, Form, Input, InputNumber, Select } from 'antd';
import ContentTable from '../../components/ContentTable';
import { tracingApi } from '../../api/content';
import { useContentPage } from '../../hooks/useContentPage';
import type { ColumnsType } from 'antd/es/table';

interface TracingItem {
  id: string;
  label: string;
  type: string;
  difficulty: number;
  hidden?: boolean;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
}

const tableColumns: ColumnsType<TracingItem> = [
  { title: 'Label', dataIndex: 'label', key: 'label' },
  { title: 'Type', dataIndex: 'type', key: 'type' },
  { title: 'Difficulty', dataIndex: 'difficulty', key: 'difficulty' },
];

export default function TracingPage() {
  const ctx = useContentPage('tracing-items', tracingApi);
  const [form] = Form.useForm();

  const handleSave = async () => {
    const values = await form.validateFields();
    ctx.onSave(values);
  };

  return (
    <>
      <h2>Tracing Items</h2>
      <ContentTable<TracingItem>
        data={ctx.data as TracingItem[]}
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
        title={ctx.editItem ? 'Edit Tracing Item' : 'New Tracing Item'}
        open={ctx.modalOpen}
        onOk={handleSave}
        onCancel={() => ctx.setModalOpen(false)}
        confirmLoading={ctx.saving}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="label" label="Label" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="type" label="Type" rules={[{ required: true }]}>
            <Select options={[
              { value: 'alphabet', label: 'Alphabet' },
              { value: 'number', label: 'Number' },
              { value: 'shape', label: 'Shape' },
            ]} />
          </Form.Item>
          <Form.Item name="difficulty" label="Difficulty">
            <InputNumber min={1} max={3} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="guide_path_json" label="Guide Path JSON" rules={[{ required: true }]}>
            <Input.TextArea rows={4} placeholder='[{"x":0,"y":0},...]' />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
