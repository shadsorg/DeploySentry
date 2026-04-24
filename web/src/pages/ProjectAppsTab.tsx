import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useApps } from '@/hooks/useEntities';
import { entitiesApi } from '@/api';

export default function ProjectAppsTab() {
  const { orgSlug, projectSlug } = useParams();
  const { apps, loading, error, refresh } = useApps(orgSlug!, projectSlug!, true);
  const [restoring, setRestoring] = useState<string | null>(null);

  if (!orgSlug || !projectSlug) return null;
  if (loading) return <div className="page-loading">Loading applications...</div>;
  if (error) return <div className="page-error">Error: {error}</div>;

  const handleRestore = async (appSlug: string) => {
    setRestoring(appSlug);
    try {
      await entitiesApi.restoreApp(orgSlug, projectSlug, appSlug);
      refresh();
    } catch (err) {
      console.error('Failed to restore app:', err);
    } finally {
      setRestoring(null);
    }
  };

  const formatHardDeleteDate = (deletedAt: string) => {
    const date = new Date(deletedAt);
    date.setDate(date.getDate() + 7);
    return date.toLocaleDateString();
  };

  const base = `/orgs/${orgSlug}/projects/${projectSlug}`;

  return (
    <div>
      <div className="page-header-row" style={{ marginBottom: 16 }}>
        <h2 style={{ margin: 0, fontFamily: 'var(--font-display)', fontWeight: 700 }}>Applications</h2>
        <Link to={`${base}/apps/new`} className="btn btn-primary">
          <span className="ms" style={{ fontSize: 16 }}>add</span>
          Add Application
        </Link>
      </div>
      {apps.length === 0 ? (
        <div className="empty-state card" style={{ padding: '48px 24px' }}>
          <span className="ms" style={{ fontSize: 40, color: 'var(--color-text-muted)', marginBottom: 12, display: 'block' }}>apps</span>
          <h3>No applications yet</h3>
          <p>Add an application to start managing deployments and feature flags.</p>
          <Link to={`${base}/apps/new`} className="btn btn-primary" style={{ marginTop: 16 }}>
            <span className="ms" style={{ fontSize: 16 }}>add</span>
            Create Your First Application
          </Link>
        </div>
      ) : (
        <div className="project-card-grid">
          {apps.map((app) => {
            const isDeleted = !!app.deleted_at;
            return (
              <div key={app.id} className="project-card" style={isDeleted ? { opacity: 0.5 } : undefined}>
                <Link to={`${base}/apps/${app.slug}/flags`} style={{ textDecoration: 'none', color: 'inherit' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
                    <div style={{
                      width: 32, height: 32, borderRadius: 8,
                      background: 'var(--color-primary-bg)', border: '1px solid rgba(99,102,241,0.2)',
                      display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
                    }}>
                      <span className="ms" style={{ fontSize: 16, color: 'var(--color-primary)' }}>apps</span>
                    </div>
                    <h3 className="project-card-name" style={{ margin: 0 }}>
                      {app.name}
                      {isDeleted && <span className="badge badge-disabled" style={{ marginLeft: 8, fontSize: 11 }}>Deleted</span>}
                    </h3>
                  </div>
                  <span className="project-card-slug">{app.slug}</span>
                </Link>
                {isDeleted && app.deleted_at && (
                  <div style={{ marginTop: 8 }}>
                    <p className="text-muted text-sm" style={{ margin: '4px 0' }}>Hard delete available on {formatHardDeleteDate(app.deleted_at)}</p>
                    <button className="btn btn-sm" onClick={() => handleRestore(app.slug)} disabled={restoring === app.slug}>
                      {restoring === app.slug ? 'Restoring...' : 'Restore'}
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
