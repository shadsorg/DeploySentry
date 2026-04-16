import { Link } from 'react-router-dom';
import { useAuth } from '@/authHooks';
import UserMenu from './UserMenu';

type SiteHeaderProps = {
  variant: 'landing' | 'app';
};

export default function SiteHeader({ variant }: SiteHeaderProps) {
  const { user } = useAuth();
  return (
    <header className="site-header">
      <Link to="/" className="site-header-brand" aria-label="DeploySentry home">
        <span className="site-header-logo">DS</span>
        <span className="site-header-wordmark">DeploySentry</span>
      </Link>

      {variant === 'landing' && (
        <nav className="site-header-nav">
          <a href="#pillars" className="site-header-link">Product</a>
          <Link to="/docs" className="site-header-link">Docs</Link>
          <Link to="/docs/sdks" className="site-header-link">SDKs</Link>
        </nav>
      )}

      {variant === 'app' && user && (
        <nav className="site-header-nav">
          <Link to="/docs" className="site-header-link">Docs</Link>
          <Link to="/docs/sdks" className="site-header-link">SDKs</Link>
        </nav>
      )}

      <div className="site-header-right">
        {!user && variant === 'landing' && (
          <>
            <Link to="/login" className="site-header-link">Log in</Link>
            <Link to="/register" className="btn-primary site-header-cta">Sign up</Link>
          </>
        )}
        {user && variant === 'landing' && (
          <Link to="/portal" className="btn-primary site-header-cta">Portal</Link>
        )}
        {user && <UserMenu />}
      </div>
    </header>
  );
}
