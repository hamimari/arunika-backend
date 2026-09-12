import { Layout, Menu, Button, Typography } from 'antd';
import {
  DashboardOutlined,
  BookOutlined,
  UserOutlined,
  CreditCardOutlined,
  NotificationOutlined,
  LogoutOutlined,
  PictureOutlined,
  EditOutlined,
  NumberOutlined,
  TagOutlined,
  AppstoreOutlined,
  GiftOutlined,
  ShoppingOutlined,
  FileTextOutlined,
} from '@ant-design/icons';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';

const { Header, Sider, Content } = Layout;
const { Text } = Typography;

const menuItems = [
  { key: '/', icon: <DashboardOutlined />, label: 'Dashboard' },
  {
    key: 'content',
    icon: <AppstoreOutlined />,
    label: 'Content',
    children: [
      { key: '/content/fairy-tales', icon: <BookOutlined />, label: 'Fairy Tales' },
      { key: '/content/ar-cards', icon: <PictureOutlined />, label: 'AR Cards' },
      { key: '/content/tracing', icon: <EditOutlined />, label: 'Tracing Items' },
      { key: '/content/counting', icon: <NumberOutlined />, label: 'Counting Questions' },
      { key: '/content/badges', icon: <TagOutlined />, label: 'Badges' },
      { key: '/content/categories', icon: <AppstoreOutlined />, label: 'Categories' },
      { key: '/content/ar-card-categories', icon: <AppstoreOutlined />, label: 'AR Card Categories' },
      { key: '/content/dongeng-categories', icon: <AppstoreOutlined />, label: 'Dongeng Categories' },
      { key: '/content/banners', icon: <PictureOutlined />, label: 'Banners' },
    ],
  },
  { key: '/users', icon: <UserOutlined />, label: 'Users' },
  { key: '/payments', icon: <CreditCardOutlined />, label: 'Payments' },
  { key: '/campaigns', icon: <NotificationOutlined />, label: 'Campaigns' },
  { key: '/packages', icon: <GiftOutlined />, label: 'Premium Packages' },
  { key: '/products', icon: <ShoppingOutlined />, label: 'Products' },
  { key: '/orders', icon: <FileTextOutlined />, label: 'Orders' },
];

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const logout = useAuthStore((s) => s.logout);

  const handleMenuClick = ({ key }: { key: string }) => {
    navigate(key);
  };

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        width={220}
        style={{ background: '#001529' }}
        breakpoint="lg"
        collapsedWidth="0"
      >
        <div
          style={{
            height: 64,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Text style={{ color: '#fff', fontSize: 18, fontWeight: 700 }}>
            Arunika Admin
          </Text>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          defaultOpenKeys={['content']}
          items={menuItems}
          onClick={handleMenuClick}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            background: '#fff',
            padding: '0 24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-end',
            borderBottom: '1px solid #f0f0f0',
          }}
        >
          <Button
            icon={<LogoutOutlined />}
            type="text"
            onClick={handleLogout}
          >
            Logout
          </Button>
        </Header>
        <Content style={{ margin: '24px', background: '#fff', padding: 24, borderRadius: 8 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
