import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useProjects } from '@/hooks/useEntities';
import { entitiesApi } from '@/api';
import { ActiveRolloutsCard } from '@/components/rollout/ActiveRolloutsCard';

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
      <ActiveRolloutsCard orgSlug={orgSlug} />
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>Projects</h1>
          <p>Organize applications and feature flags by team or service area.</p>
        </div>
        <Link to={`/orgs/${orgSlug}/projects/new`} className="btn btn-primary">
          <span className="ms" style={{ fontSize: 16 }}>
            add
          </span>
          Create Project
        </Link>
      </div>
      {projects.length === 0 ? (
        <div className="empty-state card" style={{ padding: '48px 24px' }}>
          <span
            className="ms"
            style={{
              fontSize: 40,
              color: 'var(--color-text-muted)',
              marginBottom: 12,
              display: 'block',
            }}
          >
            account_tree
          </span>
          <h3>No projects yet</h3>
          <p>Create your first project to start managing deployments and flags.</p>
          <Link
            to={`/orgs/${orgSlug}/projects/new`}
            className="btn btn-primary"
            style={{ marginTop: 16 }}
          >
            <span className="ms" style={{ fontSize: 16 }}>
              add
            </span>
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
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
                  <div
                    style={{
                      width: 32,
                      height: 32,
                      borderRadius: 8,
                      background: 'var(--color-primary-bg)',
                      border: '1px solid rgba(99,102,241,0.2)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0,
                    }}
                  >
                    <span className="ms" style={{ fontSize: 16, color: 'var(--color-primary)' }}>
                      account_tree
                    </span>
                  </div>
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
