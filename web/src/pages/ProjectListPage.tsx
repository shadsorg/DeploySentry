import { useParams, Link } from 'react-router-dom';
import { getMockProjects, getMockApps, getOrgName } from '@/mocks/hierarchy';

export default function ProjectListPage() {
  const { orgSlug } = useParams();
  if (!orgSlug) return null;

  const projects = getMockProjects(orgSlug);
  const orgName = getOrgName(orgSlug);

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">{orgName} — Projects</h1>
        <button className="btn btn-primary" disabled>Create Project</button>
      </div>

      {projects.length === 0 ? (
        <div className="empty-state">
          <p>No projects yet. Create one to get started.</p>
        </div>
      ) : (
        <div className="project-card-grid">
          {projects.map((project) => {
            const apps = getMockApps(project.slug);
            return (
              <Link
                key={project.id}
                to={`/orgs/${orgSlug}/projects/${project.slug}/flags`}
                className="project-card"
              >
                <h3 className="project-card-name">{project.name}</h3>
                <span className="project-card-slug">{project.slug}</span>
                <div className="project-card-meta">
                  {apps.length} application{apps.length !== 1 ? 's' : ''}
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
