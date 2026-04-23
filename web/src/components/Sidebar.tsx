import { NavLink, useParams } from 'react-router-dom';
import OrgSwitcher from './OrgSwitcher';

export default function Sidebar() {
  const { orgSlug } = useParams();

  return (
    <aside className="sidebar">
      <div className="sidebar-switchers">
        <OrgSwitcher />
      </div>

      <nav className="sidebar-nav">
        {orgSlug && (
          <>
            <NavLink
              to={`/orgs/${orgSlug}/status`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">▦</span>
              Status
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/deployments`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">⧗</span>
              Deploy History
            </NavLink>
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
              to={`/orgs/${orgSlug}/strategies`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">~</span>
              Strategies
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/rollouts`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">↻</span>
              Rollouts
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/rollout-groups`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">⊕</span>
              Rollout Groups
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
        <NavLink
          to="/docs"
          className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
        >
          <span className="nav-icon">?</span>
          Documentation
        </NavLink>
      </nav>
    </aside>
  );
}
