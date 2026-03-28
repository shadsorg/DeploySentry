import { Routes, Route } from 'react-router-dom';
import { AuthProvider, RequireAuth, RedirectIfAuth } from './auth';
import HierarchyLayout from './components/HierarchyLayout';
import DefaultRedirect from './components/DefaultRedirect';
import LegacyRedirect from './components/LegacyRedirect';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import ProjectListPage from './pages/ProjectListPage';
import CreateOrgPage from './pages/CreateOrgPage';
import FlagListPage from './pages/FlagListPage';
import FlagDetailPage from './pages/FlagDetailPage';
import FlagCreatePage from './pages/FlagCreatePage';
import DeploymentsPage from './pages/DeploymentsPage';
import ReleasesPage from './pages/ReleasesPage';
import AnalyticsPage from './pages/AnalyticsPage';
import SDKsPage from './pages/SDKsPage';
import SettingsPage from './pages/SettingsPage';

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        {/* Public routes */}
        <Route element={<RedirectIfAuth />}>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
        </Route>

        {/* Authenticated routes */}
        <Route element={<RequireAuth />}>
          {/* Default redirect */}
          <Route path="/" element={<DefaultRedirect />} />

          {/* Create org (outside HierarchyLayout — no sidebar context yet) */}
          <Route path="/orgs/new" element={<CreateOrgPage />} />

          {/* Hierarchy layout */}
          <Route path="/orgs/:orgSlug" element={<HierarchyLayout />}>
            {/* Org-level */}
            <Route path="projects" element={<ProjectListPage />} />
            <Route path="members" element={<SettingsPage level="org" tab="members" />} />
            <Route path="api-keys" element={<SettingsPage level="org" tab="api-keys" />} />
            <Route path="settings" element={<SettingsPage level="org" />} />

            {/* Project-level */}
            <Route path="projects/:projectSlug">
              <Route path="flags" element={<FlagListPage />} />
              <Route path="flags/new" element={<FlagCreatePage />} />
              <Route path="flags/:id" element={<FlagDetailPage />} />
              <Route path="analytics" element={<AnalyticsPage />} />
              <Route path="sdks" element={<SDKsPage />} />
              <Route path="settings" element={<SettingsPage level="project" />} />

              {/* App-level */}
              <Route path="apps/:appSlug">
                <Route path="deployments" element={<DeploymentsPage />} />
                <Route path="deployments/:id" element={<DeploymentsPage />} />
                <Route path="releases" element={<ReleasesPage />} />
                <Route path="releases/:id" element={<ReleasesPage />} />
                <Route path="flags" element={<FlagListPage />} />
                <Route path="flags/new" element={<FlagCreatePage />} />
                <Route path="flags/:id" element={<FlagDetailPage />} />
                <Route path="settings" element={<SettingsPage level="app" />} />
              </Route>
            </Route>
          </Route>

          {/* Legacy redirects */}
          <Route path="/flags" element={<LegacyRedirect to="flags" />} />
          <Route path="/flags/new" element={<LegacyRedirect to="flags/new" />} />
          <Route path="/deployments" element={<LegacyRedirect to="deployments" />} />
          <Route path="/releases" element={<LegacyRedirect to="releases" />} />
          <Route path="/analytics" element={<LegacyRedirect to="analytics" />} />
          <Route path="/sdks" element={<LegacyRedirect to="sdks" />} />
          <Route path="/settings" element={<LegacyRedirect to="settings" />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
