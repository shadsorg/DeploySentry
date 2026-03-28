import { Navigate } from 'react-router-dom';
import { MOCK_ORGS, MOCK_PROJECTS } from '@/mocks/hierarchy';

interface LegacyRedirectProps {
  to: string;
}

export default function LegacyRedirect({ to }: LegacyRedirectProps) {
  const lastOrg = localStorage.getItem('ds_last_org') || MOCK_ORGS[0]?.slug || '';
  const lastProject = localStorage.getItem('ds_last_project') || '';

  if (!lastOrg) {
    return <Navigate to="/orgs/new" replace />;
  }

  // Settings goes to org level
  if (to === 'settings') {
    return <Navigate to={`/orgs/${lastOrg}/settings`} replace />;
  }

  // Project-level pages
  const projectSlug = lastProject || MOCK_PROJECTS.find((p) => {
    const org = MOCK_ORGS.find((o) => o.slug === lastOrg);
    return org && p.org_id === org.id;
  })?.slug || '';

  if (!projectSlug) {
    return <Navigate to={`/orgs/${lastOrg}/projects`} replace />;
  }

  // App-level pages (deployments, releases)
  if (to === 'deployments' || to === 'releases') {
    const lastApp = localStorage.getItem('ds_last_app') || '';
    if (lastApp) {
      return <Navigate to={`/orgs/${lastOrg}/projects/${projectSlug}/apps/${lastApp}/${to}`} replace />;
    }
    // No app context — go to project
    return <Navigate to={`/orgs/${lastOrg}/projects/${projectSlug}/flags`} replace />;
  }

  return <Navigate to={`/orgs/${lastOrg}/projects/${projectSlug}/${to}`} replace />;
}
