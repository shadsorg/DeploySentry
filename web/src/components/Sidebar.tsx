import { NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth';

const navItems = [
  { to: '/', label: 'Dashboard', icon: '~' },
  { to: '/flags', label: 'Feature Flags', icon: '#' },
  { to: '/deployments', label: 'Deployments', icon: '>' },
  { to: '/releases', label: 'Releases', icon: '@' },
];

const secondaryItems = [
  { to: '/sdks', label: 'SDKs & Docs', icon: '{' },
  { to: '/settings', label: 'Settings', icon: '*' },
];

export default function Sidebar() {
  const { user, logout } = useAuth();
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

      <nav className="sidebar-nav">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
          >
            <span className="nav-icon">{item.icon}</span>
            {item.label}
          </NavLink>
        ))}

        <div className="sidebar-section">Integrate</div>

        {secondaryItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
          >
            <span className="nav-icon">{item.icon}</span>
            {item.label}
          </NavLink>
        ))}
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
