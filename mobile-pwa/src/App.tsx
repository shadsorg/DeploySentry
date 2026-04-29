import { Routes, Route, Navigate } from 'react-router-dom';
import { RequireAuth, RedirectIfAuth } from './auth';
import { MobileLayout } from './layout/MobileLayout';
import { LoginPage } from './pages/LoginPage';
import { OrgPickerPage } from './pages/OrgPickerPage';
import { StatusPage } from './pages/StatusPage';
import { HistoryPage } from './pages/HistoryPage';
import { DeploymentDetailPage } from './pages/DeploymentDetailPage';
import { FlagProjectPickerPage } from './pages/FlagProjectPickerPage';
import { SettingsPage } from './pages/SettingsPage';

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<RedirectIfAuth />}>
        <Route path="/login" element={<LoginPage />} />
      </Route>
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Navigate to="/orgs" replace />} />
        <Route path="/orgs" element={<OrgPickerPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/orgs/:orgSlug" element={<MobileLayout />}>
          <Route index element={<Navigate to="status" replace />} />
          <Route path="status" element={<StatusPage />} />
          <Route path="history" element={<HistoryPage />} />
          <Route path="history/:deploymentId" element={<DeploymentDetailPage />} />
          <Route path="flags" element={<FlagProjectPickerPage />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
