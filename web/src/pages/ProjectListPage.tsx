import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useProjects } from '@/hooks/useEntities';
import { entitiesApi } from '@/api';

export default function ProjectListPage() {
  const { orgSlug } = useParams();
  const { projects, loading, error, refresh } = useProjects(orgSlug, true);
  const [restoring, setRestoring] = useState<string | null>(null);

  if (!orgSlug) return null;
  if (loading) return <div className="page-loading">Loading projects...</div>;
  if (error) return <div className="page-error">Error: {error}</div>;

  const handleRestore = async (projectSlug: string) => {
    setRestoring(projectSlug);
    try {
      await entitiesApi.restoreProject(orgSlug, projectSlug);
      refresh();
    } catch (err) {
      console.error('Failed to restore project:', err);
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
        <h1 className="page-header">Projects</h1>
        <Link to={`/orgs/${orgSlug}/projects/new`} className="btn btn-primary">
          Create Project
        </Link>
      </div>
      {projects.length === 0 ? (
        <div className="empty-state">
          <p>No projects yet.</p>
          <Link to={`/orgs/${orgSlug}/projects/new`} className="btn btn-primary">
            Create Your First Project
          </Link>
        </div>
      ) : (
        <div className="project-card-grid">
          {projects.map((project) => {
            const isDeleted = !!project.deleted_at;
            return (
              <div
                key={project.id}
                className="project-card"
                style={isDeleted ? { opacity: 0.5, pointerEvents: 'auto' } : undefined}
              >
                <div className="flex items-center gap-2" style={{ justifyContent: 'space-between' }}>
                  <Link
                    to={`/orgs/${orgSlug}/projects/${project.slug}/apps`}
                    style={{ textDecoration: 'none', color: 'inherit', flex: 1 }}
                  >
                    <h3 className="project-card-name" style={{ margin: 0 }}>
                      {project.name}
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
                </div>
                <Link
                  to={`/orgs/${orgSlug}/projects/${project.slug}/apps`}
                  style={{ textDecoration: 'none', color: 'inherit' }}
                >
                  <span className="project-card-slug">{project.slug}</span>
                </Link>
                {isDeleted && project.deleted_at && (
                  <div style={{ marginTop: 8 }}>
                    <p className="text-muted text-sm" style={{ margin: '4px 0' }}>
                      Hard delete available on {formatHardDeleteDate(project.deleted_at)}
                    </p>
                    <button
                      className="btn btn-sm"
                      onClick={() => handleRestore(project.slug)}
                      disabled={restoring === project.slug}
                    >
                      {restoring === project.slug ? 'Restoring...' : 'Restore'}
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
