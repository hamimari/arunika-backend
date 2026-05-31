import { Card, Descriptions, Table, Tag, Button, Spin, Modal, InputNumber, Form, message } from 'antd';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { useState } from 'react';
import { usersApi } from '../../api/admin';

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [grantModalOpen, setGrantModalOpen] = useState(false);
  const [form] = Form.useForm();

  const { data, isLoading } = useQuery({
    queryKey: ['user', id],
    queryFn: () => usersApi.get(id!),
  });

  const permissionMutation = useMutation({
    mutationFn: ({ action, duration_days }: { action: 'grant' | 'revoke'; duration_days?: number }) =>
      usersApi.updatePermission(id!, action, duration_days),
    onSuccess: (_data, variables) => {
      qc.invalidateQueries({ queryKey: ['user', id] });
      message.success(variables.action === 'grant' ? 'Premium granted' : 'Premium revoked');
      setGrantModalOpen(false);
      form.resetFields();
    },
    onError: () => {
      message.error('Failed to update permission');
    },
  });

  if (isLoading) return <Spin />;

  const user = data?.user;
  const sub = data?.subscription;
  const isPremium = sub?.status === 'premium';

  return (
    <>
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 24 }}>
        <Button onClick={() => navigate(-1)}>Back</Button>
        <h2 style={{ margin: 0 }}>User Detail</h2>
      </div>

      <Card style={{ marginBottom: 24 }}>
        <Descriptions title="Profile" bordered column={2}>
          <Descriptions.Item label="Name">{user?.name}</Descriptions.Item>
          <Descriptions.Item label="Email">{user?.email_address}</Descriptions.Item>
          <Descriptions.Item label="Phone">{user?.phone_number}</Descriptions.Item>
          <Descriptions.Item label="City">{user?.city}</Descriptions.Item>
          <Descriptions.Item label="Joined">
            {user?.created_at ? new Date(user.created_at).toLocaleDateString() : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="Subscription">
            <Tag color={isPremium ? 'gold' : 'default'}>{sub?.status ?? 'free'}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="Subscription Expires">
            {sub?.expires_at ? new Date(sub.expires_at).toLocaleDateString() : '-'}
          </Descriptions.Item>
        </Descriptions>

        <div style={{ marginTop: 16, display: 'flex', gap: 8 }}>
          <Button
            type="primary"
            disabled={isPremium}
            onClick={() => setGrantModalOpen(true)}
          >
            Grant Premium
          </Button>
          <Button
            danger
            disabled={!isPremium}
            loading={permissionMutation.isPending && !grantModalOpen}
            onClick={() =>
              Modal.confirm({
                title: 'Revoke Premium',
                content: "Are you sure you want to revoke this user's premium access?",
                okText: 'Revoke',
                okButtonProps: { danger: true },
                onOk: () => permissionMutation.mutate({ action: 'revoke' }),
              })
            }
          >
            Revoke Premium
          </Button>
        </div>
      </Card>

      <Card title="Payment History">
        <Table
          dataSource={sub ? [sub] : []}
          rowKey="id"
          columns={[
            { title: 'Order ID', dataIndex: 'midtrans_order_id', key: 'order_id' },
            {
              title: 'Status',
              dataIndex: 'status',
              key: 'status',
              render: (v: string) => (
                <Tag color={v === 'premium' ? 'green' : 'default'}>{v}</Tag>
              ),
            },
            {
              title: 'Expires At',
              dataIndex: 'expires_at',
              key: 'expires_at',
              render: (v: string) => (v ? new Date(v).toLocaleDateString() : '-'),
            },
          ]}
        />
      </Card>

      <Modal
        title="Grant Premium Access"
        open={grantModalOpen}
        onOk={() =>
          form
            .validateFields()
            .then((values) => permissionMutation.mutate({ action: 'grant', duration_days: values.days }))
        }
        onCancel={() => {
          setGrantModalOpen(false);
          form.resetFields();
        }}
        confirmLoading={permissionMutation.isPending}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="days"
            label="Duration (days)"
            rules={[{ required: true, message: 'Please enter duration in days' }]}
          >
            <InputNumber min={1} max={3650} style={{ width: '100%' }} placeholder="e.g. 30" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
