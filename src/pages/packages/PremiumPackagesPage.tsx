import {
  Table,
  Button,
  Switch,
  Popconfirm,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  Checkbox,
  Tag,
  Space,
  Alert,
  Typography,
} from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { premiumPackagesApi } from '../../api/admin';
import type { PremiumPackage, PremiumPackageInput } from '../../api/admin';

const { Text } = Typography;

const QUERY_KEY = ['premium-packages'];

function formatPrice(price: number) {
  return `Rp ${price.toLocaleString('id-ID')}`;
}

export default function PremiumPackagesPage() {
  const queryClient = useQueryClient();
  const [modalOpen, setModalOpen] = useState(false);
  const [editingPack, setEditingPack] = useState<PremiumPackage | null>(null);
  const [form] = Form.useForm<PremiumPackageInput>();
  const [initialPrice, setInitialPrice] = useState<number | null>(null);
  const [currentPrice, setCurrentPrice] = useState<number | null>(null);

  const { data, isLoading, isError } = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => premiumPackagesApi.list().then((r) => r.data),
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: QUERY_KEY });

  const createMutation = useMutation({
    mutationFn: (d: PremiumPackageInput) => premiumPackagesApi.create(d),
    onSuccess: () => { invalidate(); closeModal(); },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: PremiumPackageInput }) =>
      premiumPackagesApi.update(id, data),
    onSuccess: () => { invalidate(); closeModal(); },
  });

  const deleteMutation = useMutation({
    mutationFn: premiumPackagesApi.remove,
    onSuccess: invalidate,
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, isActive }: { id: string; isActive: boolean }) =>
      premiumPackagesApi.toggleVisibility(id, isActive),
    onSuccess: invalidate,
  });

  const openCreate = () => {
    setEditingPack(null);
    form.resetFields();
    setInitialPrice(null);
    setCurrentPrice(null);
    setModalOpen(true);
  };

  const openEdit = (pack: PremiumPackage) => {
    setEditingPack(pack);
    form.setFieldsValue(pack);
    setInitialPrice(pack.price_idr);
    setCurrentPrice(pack.price_idr);
    setModalOpen(true);
  };

  const closeModal = () => {
    setModalOpen(false);
    setEditingPack(null);
    form.resetFields();
    setInitialPrice(null);
    setCurrentPrice(null);
  };

  const handleSubmit = async () => {
    const values = await form.validateFields();
    if (editingPack) {
      updateMutation.mutate({ id: editingPack.id, data: values });
    } else {
      createMutation.mutate(values);
    }
  };

  const priceChanged = editingPack !== null &&
    initialPrice !== null &&
    currentPrice !== null &&
    currentPrice !== initialPrice;

  const columns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (v: string) => <Text strong>{v}</Text>,
    },
    { title: 'Subtitle', dataIndex: 'subtitle', key: 'subtitle' },
    {
      title: 'Price',
      dataIndex: 'price_idr',
      key: 'price_idr',
      render: (v: number) => formatPrice(v),
    },
    {
      title: 'Type',
      dataIndex: 'type',
      key: 'type',
      render: (v: string) => (
        <Tag color={v === 'subscription' ? 'purple' : 'orange'}>
          {v === 'subscription' ? 'Subscription' : 'Content'}
        </Tag>
      ),
    },
    { title: 'Badge', dataIndex: 'badge_label', key: 'badge_label' },
    {
      title: 'Best Value',
      dataIndex: 'is_best_value',
      key: 'is_best_value',
      render: (v: boolean) => v ? <Tag color="gold">Yes</Tag> : '—',
    },
    {
      title: 'Active',
      dataIndex: 'is_active',
      key: 'is_active',
      render: (v: boolean, record: PremiumPackage) => (
        <Switch
          checked={v}
          loading={toggleMutation.isPending}
          onChange={(checked) => toggleMutation.mutate({ id: record.id, isActive: checked })}
        />
      ),
    },
    { title: 'Sort Order', dataIndex: 'sort_order', key: 'sort_order' },
    {
      title: 'Actions',
      key: 'actions',
      render: (_: unknown, record: PremiumPackage) => (
        <Space>
          <Button
            icon={<EditOutlined />}
            size="small"
            onClick={() => openEdit(record)}
          >
            Edit
          </Button>
          <Popconfirm
            title="Delete package?"
            description="This action cannot be undone."
            onConfirm={() => deleteMutation.mutate(record.id)}
            okText="Delete"
            okButtonProps={{ danger: true }}
          >
            <Button icon={<DeleteOutlined />} size="small" danger>
              Delete
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const isMutating = createMutation.isPending || updateMutation.isPending;

  return (
    <>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <h2 style={{ margin: 0 }}>Premium Packages</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          Add Package
        </Button>
      </div>

      {isError && (
        <Alert type="error" message="Failed to load packages" style={{ marginBottom: 16 }} />
      )}

      <Table
        rowKey="id"
        dataSource={data}
        columns={columns}
        loading={isLoading}
        rowClassName={(record: PremiumPackage) => record.is_active ? '' : 'row-inactive'}
        pagination={{ pageSize: 20 }}
      />

      <style>{`.row-inactive td { color: #bbb !important; }`}</style>

      <Modal
        title={editingPack ? 'Edit Package' : 'Add Package'}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={closeModal}
        okText={editingPack ? 'Save' : 'Create'}
        confirmLoading={isMutating}
        width={560}
      >
        {priceChanged && (
          <Alert
            type="warning"
            message="Price changed — existing subscribers will not be affected, but new purchases will use the new price."
            style={{ marginBottom: 16 }}
          />
        )}
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true, message: 'Name is required' }]}>
            <Input placeholder="e.g. Paket Hutan" />
          </Form.Item>
          <Form.Item name="subtitle" label="Subtitle" rules={[{ required: true, message: 'Subtitle is required' }]}>
            <Input placeholder="e.g. Belajar tentang hutan" />
          </Form.Item>
          <Form.Item
            name="price_idr"
            label="Price (IDR)"
            rules={[
              { required: true, message: 'Price is required' },
              { type: 'number', min: 1, message: 'Price must be a positive integer' },
            ]}
          >
            <InputNumber
              style={{ width: '100%' }}
              min={1}
              precision={0}
              formatter={(v) => `${v}`.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}
              parser={(v) => parseInt((v ?? '').replace(/,/g, ''), 10) as 1}
              onChange={(v) => setCurrentPrice(v ?? null)}
            />
          </Form.Item>
          <Form.Item name="type" label="Type" rules={[{ required: true, message: 'Type is required' }]}>
            <Select
              options={[
                { value: 'content', label: 'Content' },
                { value: 'subscription', label: 'Subscription' },
              ]}
            />
          </Form.Item>
          <Form.Item name="badge_label" label="Badge Label">
            <Input placeholder="e.g. POPULAR" />
          </Form.Item>
          <Form.Item name="is_best_value" label="Best Value" valuePropName="checked">
            <Checkbox />
          </Form.Item>
          <Form.Item name="sort_order" label="Sort Order">
            <InputNumber style={{ width: '100%' }} min={0} precision={0} />
          </Form.Item>
          <Form.Item name="is_active" label="Active" valuePropName="checked">
            <Checkbox />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
