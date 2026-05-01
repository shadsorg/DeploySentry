import { NavLink, useParams } from 'react-router-dom';
import OrgSwitcher from './OrgSwitcher';
import { useOrgRole } from '@/hooks/useOrgRole';

type NavItem = { to: string; icon: string; label: string; hideForViewer?: boolean };

const NAV_ITEMS: NavItem[] = [
  { to: 'status', icon: 'dashboard', label: 'Status' },
  { to: 'deployments', icon: 'history', label: 'Deploy History' },
  { to: 'projects', icon: 'account_tree', label: 'Projects' },
  { to: 'members', icon: 'group', label: 'Members' },
  { to: 'api-keys', icon: 'vpn_key', label: 'API Keys' },
  { to: 'audit', icon: 'history_edu', label: 'Audit', hideForViewer: true },
  { to: 'strategies', icon: 'architecture', label: 'Strategies' },
  { to: 'rollouts', icon: 'dynamic_feed', label: 'Rollouts' },
  { to: 'rollout-groups', icon: 'layers', label: 'Rollout Groups' },
  { to: 'settings', icon: 'settings', label: 'Settings' },
];

export default function Sidebar() {
  const { orgSlug } = useParams();
  const { role } = useOrgRole(orgSlug);
  const items = NAV_ITEMS.filter((item) => !(item.hideForViewer && role === 'viewer'));

  return (
    <aside className="sidebar">
      <div className="sidebar-switchers">
        <OrgSwitcher />
      </div>

      <nav className="sidebar-nav">
        {orgSlug &&
          items.map(({ to, icon, label }) => (
            <NavLink
              key={to}
              to={`/orgs/${orgSlug}/${to}`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="ms nav-icon" style={{ fontSize: 18 }}>
                {icon}
              </span>
              {label}
            </NavLink>
          ))}

        <div className="sidebar-section" style={{ marginTop: 20 }}>
          Help
        </div>
        <NavLink to="/docs" className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}>
          <span className="ms nav-icon" style={{ fontSize: 18 }}>
            description
          </span>
          Documentation
        </NavLink>
      </nav>
    </aside>
  );
}
