import { Link } from 'react-router-dom';
import { useAuth } from '@/authHooks';
import UserMenu from './UserMenu';
import logoUrl from '@/assets/deploy-sentry-logo-dark.svg';

type SiteHeaderProps = {
  variant: 'landing' | 'app';
  size?: 'default' | 'large';
};

export default function SiteHeader({ variant, size = 'default' }: SiteHeaderProps) {
  const { user } = useAuth();
  const headerClass = size === 'large' ? 'site-header site-header--large' : 'site-header';
  return (
    <header className={headerClass}>
      <Link to="/" className="site-header-brand" aria-label="Deploy Sentry home">
        <img src={logoUrl} alt="Deploy Sentry" className="site-header-logo-img" />
      </Link>

      {variant === 'landing' && (
        <nav className="site-header-nav">
          <a href="#pillars" className="site-header-link">Product</a>
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
