import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { entitiesApi } from '@/api';

export default function DefaultRedirect() {
  const [target, setTarget] = useState<string | null>(null);

  useEffect(() => {
    const lastOrg = localStorage.getItem('ds_last_org');
    if (lastOrg) {
      setTarget(`/orgs/${lastOrg}/projects`);
      return;
    }
    entitiesApi
      .listOrgs()
      .then((res) => {
        const orgs = res.organizations ?? [];
        if (orgs.length > 0) {
          setTarget(`/orgs/${orgs[0].slug}/projects`);
        } else {
          setTarget('/orgs/new');
        }
      })
      .catch(() => setTarget('/orgs/new'));
  }, []);

  if (!target) return <div className="page-loading">Loading...</div>;
  return <Navigate to={target} replace />;
}
