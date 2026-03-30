import { useParams, Link } from 'react-router-dom';
import { useProjects } from '@/hooks/useEntities';

export default function ProjectListPage() {
  const { orgSlug } = useParams();
  const { projects, loading, error } = useProjects(orgSlug);

  if (!orgSlug) return null;
  if (loading) return <div className="page-loading">Loading projects...</div>;
  if (error) return <div className="page-error">Error: {error}</div>;

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
            <Link
              key={project.id}
              to={`/orgs/${orgSlug}/projects/${project.slug}/flags`}
              className="project-card"
            >
              <h3 className="project-card-name">{project.name}</h3>
              <span className="project-card-slug">{project.slug}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
