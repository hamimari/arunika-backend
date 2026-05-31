import { Alert, Card, Col, Row, Select, Spin, Typography } from 'antd';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js';
import { Line, Bar } from 'react-chartjs-2';
import { analyticsApi } from '../../api/analytics';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  Title,
  Tooltip,
  Legend,
);

const { Title: AntTitle } = Typography;

const dayOptions = [
  { value: 7, label: 'Last 7 days' },
  { value: 14, label: 'Last 14 days' },
  { value: 30, label: 'Last 30 days' },
  { value: 60, label: 'Last 60 days' },
  { value: 90, label: 'Last 90 days' },
];

interface DayEntry {
  date: string;
  count: number;
}

function chartData(entries: DayEntry[], label: string, color: string) {
  return {
    labels: entries.map((e) => e.date),
    datasets: [
      {
        label,
        data: entries.map((e) => e.count),
        borderColor: color,
        backgroundColor: color + '33',
        tension: 0.3,
        fill: true,
      },
    ],
  };
}

const chartOptions = {
  responsive: true,
  plugins: { legend: { display: false } },
  scales: { y: { beginAtZero: true, ticks: { precision: 0 } } },
};

export default function AnalyticsPage() {
  const [days, setDays] = useState(30);

  const dauQuery = useQuery<DayEntry[]>({
    queryKey: ['analytics', 'dau', days],
    queryFn: () => analyticsApi.getDAU(days),
    retry: 1,
  });

  const newUsersQuery = useQuery<DayEntry[]>({
    queryKey: ['analytics', 'new-users', days],
    queryFn: () => analyticsApi.getNewUsers(days),
    retry: 1,
  });

  return (
    <>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <AntTitle level={2} style={{ margin: 0 }}>Analytics</AntTitle>
        <Select
          value={days}
          onChange={setDays}
          options={dayOptions}
          style={{ width: 160 }}
        />
      </div>

      <Row gutter={[24, 24]}>
        <Col xs={24} xl={12}>
          <Card title="Daily Active Users (DAU)">
            {dauQuery.isError ? (
              <Alert
                type="error"
                message="Failed to load DAU data"
                description={
                  dauQuery.error instanceof Error
                    ? dauQuery.error.message
                    : 'An unexpected error occurred. Please check the server logs.'
                }
              />
            ) : dauQuery.isLoading ? (
              <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
            ) : (
              <Line
                data={chartData(dauQuery.data ?? [], 'DAU', '#1677ff')}
                options={chartOptions}
              />
            )}
          </Card>
        </Col>

        <Col xs={24} xl={12}>
          <Card title="New Users">
            {newUsersQuery.isError ? (
              <Alert
                type="error"
                message="Failed to load new-users data"
                description={
                  newUsersQuery.error instanceof Error
                    ? newUsersQuery.error.message
                    : 'An unexpected error occurred. Please check the server logs.'
                }
              />
            ) : newUsersQuery.isLoading ? (
              <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
            ) : (
              <Bar
                data={chartData(newUsersQuery.data ?? [], 'New Users', '#52c41a')}
                options={chartOptions}
              />
            )}
          </Card>
        </Col>
      </Row>
    </>
  );
}
