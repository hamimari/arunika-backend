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
  List,
} from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, AppstoreOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { premiumPackagesApi, productsApi, packageItemsApi } from '../../api/admin';
import type { PremiumPackage, PremiumPackageInput, Product } from '../../api/admin';

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
  const [itemsPackage, setItemsPackage] = useState<PremiumPackage | null>(null);
  const watchedType = Form.useWatch('type', form);

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
    // Content packages don't use duration_days — clear it even if a
    // leftover value exists from switching the Type select back and forth.
    if (values.type !== 'subscription') {
      values.duration_days = null;
    }
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
    {
      title: 'Duration',
      dataIndex: 'duration_days',
      key: 'duration_days',
      render: (v: number | null) => (v ? `${v} days` : '—'),
    },
    {
      title: 'Play Billing',
      dataIndex: 'play_product_id',
      key: 'play_product_id',
      render: (v: string | null) =>
        v ? <Tag color="green">Mapped</Tag> : <Tag color="default">Unmapped</Tag>,
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
          <Button
            icon={<AppstoreOutlined />}
            size="small"
            onClick={() => setItemsPackage(record)}
          >
            Manage Items
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
          <Form.Item name="description" label="Description">
            <Input.TextArea
              rows={3}
              placeholder="Shown on the landing page's product cards"
            />
          </Form.Item>
          <Form.Item name="image_url" label="Image URL">
            <Input placeholder="https://... (sample image for the landing page)" />
          </Form.Item>
          <Form.Item
            name="play_product_id"
            label="Play Product ID"
            extra="Google Play Console in-app product/subscription SKU. Leave blank to keep this package unavailable via Google Play Billing."
          >
            <Input placeholder="e.g. pack_dongeng_bundle_1" />
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
          {watchedType === 'subscription' && (
            <Form.Item
              name="duration_days"
              label="Duration (Days)"
              rules={[
                { required: true, message: 'Duration is required for subscription packages' },
                { type: 'number', min: 1, message: 'Duration must be a positive integer' },
              ]}
            >
              <InputNumber style={{ width: '100%' }} min={1} precision={0} placeholder="e.g. 30" />
            </Form.Item>
          )}
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

      {itemsPackage && (
        <ManageItemsModal
          pkg={itemsPackage}
          onClose={() => setItemsPackage(null)}
        />
      )}
    </>
  );
}

// ── Manage Items ──────────────────────────────────────────────────────────────

function ManageItemsModal({ pkg, onClose }: { pkg: PremiumPackage; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [selectedProductId, setSelectedProductId] = useState<string | null>(null);

  const itemsKey = ['premium-package-items', pkg.id];

  const { data: items, isLoading: itemsLoading } = useQuery({
    queryKey: itemsKey,
    queryFn: () => packageItemsApi.list(pkg.id).then((r) => r.data),
  });

  const { data: products, isLoading: productsLoading } = useQuery({
    queryKey: ['products'],
    queryFn: () => productsApi.list().then((r) => r.data),
  });

  const invalidateItems = () => queryClient.invalidateQueries({ queryKey: itemsKey });

  const addMutation = useMutation({
    mutationFn: (productId: string) => packageItemsApi.add(pkg.id, productId),
    onSuccess: () => { invalidateItems(); setSelectedProductId(null); },
  });

  const removeMutation = useMutation({
    mutationFn: (productId: string) => packageItemsApi.remove(pkg.id, productId),
    onSuccess: invalidateItems,
  });

  const productById = new Map((products ?? []).map((p: Product) => [p.id, p]));
  const bundledIds = new Set((items ?? []).map((i) => i.product_id));
  const availableProducts = (products ?? []).filter((p: Product) => !bundledIds.has(p.id));

  return (
    <Modal
      title={`Manage Items — ${pkg.name}`}
      open
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={520}
    >
      <Space style={{ width: '100%', marginBottom: 16 }}>
        <Select
          style={{ width: 320 }}
          placeholder="Select a product to add"
          loading={productsLoading}
          value={selectedProductId}
          onChange={setSelectedProductId}
          options={availableProducts.map((p: Product) => ({
            value: p.id,
            label: `${p.display_name || p.id.slice(0, 8) + '…'} (${p.feature_code === 'AR_CARD' ? 'AR Card' : 'Dongeng'}) — Rp ${p.price_idr.toLocaleString('id-ID')}`,
          }))}
        />
        <Button
          type="primary"
          disabled={!selectedProductId}
          loading={addMutation.isPending}
          onClick={() => selectedProductId && addMutation.mutate(selectedProductId)}
        >
          Add
        </Button>
      </Space>

      <List
        loading={itemsLoading}
        bordered
        dataSource={items ?? []}
        locale={{ emptyText: 'No products bundled in this package yet.' }}
        renderItem={(item) => {
          const product = productById.get(item.product_id);
          return (
            <List.Item
              actions={[
                <Popconfirm
                  key="remove"
                  title="Remove this product from the package?"
                  onConfirm={() => removeMutation.mutate(item.product_id)}
                  okText="Remove"
                  okButtonProps={{ danger: true }}
                >
                  <Button size="small" danger icon={<DeleteOutlined />} />
                </Popconfirm>,
              ]}
            >
              <Text>
                {product
                  ? `${product.display_name || item.product_id} (${product.feature_code === 'AR_CARD' ? 'AR Card' : 'Dongeng'}) — Rp ${product.price_idr.toLocaleString('id-ID')}`
                  : item.product_id}
              </Text>
            </List.Item>
          );
        }}
      />
    </Modal>
  );
}
