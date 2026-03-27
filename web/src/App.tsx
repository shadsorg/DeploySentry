import { Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import DashboardPage from './pages/DashboardPage';
import FlagListPage from './pages/FlagListPage';
import FlagDetailPage from './pages/FlagDetailPage';
import FlagCreatePage from './pages/FlagCreatePage';
import DeploymentsPage from './pages/DeploymentsPage';
import ReleasesPage from './pages/ReleasesPage';
import SDKsPage from './pages/SDKsPage';
import SettingsPage from './pages/SettingsPage';

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<DashboardPage />} />
        <Route path="flags" element={<FlagListPage />} />
        <Route path="flags/new" element={<FlagCreatePage />} />
        <Route path="flags/:id" element={<FlagDetailPage />} />
        <Route path="deployments" element={<DeploymentsPage />} />
        <Route path="releases" element={<ReleasesPage />} />
        <Route path="sdks" element={<SDKsPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}
