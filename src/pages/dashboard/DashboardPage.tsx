import { Row, Col, Card, Statistic, Spin } from 'antd';
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
import { useQuery } from '@tanstack/react-query';
import { analyticsApi } from '../../api/analytics';

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, BarElement, Title, Tooltip, Legend);

const REFETCH_INTERVAL = 30_000;

export default function DashboardPage() {
  const { data: dau, isLoading: dauLoading } = useQuery({
    queryKey: ['dau'],
    queryFn: () => analyticsApi.getDAU(30),
    refetchInterval: REFETCH_INTERVAL,
  });

  const { data: newUsers, isLoading: nuLoading } = useQuery({
    queryKey: ['newUsers'],
    queryFn: () => analyticsApi.getNewUsers(30),
    refetchInterval: REFETCH_INTERVAL,
  });

  const { data: features, isLoading: featLoading } = useQuery({
    queryKey: ['popularFeatures'],
    queryFn: () => analyticsApi.getPopularFeatures(),
    refetchInterval: REFETCH_INTERVAL,
  });

  const { data: payments, isLoading: payLoading } = useQuery({
    queryKey: ['paymentMetrics'],
    queryFn: () => analyticsApi.getPayments(),
    refetchInterval: REFETCH_INTERVAL,
  });

  const { data: subStats, isLoading: subLoading } = useQuery({
    queryKey: ['subscriptionStats'],
    queryFn: () => analyticsApi.getSubscriptionStats(),
    refetchInterval: REFETCH_INTERVAL,
  });

  const todayDAU = dau?.[dau.length - 1]?.count ?? 0;
  const todayNewUsers = newUsers?.[newUsers.length - 1]?.count ?? 0;

  const successPayments = payments?.find((p: { status: string }) => p.status === 'premium')?.count ?? 0;
  const totalPayments = payments?.reduce((sum: number, p: { count: number }) => sum + p.count, 0) ?? 0;
  const successRate = totalPayments > 0 ? Math.round((successPayments / totalPayments) * 100) : 0;

  const totalUsers: number = subStats?.total ?? 0;
  const premiumUsers: number = subStats?.premium ?? 0;
  const freeUsers: number = subStats?.free ?? 0;
  const premiumRate = totalUsers > 0 ? Math.round((premiumUsers / totalUsers) * 100) : 0;

  const dauChartData = {
    labels: dau?.map((d: { date: string }) => d.date) ?? [],
    datasets: [
      {
        label: 'Daily Active Users',
        data: dau?.map((d: { count: number }) => d.count) ?? [],
        borderColor: '#1890ff',
        tension: 0.3,
      },
    ],
  };

  const nuChartData = {
    labels: newUsers?.map((d: { date: string }) => d.date) ?? [],
    datasets: [
      {
        label: 'New Users',
        data: newUsers?.map((d: { count: number }) => d.count) ?? [],
        borderColor: '#52c41a',
        tension: 0.3,
      },
    ],
  };

  const featureChartData = {
    labels: features?.map((f: { feature: string }) => f.feature) ?? [],
    datasets: [
      {
        label: 'Usage Count',
        data: features?.map((f: { count: number }) => f.count) ?? [],
        backgroundColor: '#1890ff',
      },
    ],
  };

  const paymentChartData = {
    labels: payments?.map((p: { status: string }) => p.status) ?? [],
    datasets: [
      {
        label: 'Transactions',
        data: payments?.map((p: { count: number }) => p.count) ?? [],
        backgroundColor: ['#52c41a', '#faad14', '#ff4d4f'],
      },
    ],
  };

  const subscriptionChartData = {
    labels: ['Premium', 'Free'],
    datasets: [
      {
        label: 'Users',
        data: [premiumUsers, freeUsers],
        backgroundColor: ['#faad14', '#1890ff'],
      },
    ],
  };

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>Dashboard</h2>

      {/* KPI Row 1 — Activity */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="DAU (Today)" value={todayDAU} loading={dauLoading} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="New Users (Today)" value={todayNewUsers} loading={nuLoading} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="Successful Payments" value={successPayments} loading={payLoading} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="Payment Success Rate" value={successRate} suffix="%" loading={payLoading} />
          </Card>
        </Col>
      </Row>

      {/* KPI Row 2 — Subscription Stats */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={8} lg={6}>
          <Card>
            <Statistic
              title="Total Registered Users"
              value={totalUsers}
              loading={subLoading}
            />
          </Card>
        </Col>
        <Col xs={24} sm={8} lg={6}>
          <Card>
            <Statistic
              title="Premium Users"
              value={premiumUsers}
              loading={subLoading}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={8} lg={6}>
          <Card>
            <Statistic
              title="Free Users"
              value={freeUsers}
              loading={subLoading}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={8} lg={6}>
          <Card>
            <Statistic
              title="Premium Conversion Rate"
              value={premiumRate}
              suffix="%"
              loading={subLoading}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
      </Row>

      {/* Charts Row 1 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={12}>
          <Card title="Daily Active Users (30 days)">
            {dauLoading ? <Spin /> : <Line data={dauChartData} />}
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="New Users (30 days)">
            {nuLoading ? <Spin /> : <Line data={nuChartData} />}
          </Card>
        </Col>
      </Row>

      {/* Charts Row 2 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={8}>
          <Card title="Popular Features">
            {featLoading ? <Spin /> : <Bar data={featureChartData} />}
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card title="Payment Metrics (by subscription status)">
            {payLoading ? <Spin /> : <Bar data={paymentChartData} />}
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card title="Premium vs Free Users">
            {subLoading ? <Spin /> : <Bar data={subscriptionChartData} />}
          </Card>
        </Col>
      </Row>
    </div>
  );
}
