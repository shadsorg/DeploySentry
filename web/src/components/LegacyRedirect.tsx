import { Navigate } from 'react-router-dom';

interface LegacyRedirectProps {
  to: string;
}

export default function LegacyRedirect({ to }: LegacyRedirectProps) {
  const lastOrg = localStorage.getItem('ds_last_org') || '';
  const lastProject = localStorage.getItem('ds_last_project') || '';
  const lastApp = localStorage.getItem('ds_last_app') || '';

  if (!lastOrg) return <Navigate to="/orgs/new" replace />;
  if (to === 'settings') return <Navigate to={`/orgs/${lastOrg}/settings`} replace />;
  if (!lastProject) return <Navigate to={`/orgs/${lastOrg}/projects`} replace />;

  if ((to === 'deployments' || to === 'releases') && lastApp) {
    return (
      <Navigate to={`/orgs/${lastOrg}/projects/${lastProject}/apps/${lastApp}/${to}`} replace />
    );
  }
  if (to === 'deployments' || to === 'releases') {
    return <Navigate to={`/orgs/${lastOrg}/projects/${lastProject}/flags`} replace />;
  }
  return <Navigate to={`/orgs/${lastOrg}/projects/${lastProject}/${to}`} replace />;
}
