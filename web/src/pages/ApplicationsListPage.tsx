import { useParams, Link } from 'react-router-dom';
import { useApps } from '@/hooks/useEntities';

export default function ApplicationsListPage() {
  const { orgSlug, projectSlug } = useParams();
  const { apps, loading, error } = useApps(orgSlug, projectSlug);

  if (!orgSlug || !projectSlug) return null;
  if (loading) return <div className="page-loading">Loading applications...</div>;
  if (error) return <div className="page-error">Error: {error}</div>;

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
          {apps.map((app) => (
            <Link
              key={app.id}
              to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/${app.slug}/deployments`}
              className="project-card"
            >
              <h3 className="project-card-name">{app.name}</h3>
              <span className="project-card-slug">{app.slug}</span>
              {app.description && <p className="project-card-desc">{app.description}</p>}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
