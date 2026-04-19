import { NavLink, Outlet, useParams } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { entitiesApi } from '@/api';
import type { Application } from '@/types';

export default function AppPage() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const [app, setApp] = useState<Application | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!orgSlug || !projectSlug || !appSlug) return;
    entitiesApi
      .getApp(orgSlug, projectSlug, appSlug)
      .then(setApp)
      .catch(() => setApp(null))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  if (!orgSlug || !projectSlug || !appSlug) return null;
  if (loading) return <div className="page-loading">Loading application...</div>;
  if (!app) return <div className="page-error">Application not found</div>;

  const base = `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`;

  return (
    <div>
      <div className="content-header">
        <h1 className="content-title">{app.name}</h1>
      </div>
      <NavLink to={`/orgs/${orgSlug}/projects/${projectSlug}/apps`} className="back-link">
        ← Back to {projectSlug}
      </NavLink>
      <div className="tabs">
        <NavLink
          to={`${base}/flags`}
          className={({ isActive }) => `tab${isActive ? ' active' : ''}`}
        >
          Flags
        </NavLink>
        <NavLink
          to={`${base}/deployments`}
          className={({ isActive }) => `tab${isActive ? ' active' : ''}`}
        >
          Deployments
        </NavLink>
        <NavLink
          to={`${base}/releases`}
          className={({ isActive }) => `tab${isActive ? ' active' : ''}`}
        >
          Releases
        </NavLink>
        <NavLink
          to={`${base}/settings`}
          className={({ isActive }) => `tab${isActive ? ' active' : ''}`}
        >
          Settings
        </NavLink>
      </div>
      <Outlet />
    </div>
  );
}
