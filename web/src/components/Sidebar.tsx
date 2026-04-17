import { NavLink, useParams } from 'react-router-dom';
import OrgSwitcher from './OrgSwitcher';
import ProjectSwitcher from './ProjectSwitcher';
import AppAccordion from './AppAccordion';

export default function Sidebar() {
  const { orgSlug, projectSlug } = useParams();

  return (
    <aside className="sidebar">
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
        <div className="sidebar-section">Help</div>
        <NavLink to="/docs" className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}>
          <span className="nav-icon">?</span>
          Documentation
        </NavLink>
      </nav>
    </aside>
  );
}
