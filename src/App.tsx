import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { useEffect } from 'react';
import queryClient from './api/queryClient';
import { useAuthStore } from './store/authStore';
import ProtectedRoute from './components/ProtectedRoute';
import AppLayout from './components/AppLayout';
import LoginPage from './pages/LoginPage';
import DashboardPage from './pages/dashboard/DashboardPage';
import FairyTalesPage from './pages/content/FairyTalesPage';
import ArCardsPage from './pages/content/ArCardsPage';
import TracingPage from './pages/content/TracingPage';
import CountingPage from './pages/content/CountingPage';
import BadgesPage from './pages/content/BadgesPage';
import CategoriesPage from './pages/content/CategoriesPage';
import BannersPage from './pages/content/BannersPage';
import UsersPage from './pages/users/UsersPage';
import UserDetailPage from './pages/users/UserDetailPage';
import PaymentsPage from './pages/payments/PaymentsPage';
import CampaignsPage from './pages/campaigns/CampaignsPage';
import PremiumPackagesPage from './pages/packages/PremiumPackagesPage';
import ArCardCategoriesPage from './pages/content/ArCardCategoriesPage';
import DongengCategoriesPage from './pages/content/DongengCategoriesPage';
import ProductsPage from './pages/products/ProductsPage';
import OrdersPage from './pages/orders/OrdersPage';
import FeatureFlagsPage from './pages/settings/FeatureFlagsPage';

function AppInit() {
  const initFromStorage = useAuthStore((s) => s.initFromStorage);
  useEffect(() => {
    initFromStorage();
  }, [initFromStorage]);
  return null;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AppInit />
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <AppLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<DashboardPage />} />
            <Route path="content/fairy-tales" element={<FairyTalesPage />} />
            <Route path="content/ar-cards" element={<ArCardsPage />} />
            <Route path="content/tracing" element={<TracingPage />} />
            <Route path="content/counting" element={<CountingPage />} />
            <Route path="content/badges" element={<BadgesPage />} />
            <Route path="content/categories" element={<CategoriesPage />} />
            <Route path="content/ar-card-categories" element={<ArCardCategoriesPage />} />
            <Route path="content/dongeng-categories" element={<DongengCategoriesPage />} />
            <Route path="content/banners" element={<BannersPage />} />
            <Route path="users" element={<UsersPage />} />
            <Route path="users/:id" element={<UserDetailPage />} />
            <Route path="payments" element={<PaymentsPage />} />
            <Route path="campaigns" element={<CampaignsPage />} />
            <Route path="packages" element={<PremiumPackagesPage />} />
            <Route path="products" element={<ProductsPage />} />
            <Route path="orders" element={<OrdersPage />} />
            <Route path="feature-flags" element={<FeatureFlagsPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
