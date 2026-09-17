import { Table, Switch, Typography, Tag, message } from 'antd';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { featureFlagsApi } from '../../api/admin';
import type { FeatureFlag } from '../../api/admin';
import type { ColumnsType } from 'antd/es/table';

const { Text, Paragraph } = Typography;

export default function FeatureFlagsPage() {
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ['feature-flags'],
    queryFn: featureFlagsApi.list,
  });

  const toggleMutation = useMutation({
    mutationFn: ({ key, isEnabled }: { key: string; isEnabled: boolean }) =>
      featureFlagsApi.toggle(key, isEnabled),
    onSuccess: (res) => {
      message.success(`${res.data.name} ${res.data.is_enabled ? 'shown' : 'hidden'} in the app`);
      queryClient.invalidateQueries({ queryKey: ['feature-flags'] });
    },
    onError: () => message.error('Failed to update feature'),
  });

  const columns: ColumnsType<FeatureFlag> = [
    {
      title: 'Feature',
      key: 'name',
      render: (_: unknown, record: FeatureFlag) => (
        <>
          <div>
            <Text strong>{record.name}</Text> <Text type="secondary" code>{record.key}</Text>
          </div>
          <Text type="secondary">{record.description}</Text>
        </>
      ),
    },
    {
      title: 'Status',
      key: 'status',
      width: 120,
      render: (_: unknown, record: FeatureFlag) =>
        record.is_enabled ? <Tag color="green">Visible</Tag> : <Tag>Hidden</Tag>,
    },
    {
      title: 'Show in app',
      key: 'toggle',
      width: 120,
      render: (_: unknown, record: FeatureFlag) => (
        <Switch
          aria-label={`Toggle ${record.name}`}
          checked={record.is_enabled}
          loading={toggleMutation.isPending && toggleMutation.variables?.key === record.key}
          onChange={(checked) => toggleMutation.mutate({ key: record.key, isEnabled: checked })}
        />
      ),
    },
    {
      title: 'Last changed',
      dataIndex: 'updated_at',
      key: 'updated_at',
      width: 200,
      render: (v: string) => new Date(v).toLocaleString('id-ID'),
    },
  ];

  return (
    <>
      <h2>App Features</h2>
      <Paragraph type="secondary">
        Hide or show features in the mobile app. Changes reach users the next time the app starts or
        returns to the foreground.
      </Paragraph>
      <Table
        rowKey="key"
        columns={columns}
        dataSource={data?.data ?? []}
        loading={isLoading}
        pagination={false}
      />
    </>
  );
}
