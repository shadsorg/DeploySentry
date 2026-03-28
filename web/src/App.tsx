import { useEffect } from 'react';
import { Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import { AuthProvider, RequireAuth, RedirectIfAuth } from './auth';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import DashboardPage from './pages/DashboardPage';
import FlagListPage from './pages/FlagListPage';
import FlagDetailPage from './pages/FlagDetailPage';
import FlagCreatePage from './pages/FlagCreatePage';
import DeploymentsPage from './pages/DeploymentsPage';
import ReleasesPage from './pages/ReleasesPage';
import AnalyticsPage from './pages/AnalyticsPage';
import SDKsPage from './pages/SDKsPage';
import SettingsPage from './pages/SettingsPage';
import RealtimeManager from './services/realtime';

export default function App() {
  useEffect(() => {
    const initializeRealtime = async () => {
      try {
        const realtimeManager = RealtimeManager.getInstance();
        await realtimeManager.initialize({
          baseUrl: window.location.origin,
          refreshInterval: 30000,
        });
      } catch (error) {
        console.warn('[App] Failed to initialize real-time updates:', error);
      }
    };

    initializeRealtime();
    return () => { RealtimeManager.getInstance().dispose(); };
  }, []);

  return (
    <AuthProvider>
      <Routes>
        {/* Public routes */}
        <Route path="/login" element={<RedirectIfAuth><LoginPage /></RedirectIfAuth>} />
        <Route path="/register" element={<RedirectIfAuth><RegisterPage /></RedirectIfAuth>} />

        {/* Protected routes */}
        <Route element={<RequireAuth><Layout /></RequireAuth>}>
          <Route index element={<DashboardPage />} />
          <Route path="flags" element={<FlagListPage />} />
          <Route path="flags/new" element={<FlagCreatePage />} />
          <Route path="flags/:id" element={<FlagDetailPage />} />
          <Route path="deployments" element={<DeploymentsPage />} />
          <Route path="releases" element={<ReleasesPage />} />
          <Route path="analytics" element={<AnalyticsPage />} />
          <Route path="sdks" element={<SDKsPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}