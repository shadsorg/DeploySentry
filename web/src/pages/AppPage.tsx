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
      <div style={{ marginBottom: 4 }}>
        <NavLink
          to={`/orgs/${orgSlug}/projects/${projectSlug}/apps`}
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 4,
            fontSize: 13,
            color: 'var(--color-text-muted)',
            textDecoration: 'none',
          }}
        >
          <span className="ms" style={{ fontSize: 14 }}>
            arrow_back
          </span>
          {projectSlug}
        </NavLink>
      </div>
      <div className="page-header-row" style={{ marginBottom: 0 }}>
        <div className="page-header" style={{ marginBottom: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 10,
                background: 'var(--color-primary-bg)',
                border: '1px solid rgba(99,102,241,0.25)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>
                apps
              </span>
            </div>
            <h1 style={{ margin: 0 }}>{app.name}</h1>
          </div>
        </div>
      </div>
      <div className="tabs" style={{ marginTop: 16 }}>
        {[
          { to: `${base}/flags`, icon: 'toggle_on', label: 'Flags' },
          { to: `${base}/deployments`, icon: 'rocket_launch', label: 'Deployments' },
          { to: `${base}/releases`, icon: 'local_shipping', label: 'Releases' },
          { to: `${base}/settings`, icon: 'settings', label: 'Settings' },
        ].map(({ to, icon, label }) => (
          <NavLink key={to} to={to} className={({ isActive }) => `tab${isActive ? ' active' : ''}`}>
            <span className="ms" style={{ fontSize: 15, verticalAlign: 'middle', marginRight: 6 }}>
              {icon}
            </span>
            {label}
          </NavLink>
        ))}
      </div>
      <Outlet />
    </div>
  );
}
