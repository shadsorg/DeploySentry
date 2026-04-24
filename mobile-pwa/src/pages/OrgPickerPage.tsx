import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { orgsApi } from '../api';
import type { Organization } from '../types';

export function OrgPickerPage() {
  const nav = useNavigate();
  const [orgs, setOrgs] = useState<Organization[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    orgsApi
      .list()
      .then((r) => {
        if (r.organizations.length === 1) {
          const only = r.organizations[0];
          localStorage.setItem('ds_active_org', only.slug);
          nav(`/orgs/${only.slug}/status`, { replace: true });
        } else {
          setOrgs(r.organizations);
        }
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load organizations'));
  }, [nav]);

  if (error) {
    return (
      <div className="m-screen" style={{ padding: 20 }}>
        <p style={{ color: 'var(--color-danger, #ef4444)' }}>{error}</p>
      </div>
    );
  }
  if (orgs === null) {
    return <div className="m-page-loading">Loading…</div>;
  }
  if (orgs.length === 0) {
    return (
      <div className="m-screen" style={{ padding: 20 }}>
        <h2>No organizations</h2>
        <p style={{ color: 'var(--color-text-muted, #64748b)' }}>
          You&apos;re not a member of any organization. Ask an admin to invite you, or create one in the desktop dashboard.
        </p>
      </div>
    );
  }

  return (
    <div className="m-screen" style={{ padding: 20 }}>
      <h2 style={{ margin: '8px 0 16px' }}>Choose an organization</h2>
      <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
        {orgs.map((o) => (
          <li key={o.id} className="m-list-row">
            <button
              type="button"
              className="m-button"
              style={{ width: '100%', textAlign: 'left', justifyContent: 'flex-start' }}
              onClick={() => {
                localStorage.setItem('ds_active_org', o.slug);
                nav(`/orgs/${o.slug}/status`);
              }}
            >
              {o.name}
              <span style={{ color: 'var(--color-text-muted, #64748b)', marginLeft: 8, fontSize: 12 }}>{o.slug}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
