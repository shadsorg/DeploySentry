import { useParams, Link } from 'react-router-dom';
import { useProjects } from '@/hooks/useEntities';
import { entitiesApi } from '@/api';

export default function ProjectListPage() {
  const { orgSlug } = useParams();
  const { projects, loading, error, refresh } = useProjects(orgSlug, true);

  if (!orgSlug) return null;
  if (loading) return <div className="page-loading">Loading projects...</div>;
  if (error) return <div className="page-error">Error: {error}</div>;

  const handleRestore = async (slug: string) => {
    try {
      await entitiesApi.restoreProject(orgSlug, slug);
      refresh();
    } catch (err) {
      console.error('Failed to restore project:', err);
    }
  };

  const formatDate = (dateStr: string) => {
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', { month: '2-digit', day: '2-digit', year: 'numeric' });
  };

  const getHardDeleteDate = (deletedAt: string) => {
    const d = new Date(deletedAt);
    d.setDate(d.getDate() + 7);
    return formatDate(d.toISOString());
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
          {projects.map((project) => (
            <div
              key={project.id}
              className={`project-card${project.deleted_at ? ' project-card--deleted' : ''}`}
            >
              {project.deleted_at ? (
                <>
                  <div className="project-card-header">
                    <h3 className="project-card-name">{project.name}</h3>
                    <span className="badge badge-danger">Deleted</span>
                  </div>
                  <span className="project-card-slug">{project.slug}</span>
                  <p className="text-sm text-muted" style={{ marginTop: 8 }}>
                    Hard delete available on {getHardDeleteDate(project.deleted_at)}
                  </p>
                  <button
                    className="btn btn-sm"
                    style={{ marginTop: 8 }}
                    onClick={() => handleRestore(project.slug)}
                  >
                    Restore
                  </button>
                </>
              ) : (
                <>
                  <div className="project-card-header">
                    <Link
                      to={`/orgs/${orgSlug}/projects/${project.slug}/flags`}
                      className="project-card-name-link"
                    >
                      <h3 className="project-card-name">{project.name}</h3>
                    </Link>
                    <Link
                      to={`/orgs/${orgSlug}/projects/${project.slug}/settings`}
                      className="project-card-settings"
                      title="Project Settings"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                        <path d="M8 4.754a3.246 3.246 0 1 0 0 6.492 3.246 3.246 0 0 0 0-6.492zM5.754 8a2.246 2.246 0 1 1 4.492 0 2.246 2.246 0 0 1-4.492 0z" />
                        <path d="M9.796 1.343c-.527-1.79-3.065-1.79-3.592 0l-.094.319a.873.873 0 0 1-1.255.52l-.292-.16c-1.64-.892-3.433.902-2.54 2.541l.159.292a.873.873 0 0 1-.52 1.255l-.319.094c-1.79.527-1.79 3.065 0 3.592l.319.094a.873.873 0 0 1 .52 1.255l-.16.292c-.892 1.64.901 3.434 2.541 2.54l.292-.159a.873.873 0 0 1 1.255.52l.094.319c.527 1.79 3.065 1.79 3.592 0l.094-.319a.873.873 0 0 1 1.255-.52l.292.16c1.64.893 3.434-.902 2.54-2.541l-.159-.292a.873.873 0 0 1 .52-1.255l.319-.094c1.79-.527 1.79-3.065 0-3.592l-.319-.094a.873.873 0 0 1-.52-1.255l.16-.292c.893-1.64-.902-3.433-2.541-2.54l-.292.159a.873.873 0 0 1-1.255-.52l-.094-.319zm-2.633.283c.246-.835 1.428-.835 1.674 0l.094.319a1.873 1.873 0 0 0 2.693 1.115l.291-.16c.764-.415 1.6.42 1.184 1.185l-.159.292a1.873 1.873 0 0 0 1.116 2.692l.318.094c.835.246.835 1.428 0 1.674l-.319.094a1.873 1.873 0 0 0-1.115 2.693l.16.291c.415.764-.421 1.6-1.185 1.184l-.291-.159a1.873 1.873 0 0 0-2.693 1.116l-.094.318c-.246.835-1.428.835-1.674 0l-.094-.319a1.873 1.873 0 0 0-2.692-1.115l-.292.16c-.764.415-1.6-.421-1.184-1.185l.159-.291A1.873 1.873 0 0 0 1.945 8.93l-.319-.094c-.835-.246-.835-1.428 0-1.674l.319-.094A1.873 1.873 0 0 0 3.06 4.377l-.16-.292c-.415-.764.42-1.6 1.185-1.184l.292.159a1.873 1.873 0 0 0 2.692-1.116l.094-.318z" />
                      </svg>
                    </Link>
                  </div>
                  <Link
                    to={`/orgs/${orgSlug}/projects/${project.slug}/flags`}
                    className="project-card-slug"
                  >
                    {project.slug}
                  </Link>
                </>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
