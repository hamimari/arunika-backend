import { Table, Tag, Select, Input, Space, Typography, Button, message, Modal, Descriptions, Spin } from 'antd';
import { SyncOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { ordersApi, usersApi, productsApi, premiumPackagesApi } from '../../api/admin';
import type { Order } from '../../api/admin';
import type { ColumnsType } from 'antd/es/table';

const { Text, Link } = Typography;
const { Search } = Input;

const STATUS_COLORS: Record<Order['status'], string> = {
  PENDING: 'orange',
  PAID: 'green',
  FAILED: 'red',
  EXPIRED: 'default',
};

export default function OrdersPage() {
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [userModalId, setUserModalId] = useState<string | null>(null);
  const [itemModalOrder, setItemModalOrder] = useState<Order | null>(null);
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
      message.success('Order status synced with Midtrans');
      queryClient.invalidateQueries({ queryKey: ['orders'] });
    },
    onError: () => message.error('Failed to sync with Midtrans'),
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
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (v: string) => new Date(v).toLocaleString('id-ID'),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_: unknown, record: Order) => (
        <Button
          size="small"
          icon={<SyncOutlined />}
          loading={syncMutation.isPending && syncMutation.variables === record.id}
          disabled={record.status !== 'PENDING'}
          onClick={() => syncMutation.mutate(record.id)}
        >
          Sync Midtrans
        </Button>
      ),
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
          ]}
        />
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
    </>
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
