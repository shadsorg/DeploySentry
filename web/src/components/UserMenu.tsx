import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '@/authHooks';

function getInitials(user: { name?: string; email: string }): string {
  if (user.name && user.name.trim().length > 0) {
    const parts = user.name.trim().split(/\s+/);
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  }
  return user.email.slice(0, 2).toUpperCase();
}

export default function UserMenu() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { orgSlug } = useParams();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    function onMouseDown(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('keydown', onKey);
    document.addEventListener('mousedown', onMouseDown);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('mousedown', onMouseDown);
    };
  }, [open]);

  if (!user) return null;

  const settingsOrg = orgSlug || localStorage.getItem('ds_last_org') || '';
  const settingsHref = settingsOrg ? `/orgs/${settingsOrg}/settings` : '/portal';

  function handleLogout() {
    logout();
    setOpen(false);
    navigate('/');
  }

  return (
    <div className="user-menu" ref={containerRef}>
      <button
        type="button"
        className="user-menu-trigger"
        aria-label="User menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        {getInitials(user)}
      </button>
      {open && (
        <div className="user-menu-dropdown" role="menu">
          <div className="user-menu-header">
            <div className="user-menu-name">{user.name || user.email}</div>
            <div className="user-menu-email">{user.email}</div>
          </div>
          <Link to={settingsHref} className="user-menu-item" onClick={() => setOpen(false)}>
            Settings
          </Link>
          <button type="button" className="user-menu-item" onClick={handleLogout}>
            Logout
          </button>
        </div>
      )}
    </div>
  );
}
