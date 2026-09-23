import { Table, Tag, Select, Input, Space, Typography, Button, message, Modal, Descriptions, Spin } from 'antd';
import { SyncOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { ordersApi, usersApi, productsApi, premiumPackagesApi } from '../../api/admin';
import type { Order } from '../../api/admin';
import type { ColumnsType } from 'antd/es/table';

const { Text, Link } = Typography;
const { Search } = Input;

// Surfaces the backend's own {"error": "..."} message when there is one
// (e.g. "purchase is not valid") instead of a generic string that hides why
// the action actually failed.
function extractErrorMessage(err: unknown, fallback: string): string {
  const detail =
    err && typeof err === 'object' && 'response' in err
      ? (err as { response?: { data?: { error?: string } } }).response?.data?.error
      : undefined;
  return detail ?? fallback;
}

const STATUS_COLORS: Record<Order['status'], string> = {
  PENDING: 'orange',
  PAID: 'green',
  FAILED: 'red',
  EXPIRED: 'default',
  REFUNDED: 'volcano',
};

export default function OrdersPage() {
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [userModalId, setUserModalId] = useState<string | null>(null);
  const [itemModalOrder, setItemModalOrder] = useState<Order | null>(null);
  const [recoverPlayOrder, setRecoverPlayOrder] = useState<Order | null>(null);
  const perPage = 20;
  const queryClient = useQueryClient();

  const queryKey = ['orders', statusFilter, search, page];

  const { data, isLoading } = useQuery({
    queryKey,
    queryFn: () => ordersApi.list({ status: statusFilter, search: search || undefined, page, per_page: perPage }),
  });

  const syncMutation = useMutation({
    mutationFn: (id: string) => ordersApi.sync(id),
    onSuccess: () => {
      message.success('Order status synced');
      queryClient.invalidateQueries({ queryKey: ['orders'] });
    },
    onError: (err: unknown) => message.error(extractErrorMessage(err, 'Failed to sync order')),
  });

  const reconcilePlayMutation = useMutation({
    mutationFn: () => ordersApi.reconcilePlay(),
    onSuccess: ({ data }) => {
      message.success(`Checked Google Play for refunds — ${data.reconciled} order(s) revoked`);
      queryClient.invalidateQueries({ queryKey: ['orders'] });
    },
    onError: (err: unknown) => message.error(extractErrorMessage(err, 'Failed to reconcile with Google Play')),
  });

  const orders = data?.data ?? [];
  const total = data?.total ?? 0;

  const columns: ColumnsType<Order> = [
    {
      title: 'Order ID',
      dataIndex: 'id',
      key: 'id',
      ellipsis: true,
      render: (v: string) => <Text copyable={{ text: v }}>{v.slice(0, 8)}…</Text>,
    },
    {
      title: 'User',
      key: 'user',
      render: (_: unknown, record: Order) => (
        <Link onClick={() => setUserModalId(record.user_id)}>
          <div>{record.user_name || record.user_id.slice(0, 8) + '…'}</div>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {record.user_email}
          </Text>
        </Link>
      ),
    },
    {
      title: 'Item',
      key: 'item',
      render: (_: unknown, record: Order) => (
        <Link onClick={() => setItemModalOrder(record)}>
          {record.package_id ? (
            <span>
              <Tag color="purple">Package</Tag> {record.package_name ?? record.package_id.slice(0, 8) + '…'}
            </span>
          ) : (
            <span>
              <Tag color="blue">Product</Tag> {record.product_name ?? record.product_id?.slice(0, 8) + '…'}
            </span>
          )}
        </Link>
      ),
    },
    {
      title: 'Amount',
      dataIndex: 'amount_idr',
      key: 'amount_idr',
      render: (v: number) => `Rp ${v.toLocaleString('id-ID')}`,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (v: Order['status']) => <Tag color={STATUS_COLORS[v] ?? 'default'}>{v}</Tag>,
    },
    {
      title: 'Provider',
      dataIndex: 'provider',
      key: 'provider',
      render: (v: Order['provider']) => (
        <Tag color={v === 'google_play' ? 'blue' : 'purple'}>{v === 'google_play' ? 'Google Play' : 'Midtrans'}</Tag>
      ),
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (v: string) => new Date(v).toLocaleString('id-ID'),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_: unknown, record: Order) => {
        if (record.status !== 'PENDING') return null;

        // One action per record, driven by which payment rail the order
        // was created against — a Play order with a token already on file
        // syncs in one click just like Midtrans; only a Play order with no
        // token yet needs an admin to supply one.
        if (record.provider === 'google_play' && !record.has_purchase_token) {
          return (
            <Button size="small" onClick={() => setRecoverPlayOrder(record)}>
              Recover Play
            </Button>
          );
        }
        return (
          <Button
            size="small"
            icon={<SyncOutlined />}
            loading={syncMutation.isPending && syncMutation.variables === record.id}
            onClick={() => syncMutation.mutate(record.id)}
          >
            {record.provider === 'google_play' ? 'Sync Google Play' : 'Sync Midtrans'}
          </Button>
        );
      },
    },
  ];

  return (
    <>
      <h2>Orders</h2>
      <Space style={{ marginBottom: 16 }} wrap>
        <Search
          placeholder="Search by name, email, phone, user ID, or order ID"
          allowClear
          style={{ width: 340 }}
          onSearch={(v) => { setSearch(v); setPage(1); }}
          onChange={(e) => { if (!e.target.value) { setSearch(''); setPage(1); } }}
        />
        <Select
          placeholder="Filter by status"
          allowClear
          style={{ width: 180 }}
          onChange={(v) => { setStatusFilter(v); setPage(1); }}
          options={[
            { value: 'PENDING', label: 'Pending' },
            { value: 'PAID', label: 'Paid' },
            { value: 'FAILED', label: 'Failed' },
            { value: 'EXPIRED', label: 'Expired' },
            { value: 'REFUNDED', label: 'Refunded' },
          ]}
        />
        <Button
          icon={<SyncOutlined />}
          loading={reconcilePlayMutation.isPending}
          onClick={() => reconcilePlayMutation.mutate()}
        >
          Check Google Play Refunds
        </Button>
      </Space>
      <Table<Order>
        rowKey="id"
        dataSource={orders}
        columns={columns}
        loading={isLoading}
        scroll={{ x: 1100 }}
        pagination={{
          current: page,
          pageSize: perPage,
          total,
          showSizeChanger: false,
          onChange: (p) => setPage(p),
        }}
      />

      {userModalId && (
        <UserDetailModal userId={userModalId} onClose={() => setUserModalId(null)} />
      )}
      {itemModalOrder && (
        <ItemDetailModal order={itemModalOrder} onClose={() => setItemModalOrder(null)} />
      )}
      {recoverPlayOrder && (
        <RecoverPlayModal order={recoverPlayOrder} onClose={() => setRecoverPlayOrder(null)} />
      )}
    </>
  );
}

// Manually settles a Google Play purchase stuck PENDING because the app
// never called verify — an admin supplies the purchase token obtained
// out-of-band (e.g. a support case), and this reuses the same verify path
// a normal purchase completes through.
function RecoverPlayModal({ order, onClose }: { order: Order; onClose: () => void }) {
  const [purchaseToken, setPurchaseToken] = useState('');
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: () => ordersApi.recoverPlay(order.id, purchaseToken.trim()),
    onSuccess: () => {
      message.success('Purchase verified and entitlement granted');
      queryClient.invalidateQueries({ queryKey: ['orders'] });
      onClose();
    },
    onError: (err: unknown) => message.error(extractErrorMessage(err, 'Failed to verify purchase')),
  });

  return (
    <Modal
      title="Recover Google Play Purchase"
      open
      onCancel={onClose}
      onOk={() => mutation.mutate()}
      okButtonProps={{ disabled: !purchaseToken.trim(), loading: mutation.isPending }}
      okText="Verify & Grant"
    >
      <Text type="secondary">
        Order {order.id.slice(0, 8)}… for {order.package_name ?? order.package_id}. Paste the Google Play purchase
        token for this order (from Play Console's order management, a support case, or app logs) — this re-runs the
        same verification a normal purchase completes through.
      </Text>
      <Input.TextArea
        style={{ marginTop: 12 }}
        rows={3}
        placeholder="Purchase token"
        value={purchaseToken}
        onChange={(e) => setPurchaseToken(e.target.value)}
      />
    </Modal>
  );
}

function UserDetailModal({ userId, onClose }: { userId: string; onClose: () => void }) {
  const { data, isLoading } = useQuery({
    queryKey: ['user', userId],
    queryFn: () => usersApi.get(userId),
  });

  const user = data?.user;
  const sub = data?.subscription;

  return (
    <Modal title="User Detail" open onCancel={onClose} footer={<Button onClick={onClose}>Close</Button>} width={520}>
      {isLoading ? (
        <Spin />
      ) : (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="Name">{user?.name || '—'}</Descriptions.Item>
          <Descriptions.Item label="Email">{user?.email_address || '—'}</Descriptions.Item>
          <Descriptions.Item label="Phone">{user?.phone_number || '—'}</Descriptions.Item>
          <Descriptions.Item label="Address">{user?.address || '—'}</Descriptions.Item>
          <Descriptions.Item label="City">{user?.city || '—'}</Descriptions.Item>
          <Descriptions.Item label="Joined">
            {user?.created_at ? new Date(user.created_at).toLocaleString('id-ID') : '—'}
          </Descriptions.Item>
          <Descriptions.Item label="Subscription">
            <Tag color={sub?.status === 'premium' ? 'gold' : 'default'}>{sub?.status ?? 'free'}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="Subscription Expires">
            {sub?.expires_at ? new Date(sub.expires_at).toLocaleString('id-ID') : '—'}
          </Descriptions.Item>
        </Descriptions>
      )}
    </Modal>
  );
}

function ItemDetailModal({ order, onClose }: { order: Order; onClose: () => void }) {
  const isPackage = !!order.package_id;

  const { data: products, isLoading: productsLoading } = useQuery({
    queryKey: ['products'],
    queryFn: () => productsApi.list().then((r) => r.data),
    enabled: !isPackage,
  });

  const { data: packages, isLoading: packagesLoading } = useQuery({
    queryKey: ['premium-packages'],
    queryFn: () => premiumPackagesApi.list().then((r) => r.data),
    enabled: isPackage,
  });

  const product = products?.find((p) => p.id === order.product_id);
  const pkg = packages?.find((p) => p.id === order.package_id);
  const isLoading = isPackage ? packagesLoading : productsLoading;

  return (
    <Modal title="Item Detail" open onCancel={onClose} footer={<Button onClick={onClose}>Close</Button>} width={520}>
      {isLoading ? (
        <Spin />
      ) : isPackage ? (
        pkg ? (
          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label="Name">{pkg.name}</Descriptions.Item>
            <Descriptions.Item label="Subtitle">{pkg.subtitle}</Descriptions.Item>
            <Descriptions.Item label="Type">
              <Tag color={pkg.type === 'subscription' ? 'purple' : 'orange'}>
                {pkg.type === 'subscription' ? 'Subscription' : 'Content'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="Price">{`Rp ${pkg.price_idr.toLocaleString('id-ID')}`}</Descriptions.Item>
            {pkg.duration_days && (
              <Descriptions.Item label="Duration">{`${pkg.duration_days} days`}</Descriptions.Item>
            )}
            <Descriptions.Item label="Active">
              {pkg.is_active ? <Tag color="green">Active</Tag> : <Tag>Inactive</Tag>}
            </Descriptions.Item>
          </Descriptions>
        ) : (
          <Text type="secondary">Package details unavailable (it may have been deleted).</Text>
        )
      ) : product ? (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="Name">{product.display_name || '—'}</Descriptions.Item>
          <Descriptions.Item label="Feature">
            <Tag color={product.feature_code === 'AR_CARD' ? 'blue' : 'purple'}>
              {product.feature_code === 'AR_CARD' ? 'AR Card' : product.feature_code === 'DONGENG' ? 'Dongeng' : '—'}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="Price">{`Rp ${product.price_idr.toLocaleString('id-ID')}`}</Descriptions.Item>
          <Descriptions.Item label="Active">
            {product.is_active ? <Tag color="green">Active</Tag> : <Tag>Inactive</Tag>}
          </Descriptions.Item>
        </Descriptions>
      ) : (
        <Text type="secondary">Product details unavailable (it may have been deleted).</Text>
      )}
    </Modal>
  );
}
