import { useEffect } from 'react';
import { Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
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
  // Initialize real-time updates when the app starts
  useEffect(() => {
    const initializeRealtime = async () => {
      try {
        const realtimeManager = RealtimeManager.getInstance();

        // Initialize with current origin and 30s refresh interval
        await realtimeManager.initialize({
          baseUrl: window.location.origin,
          refreshInterval: 30000, // 30 seconds
        });

        console.log('[App] Real-time updates initialized');
      } catch (error) {
        console.warn('[App] Failed to initialize real-time updates:', error);
        // Don't fail the app if real-time features aren't available
      }
    };

    initializeRealtime();

    // Cleanup on unmount
    return () => {
      RealtimeManager.getInstance().dispose();
    };
  }, []);

  return (
    <Routes>
      <Route element={<Layout />}>
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
  );
}
