import { NavLink, useParams, useNavigate } from 'react-router-dom';
import { useAuth } from '@/auth';
import OrgSwitcher from './OrgSwitcher';
import ProjectSwitcher from './ProjectSwitcher';
import AppAccordion from './AppAccordion';

export default function Sidebar() {
  const { user, logout } = useAuth();
  const { orgSlug, projectSlug } = useParams();
  const navigate = useNavigate();

  function handleLogout() {
    logout();
    navigate('/login');
  }

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <div className="sidebar-logo">DS</div>
        <span className="sidebar-title">DeploySentry</span>
      </div>

      <div className="sidebar-switchers">
        <OrgSwitcher />
        {projectSlug && <ProjectSwitcher />}
      </div>

      <nav className="sidebar-nav">
        {/* App accordion — only when a project is selected */}
        {projectSlug && <AppAccordion />}

        {/* Project-level nav */}
        {projectSlug && orgSlug && (
          <>
            <NavLink
              to={`/orgs/${orgSlug}/projects/${projectSlug}/apps`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">□</span>
              Applications
            </NavLink>
            <div className="sidebar-section">Project</div>
            <NavLink
              to={`/orgs/${orgSlug}/projects/${projectSlug}/flags`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">#</span>
              Feature Flags
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/projects/${projectSlug}/analytics`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">%</span>
              Analytics
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/projects/${projectSlug}/sdks`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">{'{'}</span>
              SDKs & Docs
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/projects/${projectSlug}/settings`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">*</span>
              Settings
            </NavLink>
          </>
        )}

        {/* Org-level nav */}
        {orgSlug && (
          <>
            <div className="sidebar-section">Organization</div>
            <NavLink
              to={`/orgs/${orgSlug}/projects`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">□</span>
              Projects
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/members`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">@</span>
              Members
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/api-keys`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">!</span>
              API Keys
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/settings`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">*</span>
              Settings
            </NavLink>
          </>
        )}
      </nav>

      <div className="sidebar-footer">
        {user && (
          <div className="sidebar-user">
            <span className="text-sm">{user.name || user.email}</span>
            <button className="btn-link text-xs text-muted" onClick={handleLogout}>
              Sign out
            </button>
          </div>
        )}
        <div className="nav-item text-xs text-muted">v1.0.0</div>
      </div>
    </aside>
  );
}
