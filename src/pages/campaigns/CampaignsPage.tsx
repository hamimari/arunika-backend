import {
  Form,
  Input,
  Select,
  Button,
  Card,
  Alert,
  Modal,
  Descriptions,
  Table,
  Tag,
  Typography,
  Row,
  Col,
  message,
} from 'antd';
import { BellOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { campaignsApi } from '../../api/admin';
import type { Campaign, CampaignInput, CampaignLinkType, CampaignSegment } from '../../api/admin';
import { arCardsApi, fairyTalesApi } from '../../api/content';
import type { ColumnsType } from 'antd/es/table';

const { Text } = Typography;

const SEGMENT_LABELS: Record<CampaignSegment, string> = {
  all_devices: 'All app installs (incl. guests)',
  all: 'All registered users',
  subscribers: 'Premium subscribers only',
};

const CHANNEL_LABELS: Record<string, string> = {
  push: 'Push only',
  email: 'Email only',
  both: 'Push + Email',
};

const LINK_LABELS: Record<CampaignLinkType, string> = {
  none: 'Just open the app',
  ar_card: 'AR card',
  dongeng: 'Dongeng',
};

const STATUS_COLORS: Record<Campaign['status'], string> = {
  SENDING: 'processing',
  COMPLETED: 'green',
  FAILED: 'red',
};

interface ContentOption {
  id: string;
  title: string;
}

// Searchable picker for the AR card / dongeng a campaign links to. Lists the
// newest items first, so a just-released card is at the top.
function ContentPicker({
  linkType,
  value,
  onChange,
}: {
  linkType: 'ar_card' | 'dongeng';
  value?: string;
  onChange?: (v: string) => void;
}) {
  const [search, setSearch] = useState('');
  const [debounced, setDebounced] = useState('');
  useEffect(() => {
    const t = setTimeout(() => setDebounced(search), 300);
    return () => clearTimeout(t);
  }, [search]);

  const listApi = linkType === 'ar_card' ? arCardsApi : fairyTalesApi;
  const { data, isFetching } = useQuery({
    queryKey: ['campaign-link-options', linkType, debounced],
    queryFn: () => listApi.list({ search: debounced || undefined, page: 1, per_page: 20 }),
  });
  const items: ContentOption[] = data?.data ?? [];

  return (
    <Select
      showSearch
      value={value}
      onChange={onChange}
      onSearch={setSearch}
      filterOption={false}
      loading={isFetching}
      placeholder={linkType === 'ar_card' ? 'Search AR cards…' : 'Search dongeng…'}
      options={items.map((i) => ({ value: i.id, label: i.title }))}
      notFoundContent={isFetching ? 'Loading…' : 'No matches'}
    />
  );
}

function NotificationPreview({ title, body, imageUrl }: { title?: string; body?: string; imageUrl?: string }) {
  return (
    <Card size="small" style={{ background: '#f5f5f5' }}>
      <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}>
        <BellOutlined style={{ fontSize: 18, marginTop: 3, color: '#fa8c16' }} />
        <div style={{ minWidth: 0, flex: 1 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            Arunika · now
          </Text>
          <div>
            <Text strong ellipsis style={{ display: 'block' }}>
              {title || 'Notification title'}
            </Text>
          </div>
          <Text style={{ fontSize: 13 }}>{body || 'Notification message'}</Text>
          {imageUrl?.startsWith('https://') && (
            <img
              src={imageUrl}
              alt=""
              style={{ display: 'block', width: '100%', maxHeight: 160, objectFit: 'cover', marginTop: 8, borderRadius: 6 }}
            />
          )}
        </div>
      </div>
    </Card>
  );
}

export default function CampaignsPage() {
  const [form] = Form.useForm<CampaignInput>();
  const [confirmValues, setConfirmValues] = useState<CampaignInput | null>(null);
  const [page, setPage] = useState(1);
  const queryClient = useQueryClient();

  const segment = Form.useWatch('segment', form);
  const linkType = Form.useWatch('link_type', form);
  const title = Form.useWatch('title', form);
  const body = Form.useWatch('body', form);
  const imageUrl = Form.useWatch('image_url', form);

  // Guests have no email address, so the device-wide segment is push-only.
  useEffect(() => {
    if (segment === 'all_devices') form.setFieldValue('channel', 'push');
  }, [segment, form]);

  const history = useQuery({
    queryKey: ['campaigns', page],
    queryFn: () => campaignsApi.list({ page, per_page: 10 }),
    refetchInterval: (query) =>
      query.state.data?.data.some((c) => c.status === 'SENDING') ? 3000 : false,
  });

  const mutation = useMutation({
    mutationFn: campaignsApi.dispatch,
    onSuccess: () => {
      message.success('Campaign is being sent');
      form.resetFields();
      setPage(1);
      queryClient.invalidateQueries({ queryKey: ['campaigns'] });
    },
  });

  const handleSubmit = async () => {
    const values = await form.validateFields();
    setConfirmValues(values);
  };

  const handleConfirm = () => {
    if (confirmValues) {
      mutation.mutate({
        ...confirmValues,
        image_url: confirmValues.image_url?.trim() || undefined,
        link_id: confirmValues.link_type === 'none' ? undefined : confirmValues.link_id,
      });
    }
    setConfirmValues(null);
  };

  const mutationError =
    (mutation.error as { response?: { data?: { error?: string } } } | null)?.response?.data?.error;

  const columns: ColumnsType<Campaign> = [
    {
      title: 'Campaign',
      key: 'title',
      render: (_: unknown, c: Campaign) => (
        <>
          <Text strong>{c.title}</Text>
          <div>
            <Text type="secondary" ellipsis style={{ maxWidth: 320, display: 'inline-block' }}>
              {c.body}
            </Text>
          </div>
        </>
      ),
    },
    {
      title: 'Audience',
      key: 'audience',
      render: (_: unknown, c: Campaign) => (
        <>
          <div>{SEGMENT_LABELS[c.segment] ?? c.segment}</div>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {CHANNEL_LABELS[c.channel] ?? c.channel}
          </Text>
        </>
      ),
    },
    {
      title: 'Opens',
      key: 'link',
      render: (_: unknown, c: Campaign) =>
        c.link_type === 'none' ? <Text type="secondary">App</Text> : <Tag>{LINK_LABELS[c.link_type]}</Tag>,
    },
    {
      title: 'Status',
      key: 'status',
      render: (_: unknown, c: Campaign) => (
        <>
          <Tag color={STATUS_COLORS[c.status]}>{c.status}</Tag>
          {c.error && (
            <div>
              <Text type="danger" style={{ fontSize: 12 }}>
                {c.error}
              </Text>
            </div>
          )}
        </>
      ),
    },
    {
      title: 'Delivered',
      key: 'counts',
      render: (_: unknown, c: Campaign) =>
        c.segment === 'all_devices' ? (
          <Text type="secondary">{c.sent > 0 ? 'Sent to topic' : '—'}</Text>
        ) : (
          <span>
            {c.sent} sent · <Text type={c.failed > 0 ? 'danger' : 'secondary'}>{c.failed} failed</Text>
          </span>
        ),
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (v: string) => new Date(v).toLocaleString('id-ID'),
    },
  ];

  return (
    <>
      <h2>Campaign Notifications</h2>

      {mutation.isError && (
        <Alert
          type="error"
          message={mutationError ?? 'Failed to send campaign'}
          closable
          onClose={() => mutation.reset()}
          style={{ marginBottom: 24 }}
        />
      )}

      <Row gutter={24}>
        <Col xs={24} lg={14}>
          <Card>
            <Form
              form={form}
              layout="vertical"
              initialValues={{ segment: 'all_devices', channel: 'push', link_type: 'none' }}
            >
              <Form.Item name="title" label="Title" rules={[{ required: true, whitespace: true }, { max: 200 }]}>
                <Input placeholder="e.g. Kartu AR baru: Harimau Sumatera!" showCount maxLength={200} />
              </Form.Item>
              <Form.Item name="body" label="Message" rules={[{ required: true, whitespace: true }]}>
                <Input.TextArea rows={3} placeholder="Short message shown under the title" showCount />
              </Form.Item>
              <Form.Item
                name="image_url"
                label="Image URL (optional)"
                rules={[{ pattern: /^https:\/\//, message: 'Must be an https:// URL' }]}
              >
                <Input placeholder="https://… (shown as a large image on Android)" />
              </Form.Item>
              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item name="segment" label="Audience" rules={[{ required: true }]}>
                    <Select
                      options={(Object.keys(SEGMENT_LABELS) as CampaignSegment[]).map((k) => ({
                        value: k,
                        label: SEGMENT_LABELS[k],
                      }))}
                    />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item name="channel" label="Channel" rules={[{ required: true }]}>
                    <Select
                      disabled={segment === 'all_devices'}
                      options={Object.entries(CHANNEL_LABELS).map(([value, label]) => ({ value, label }))}
                    />
                  </Form.Item>
                </Col>
              </Row>
              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item name="link_type" label="When tapped, open">
                    <Select
                      onChange={() => form.setFieldValue('link_id', undefined)}
                      options={(Object.keys(LINK_LABELS) as CampaignLinkType[]).map((k) => ({
                        value: k,
                        label: LINK_LABELS[k],
                      }))}
                    />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  {(linkType === 'ar_card' || linkType === 'dongeng') && (
                    <Form.Item
                      name="link_id"
                      label={linkType === 'ar_card' ? 'AR card' : 'Dongeng'}
                      rules={[{ required: true, message: 'Pick the content to open' }]}
                    >
                      <ContentPicker key={linkType} linkType={linkType} />
                    </Form.Item>
                  )}
                </Col>
              </Row>
              <Form.Item style={{ marginBottom: 0 }}>
                <Button type="primary" onClick={handleSubmit} loading={mutation.isPending} block>
                  Send Campaign
                </Button>
              </Form.Item>
            </Form>
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Text type="secondary">Preview</Text>
          <div style={{ marginTop: 8 }}>
            <NotificationPreview title={title} body={body} imageUrl={imageUrl} />
          </div>
        </Col>
      </Row>

      <h3 style={{ marginTop: 32 }}>History</h3>
      <Table
        rowKey="id"
        columns={columns}
        dataSource={history.data?.data ?? []}
        loading={history.isLoading}
        scroll={{ x: true }}
        pagination={{
          current: page,
          pageSize: 10,
          total: history.data?.total ?? 0,
          onChange: setPage,
        }}
      />

      <Modal
        title="Confirm Campaign"
        open={confirmValues !== null}
        onOk={handleConfirm}
        onCancel={() => setConfirmValues(null)}
        okText="Yes, Send"
        okButtonProps={{ danger: true }}
      >
        {confirmValues && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="Title">{confirmValues.title}</Descriptions.Item>
            <Descriptions.Item label="Message">{confirmValues.body}</Descriptions.Item>
            <Descriptions.Item label="Audience">{SEGMENT_LABELS[confirmValues.segment]}</Descriptions.Item>
            <Descriptions.Item label="Channel">{CHANNEL_LABELS[confirmValues.channel]}</Descriptions.Item>
            <Descriptions.Item label="Opens">{LINK_LABELS[confirmValues.link_type]}</Descriptions.Item>
          </Descriptions>
        )}
        <p style={{ marginTop: 16, color: '#ff4d4f' }}>
          This sends a notification to real users and cannot be undone. Are you sure?
        </p>
      </Modal>
    </>
  );
}
