import { Table, Input, Button } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { usersApi } from '../../api/admin';

export default function UsersPage() {
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [perPage] = useState(20);
  const navigate = useNavigate();

  const { data, isLoading } = useQuery({
    queryKey: ['users', search, page, perPage],
    queryFn: () => usersApi.list({ search, page, per_page: perPage }),
  });

  return (
    <>
      <h2>Users</h2>
      <div style={{ display: 'flex', marginBottom: 16 }}>
        <Input.Search
          placeholder="Search by name or email"
          onSearch={setSearch}
          style={{ width: 300 }}
          allowClear
        />
      </div>
      <Table
        dataSource={data?.data}
        rowKey="id"
        loading={isLoading}
        pagination={{
          current: page,
          pageSize: perPage,
          total: data?.total,
          showTotal: (t) => `${t} users`,
          onChange: setPage,
        }}
        columns={[
          { title: 'Name', dataIndex: 'name', key: 'name' },
          { title: 'Email', dataIndex: 'email_address', key: 'email_address' },
          { title: 'City', dataIndex: 'city', key: 'city' },
          {
            title: 'Joined',
            dataIndex: 'created_at',
            key: 'created_at',
            render: (v: string) => new Date(v).toLocaleDateString(),
          },
          {
            title: 'Action',
            key: 'action',
            render: (_, record: { id: string }) => (
              <Button size="small" onClick={() => navigate(`/users/${record.id}`)}>
                View
              </Button>
            ),
          },
        ]}
      />
    </>
  );
}
