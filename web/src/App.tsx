import { Routes, Route, Navigate } from 'react-router-dom';
import { lazy, Suspense } from 'react';
import { AuthProvider, RequireAuth, RedirectIfAuth } from './auth';
import SessionExpiryWarning from './components/SessionExpiryWarning';
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
import DeploymentDetailPage from './pages/DeploymentDetailPage';
import ReleaseDetailPage from './pages/ReleaseDetailPage';
import AnalyticsPage from './pages/AnalyticsPage';
import SettingsPage from './pages/SettingsPage';
import MembersPage from './pages/MembersPage';
import APIKeysPage from './pages/APIKeysPage';
import StrategiesPage from './pages/StrategiesPage';
import RolloutsPage from './pages/RolloutsPage';
import RolloutDetailPage from './pages/RolloutDetailPage';
import RolloutGroupsPage from './pages/RolloutGroupsPage';
import RolloutGroupDetailPage from './pages/RolloutGroupDetailPage';
import CreateAppPage from './pages/CreateAppPage';
import ProjectPage from './pages/ProjectPage';
import AppPage from './pages/AppPage';
import ProjectAppsTab from './pages/ProjectAppsTab';
import CreateProjectPage from './pages/CreateProjectPage';
import OrgStatusPage from './pages/OrgStatusPage';
import OrgDeploymentsPage from './pages/OrgDeploymentsPage';
const LandingPage = lazy(() => import('./pages/LandingPage'));
const DocsPage = lazy(() => import('./pages/DocsPage'));

export default function App() {
  return (
    <AuthProvider>
      <SessionExpiryWarning />
      <Suspense fallback={<div className="page-loading">Loading...</div>}>
      <Routes>
        {/* Public routes */}
        <Route path="/" element={<LandingPage />} />
        <Route element={<RedirectIfAuth />}>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
        </Route>

        {/* Authenticated routes */}
        <Route element={<RequireAuth />}>
          <Route path="/portal" element={<DefaultRedirect />} />
          <Route path="/docs" element={<DocsPage />} />
          <Route path="/docs/:slug" element={<DocsPage />} />

          {/* Create org (outside HierarchyLayout — no sidebar context yet) */}
          <Route path="/orgs/new" element={<CreateOrgPage />} />

          {/* Hierarchy layout */}
          <Route path="/orgs/:orgSlug" element={<HierarchyLayout />}>
            {/* Org-level */}
            <Route path="status" element={<OrgStatusPage />} />
            <Route path="deployments" element={<OrgDeploymentsPage />} />
            <Route path="projects" element={<ProjectListPage />} />
            <Route path="projects/new" element={<CreateProjectPage />} />
            <Route path="members" element={<MembersPage />} />
            <Route path="api-keys" element={<APIKeysPage />} />
            <Route path="strategies" element={<StrategiesPage />} />
            <Route path="rollouts" element={<RolloutsPage />} />
            <Route path="rollouts/:id" element={<RolloutDetailPage />} />
            <Route path="rollout-groups" element={<RolloutGroupsPage />} />
            <Route path="rollout-groups/:id" element={<RolloutGroupDetailPage />} />
            <Route path="settings" element={<SettingsPage level="org" />} />

            {/* Project-level — wrapped by ProjectPage (tabs) */}
            <Route path="projects/:projectSlug" element={<ProjectPage />}>
              <Route index element={<Navigate to="apps" replace />} />
              <Route path="apps" element={<ProjectAppsTab />} />
              <Route path="apps/new" element={<CreateAppPage />} />
              <Route path="flags" element={<FlagListPage />} />
              <Route path="flags/new" element={<FlagCreatePage />} />
              <Route path="flags/:id" element={<FlagDetailPage />} />
              <Route path="analytics" element={<AnalyticsPage />} />
              <Route path="settings" element={<SettingsPage level="project" />} />

              {/* App-level — wrapped by AppPage (tabs) */}
              <Route path="apps/:appSlug" element={<AppPage />}>
                <Route index element={<Navigate to="flags" replace />} />
                <Route path="flags" element={<FlagListPage />} />
                <Route path="flags/new" element={<FlagCreatePage />} />
                <Route path="flags/:id" element={<FlagDetailPage />} />
                <Route path="deployments" element={<DeploymentsPage />} />
                <Route path="deployments/:id" element={<DeploymentDetailPage />} />
                <Route path="releases" element={<ReleasesPage />} />
                <Route path="releases/:id" element={<ReleaseDetailPage />} />
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
      </Suspense>
    </AuthProvider>
  );
}
