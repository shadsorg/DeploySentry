import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useApps } from '@/hooks/useEntities';
import { entitiesApi } from '@/api';

export default function ApplicationsListPage() {
  const { orgSlug, projectSlug } = useParams();
  const { apps, loading, error, refresh } = useApps(orgSlug, projectSlug, true);
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

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">Applications</h1>
        <Link to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/new`} className="btn btn-primary">
          Create App
        </Link>
      </div>
      {apps.length === 0 ? (
        <div className="empty-state">
          <p>No applications yet.</p>
          <Link
            to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/new`}
            className="btn btn-primary"
          >
            Create Your First App
          </Link>
        </div>
      ) : (
        <div className="project-card-grid">
          {apps.map((app) => {
            const isDeleted = !!app.deleted_at;
            return (
              <div
                key={app.id}
                className="project-card"
                style={isDeleted ? { opacity: 0.5, pointerEvents: 'auto' } : undefined}
              >
                <div className="flex items-center gap-2" style={{ justifyContent: 'space-between' }}>
                  <Link
                    to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/${app.slug}/deployments`}
                    style={{ textDecoration: 'none', color: 'inherit', flex: 1 }}
                  >
                    <h3 className="project-card-name" style={{ margin: 0 }}>
                      {app.name}
                      {isDeleted && (
                        <span
                          className="badge badge-disabled"
                          style={{ marginLeft: 8, fontSize: 11 }}
                        >
                          Deleted
                        </span>
                      )}
                    </h3>
                  </Link>
                  {!isDeleted && (
                    <Link
                      to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/${app.slug}/settings`}
                      title="App Settings"
                      style={{ textDecoration: 'none', fontSize: 18, lineHeight: 1 }}
                      onClick={(e) => e.stopPropagation()}
                    >
                      &#x2699;
                    </Link>
                  )}
                </div>
                <Link
                  to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/${app.slug}/deployments`}
                  style={{ textDecoration: 'none', color: 'inherit' }}
                >
                  <span className="project-card-slug">{app.slug}</span>
                  {app.description && <p className="project-card-desc">{app.description}</p>}
                </Link>
                {isDeleted && app.deleted_at && (
                  <div style={{ marginTop: 8 }}>
                    <p className="text-muted text-sm" style={{ margin: '4px 0' }}>
                      Hard delete available on {formatHardDeleteDate(app.deleted_at)}
                    </p>
                    <button
                      className="btn btn-sm"
                      onClick={() => handleRestore(app.slug)}
                      disabled={restoring === app.slug}
                    >
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
