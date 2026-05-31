import { Modal, Form, InputNumber, Select } from 'antd';
import ContentTable from '../../components/ContentTable';
import { badgesApi } from '../../api/content';
import { useContentPage } from '../../hooks/useContentPage';
import type { ColumnsType } from 'antd/es/table';

interface Badge {
  id: string;
  feature: string;
  level: string;
  threshold: number;
  hidden?: boolean;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
}

const tableColumns: ColumnsType<Badge> = [
  { title: 'Feature', dataIndex: 'feature', key: 'feature' },
  { title: 'Level', dataIndex: 'level', key: 'level' },
  { title: 'Threshold', dataIndex: 'threshold', key: 'threshold' },
];

export default function BadgesPage() {
  const ctx = useContentPage('badges', badgesApi);
  const [form] = Form.useForm();

  const handleSave = async () => {
    const values = await form.validateFields();
    ctx.onSave(values);
  };

  return (
    <>
      <h2>Badges</h2>
      <ContentTable<Badge>
        data={ctx.data as Badge[]}
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
        title={ctx.editItem ? 'Edit Badge' : 'New Badge'}
        open={ctx.modalOpen}
        onOk={handleSave}
        onCancel={() => ctx.setModalOpen(false)}
        confirmLoading={ctx.saving}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="feature" label="Feature" rules={[{ required: true }]}>
            <Select options={[
              { value: 'tracing', label: 'Tracing' },
              { value: 'counting', label: 'Counting' },
              { value: 'global', label: 'Global' },
            ]} />
          </Form.Item>
          <Form.Item name="level" label="Level" rules={[{ required: true }]}>
            <Select options={[
              { value: 'beginner', label: 'Beginner' },
              { value: 'explorer', label: 'Explorer' },
              { value: 'master', label: 'Master' },
              { value: 'all_rounder', label: 'All Rounder' },
            ]} />
          </Form.Item>
          <Form.Item name="threshold" label="Threshold" rules={[{ required: true }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
