import { Table, Input, Button, Space, Tag, Popconfirm, Switch, Tooltip } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { ExpandableConfig } from 'antd/es/table/interface';

export interface ContentItem {
  id: string;
  hidden?: boolean;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
}

interface ContentTableProps<T extends ContentItem> {
  data: T[] | undefined;
  total: number;
  page: number;
  perPage: number;
  loading: boolean;
  columns: ColumnsType<T>;
  onSearch: (value: string) => void;
  onPageChange: (page: number, pageSize: number) => void;
  onAdd: () => void;
  onEdit: (item: T) => void;
  onDelete: (id: string) => void;
  onToggleVisibility: (id: string, hidden: boolean) => void;
  expandable?: ExpandableConfig<T>;
}

export default function ContentTable<T extends ContentItem>({
  data,
  total,
  page,
  perPage,
  loading,
  columns,
  onSearch,
  onPageChange,
  onAdd,
  onEdit,
  onDelete,
  onToggleVisibility,
  expandable,
}: ContentTableProps<T>) {
  const actionColumn: ColumnsType<T>[0] = {
    title: 'Actions',
    key: 'actions',
    width: 180,
    render: (_, record) => (
      <Space>
        <Tooltip title={record.hidden ? 'Show' : 'Hide'}>
          <Switch
            size="small"
            checked={!record.hidden}
            onChange={(checked) => onToggleVisibility(record.id, !checked)}
          />
        </Tooltip>
        <Button
          size="small"
          icon={<EditOutlined />}
          onClick={() => onEdit(record)}
        />
        <Popconfirm
          title="Delete this item?"
          onConfirm={() => onDelete(record.id)}
          okText="Yes"
          cancelText="No"
        >
          <Button size="small" danger icon={<DeleteOutlined />} />
        </Popconfirm>
      </Space>
    ),
  };

  const hiddenColumn: ColumnsType<T>[0] = {
    title: 'Status',
    key: 'visible',
    width: 90,
    render: (_, record) =>
      record.hidden ? (
        <Tag color="default">Hidden</Tag>
      ) : (
        <Tag color="green">Visible</Tag>
      ),
  };

  return (
    <div>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          marginBottom: 16,
        }}
      >
        <Input.Search
          placeholder="Search..."
          onSearch={onSearch}
          style={{ width: 280 }}
          allowClear
        />
        <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>
          Add New
        </Button>
      </div>
      <Table<T>
        dataSource={data}
        columns={[...columns, hiddenColumn, actionColumn]}
        rowKey="id"
        loading={loading}
        expandable={expandable}
        pagination={{
          current: page,
          pageSize: perPage,
          total,
          showSizeChanger: true,
          showTotal: (t) => `${t} items`,
          onChange: onPageChange,
        }}
      />
    </div>
  );
}
