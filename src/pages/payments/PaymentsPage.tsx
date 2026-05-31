import { Table, Card, Row, Col, Statistic, Select, Input, Space, Tag, Drawer, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { paymentsApi } from '../../api/admin';
import { analyticsApi } from '../../api/analytics';
import type { ColumnsType } from 'antd/es/table';

const { Search } = Input;
const { Text } = Typography;

interface PaymentTransaction {
  id: string;
  order_id: string;
  user_id: string | null;
  transaction_id: string;
  transaction_status: string;
  payment_type: string;
  gross_amount: string;
  status_code: string;
  fraud_status: string;
  raw_payload: string;
  created_at: string;
  updated_at: string;
}

const STATUS_COLORS: Record<string, string> = {
  settlement: 'green',
  capture: 'green',
  pending: 'orange',
  deny: 'red',
  expire: 'red',
  cancel: 'red',
  refund: 'purple',
};

export default function PaymentsPage() {
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const perPage = 20;

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedItem, setSelectedItem] = useState<PaymentTransaction | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ['payments', statusFilter, search, page],
    queryFn: () =>
      paymentsApi.list({
        status: statusFilter,
        search: search || undefined,
        page,
        per_page: perPage,
      }),
  });

  const { data: subStats, isLoading: subLoading } = useQuery({
    queryKey: ['subscriptionStats'],
    queryFn: () => analyticsApi.getSubscriptionStats(),
  });

  const transactions: PaymentTransaction[] = data?.data ?? [];
  const total: number = data?.total ?? 0;
  const totalUsers: number = subStats?.total ?? 0;
  const premiumUsers: number = subStats?.premium ?? 0;
  const freeUsers: number = subStats?.free ?? 0;
  const conversionRate = totalUsers > 0 ? Math.round((premiumUsers / totalUsers) * 100) : 0;

  const columns: ColumnsType<PaymentTransaction> = [
    {
      title: 'Order ID',
      dataIndex: 'order_id',
      key: 'order_id',
      ellipsis: true,
      width: 220,
    },
    {
      title: 'Transaction ID',
      dataIndex: 'transaction_id',
      key: 'transaction_id',
      ellipsis: true,
      width: 200,
      render: (v: string) => v || '-',
    },
    {
      title: 'Status',
      dataIndex: 'transaction_status',
      key: 'transaction_status',
      width: 120,
      render: (v: string) => (
        <Tag color={STATUS_COLORS[v] ?? 'default'}>{v || '-'}</Tag>
      ),
    },
    {
      title: 'Payment Type',
      dataIndex: 'payment_type',
      key: 'payment_type',
      width: 140,
      render: (v: string) => v || '-',
    },
    {
      title: 'Amount',
      dataIndex: 'gross_amount',
      key: 'gross_amount',
      width: 130,
      render: (v: string) =>
        v ? `IDR ${Number(v).toLocaleString('id-ID')}` : '-',
    },
    {
      title: 'Date',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (v: string) => new Date(v).toLocaleString('id-ID'),
    },
  ];

  const formatRawPayload = (raw: string) => {
    try {
      return JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
      return raw;
    }
  };

  return (
    <>
      <h2>Payments</h2>

      {/* Summary */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={12} lg={6}>
          <Card>
            <Statistic title="Total Transactions" value={total} loading={isLoading} />
          </Card>
        </Col>
        <Col xs={12} lg={6}>
          <Card>
            <Statistic
              title="Premium Users"
              value={premiumUsers}
              loading={subLoading}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
        <Col xs={12} lg={6}>
          <Card>
            <Statistic
              title="Free Users"
              value={freeUsers}
              loading={subLoading}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col xs={12} lg={6}>
          <Card>
            <Statistic
              title="Conversion Rate"
              value={conversionRate}
              suffix="%"
              loading={subLoading}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
      </Row>

      {/* Filters */}
      <Space style={{ marginBottom: 16 }} wrap>
        <Search
          placeholder="Search order ID or transaction ID"
          allowClear
          style={{ width: 300 }}
          onSearch={(v) => { setSearch(v); setPage(1); }}
          onChange={(e) => { if (!e.target.value) { setSearch(''); setPage(1); } }}
        />
        <Select
          placeholder="Filter by status"
          allowClear
          style={{ width: 180 }}
          onChange={(v) => { setStatusFilter(v); setPage(1); }}
          options={[
            { value: 'settlement', label: 'Settlement' },
            { value: 'capture', label: 'Capture' },
            { value: 'pending', label: 'Pending' },
            { value: 'deny', label: 'Deny' },
            { value: 'expire', label: 'Expire' },
            { value: 'cancel', label: 'Cancel' },
          ]}
        />
      </Space>

      <Table<PaymentTransaction>
        dataSource={transactions}
        rowKey="id"
        loading={isLoading}
        columns={columns}
        scroll={{ x: 1000 }}
        pagination={{
          current: page,
          pageSize: perPage,
          total,
          showSizeChanger: false,
          onChange: (p) => setPage(p),
        }}
        onRow={(record) => ({
          onClick: () => { setSelectedItem(record); setDrawerOpen(true); },
          style: { cursor: 'pointer' },
        })}
      />

      <Drawer
        title="Transaction Detail"
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={520}
      >
        {selectedItem && (
          <Space direction="vertical" style={{ width: '100%' }} size="small">
            <div><Text strong>Order ID:</Text> <Text copyable>{selectedItem.order_id}</Text></div>
            <div><Text strong>Transaction ID:</Text> <Text copyable>{selectedItem.transaction_id || '-'}</Text></div>
            <div>
              <Text strong>Status: </Text>
              <Tag color={STATUS_COLORS[selectedItem.transaction_status] ?? 'default'}>
                {selectedItem.transaction_status}
              </Tag>
            </div>
            <div><Text strong>Payment Type:</Text> {selectedItem.payment_type || '-'}</div>
            <div>
              <Text strong>Amount:</Text>{' '}
              {selectedItem.gross_amount
                ? `IDR ${Number(selectedItem.gross_amount).toLocaleString('id-ID')}`
                : '-'}
            </div>
            <div><Text strong>Status Code:</Text> {selectedItem.status_code || '-'}</div>
            <div><Text strong>Fraud Status:</Text> {selectedItem.fraud_status || '-'}</div>
            <div><Text strong>User ID:</Text> <Text copyable={{ text: selectedItem.user_id ?? '' }}>{selectedItem.user_id || '-'}</Text></div>
            <div><Text strong>Created At:</Text> {new Date(selectedItem.created_at).toLocaleString('id-ID')}</div>

            <div style={{ marginTop: 12 }}>
              <Text strong>Raw Payload:</Text>
              <pre
                style={{
                  background: '#f5f5f5',
                  padding: 12,
                  borderRadius: 4,
                  overflowX: 'auto',
                  fontSize: 12,
                  marginTop: 8,
                }}
              >
                {formatRawPayload(selectedItem.raw_payload)}
              </pre>
            </div>
          </Space>
        )}
      </Drawer>
    </>
  );
}
