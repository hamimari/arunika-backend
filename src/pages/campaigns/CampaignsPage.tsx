import { Form, Input, Select, Button, Card, Alert, Modal, Descriptions } from 'antd';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { campaignsApi } from '../../api/admin';

interface CampaignResult {
  sent: number;
  failed: number;
}

export default function CampaignsPage() {
  const [form] = Form.useForm();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [formValues, setFormValues] = useState<Record<string, string> | null>(null);
  const [result, setResult] = useState<CampaignResult | null>(null);

  const mutation = useMutation({
    mutationFn: campaignsApi.dispatch,
    onSuccess: (data) => {
      setResult(data);
      form.resetFields();
    },
  });

  const handleSubmit = async () => {
    const values = await form.validateFields();
    setFormValues(values);
    setConfirmOpen(true);
  };

  const handleConfirm = () => {
    if (formValues) {
      mutation.mutate(formValues as Parameters<typeof campaignsApi.dispatch>[0]);
    }
    setConfirmOpen(false);
  };

  return (
    <>
      <h2>Campaign Notifications</h2>

      {result && (
        <Alert
          type="success"
          message={`Campaign dispatched: ${result.sent} sent, ${result.failed} failed`}
          closable
          onClose={() => setResult(null)}
          style={{ marginBottom: 24 }}
        />
      )}
      {mutation.isError && (
        <Alert
          type="error"
          message="Failed to dispatch campaign"
          closable
          style={{ marginBottom: 24 }}
        />
      )}

      <Card style={{ maxWidth: 600 }}>
        <Form form={form} layout="vertical">
          <Form.Item name="title" label="Title" rules={[{ required: true }]}>
            <Input placeholder="Campaign title" />
          </Form.Item>
          <Form.Item name="body" label="Message" rules={[{ required: true }]}>
            <Input.TextArea rows={4} placeholder="Campaign message body..." />
          </Form.Item>
          <Form.Item name="channel" label="Channel" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'push', label: 'Push Notification Only' },
                { value: 'email', label: 'Email Only' },
                { value: 'both', label: 'Push + Email' },
              ]}
            />
          </Form.Item>
          <Form.Item name="segment" label="Target Segment" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'all', label: 'All Users' },
                { value: 'subscribers', label: 'Premium Subscribers Only' },
              ]}
            />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              onClick={handleSubmit}
              loading={mutation.isPending}
              block
            >
              Send Campaign
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Modal
        title="Confirm Campaign Dispatch"
        open={confirmOpen}
        onOk={handleConfirm}
        onCancel={() => setConfirmOpen(false)}
        okText="Yes, Send"
        okButtonProps={{ danger: true }}
      >
        {formValues && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="Title">{formValues.title}</Descriptions.Item>
            <Descriptions.Item label="Body">{formValues.body}</Descriptions.Item>
            <Descriptions.Item label="Channel">{formValues.channel}</Descriptions.Item>
            <Descriptions.Item label="Segment">{formValues.segment}</Descriptions.Item>
          </Descriptions>
        )}
        <p style={{ marginTop: 16, color: '#ff4d4f' }}>
          This will send notifications to users. Are you sure?
        </p>
      </Modal>
    </>
  );
}
