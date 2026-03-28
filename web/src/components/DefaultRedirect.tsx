import { Navigate } from 'react-router-dom';
import { MOCK_ORGS } from '@/mocks/hierarchy';

export default function DefaultRedirect() {
  const lastOrg = localStorage.getItem('ds_last_org');

  if (lastOrg) {
    return <Navigate to={`/orgs/${lastOrg}/projects`} replace />;
  }

  // Fall back to first org or create new
  if (MOCK_ORGS.length > 0) {
    return <Navigate to={`/orgs/${MOCK_ORGS[0].slug}/projects`} replace />;
  }

  return <Navigate to="/orgs/new" replace />;
}
