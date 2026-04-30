import { NavLink, Outlet, useParams } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { entitiesApi } from '@/api';
import type { Project } from '@/types';

export default function ProjectPage() {
  const { orgSlug, projectSlug } = useParams();
  const [project, setProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!orgSlug || !projectSlug) return;
    entitiesApi
      .getProject(orgSlug, projectSlug)
      .then(setProject)
      .catch(() => setProject(null))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug]);

  if (!orgSlug || !projectSlug) return null;
  if (loading) return <div className="page-loading">Loading project...</div>;
  if (!project) return <div className="page-error">Project not found</div>;

  const base = `/orgs/${orgSlug}/projects/${projectSlug}`;

  return (
    <div>
      <div className="content-header">
        <h1 className="content-title">{project.name}</h1>
      </div>
      <NavLink to={`/orgs/${orgSlug}/projects`} className="back-link">
        ← Back to Projects
      </NavLink>
      <div className="tabs">
        <NavLink
          to={`${base}/apps`}
          className={({ isActive }) => `tab${isActive ? ' active' : ''}`}
          end
        >
          Applications
        </NavLink>
        <NavLink
          to={`${base}/flags`}
          className={({ isActive }) => `tab${isActive ? ' active' : ''}`}
        >
          Flags
        </NavLink>
        <NavLink
          to={`${base}/analytics`}
          className={({ isActive }) => `tab${isActive ? ' active' : ''}`}
        >
          Analytics
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
