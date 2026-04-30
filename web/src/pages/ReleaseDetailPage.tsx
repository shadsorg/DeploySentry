import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import ActionBar from '@/components/ActionBar';
import { releasesApi } from '@/api';
import type { Release, ReleaseStatus } from '@/types';

function getReleaseActions(status: ReleaseStatus) {
  const noop = () => {};
  switch (status) {
    case 'draft':
      return {
        primaryAction: { label: 'Start Rollout', onClick: noop },
        secondaryActions: [{ label: 'Delete', onClick: noop, variant: 'danger' as const }],
      };
    case 'rolling_out':
      return {
        primaryAction: { label: 'Promote', onClick: noop },
        secondaryActions: [
          { label: 'Pause', onClick: noop },
          { label: 'Rollback', onClick: noop, variant: 'danger' as const },
        ],
      };
    case 'paused':
      return {
        primaryAction: { label: 'Resume', onClick: noop },
        secondaryActions: [{ label: 'Rollback', onClick: noop, variant: 'danger' as const }],
      };
    default:
      return {};
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'rolling_out':
      return 'badge badge-active';
    case 'completed':
      return 'badge badge-completed';
    case 'rolled_back':
      return 'badge badge-danger';
    case 'paused':
      return 'badge badge-ops';
    case 'draft':
      return 'badge badge-pending';
    default:
      return 'badge';
  }
}

export default function ReleaseDetailPage() {
  const { id, orgSlug, projectSlug, appSlug } = useParams();
  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/releases`;

  const [release, setRelease] = useState<Release | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);

    releasesApi
      .get(id)
      .then((data) => setRelease(data))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <div className="empty-state" style={{ padding: '40px 0' }}>
        <span
          className="ms"
          style={{
            fontSize: 32,
            color: 'var(--color-text-muted)',
            animation: 'spin 1s linear infinite',
          }}
        >
          sync
        </span>
        <p style={{ color: 'var(--color-text-muted)', marginTop: 8 }}>Loading release…</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="empty-state card" style={{ padding: '48px 24px' }}>
        <span className="ms" style={{ fontSize: 36, color: 'var(--color-danger)' }}>
          error
        </span>
        <h3>Failed to load</h3>
        <p>{error}</p>
      </div>
    );
  }

  if (!release) {
    return (
      <div className="empty-state card" style={{ padding: '48px 24px' }}>
        <span className="ms" style={{ fontSize: 36, color: 'var(--color-text-muted)' }}>
          local_shipping
        </span>
        <h3>Release not found</h3>
        <p>This release may have been deleted.</p>
        <Link to={backPath} className="btn btn-secondary btn-sm" style={{ marginTop: 12 }}>
          Back to Releases
        </Link>
      </div>
    );
  }

  const actions = getReleaseActions(release.status);

  return (
    <div className="page">
      <div className="page-header-row">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <Link
            to={backPath}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 4,
              fontSize: 13,
              color: 'var(--color-text-muted)',
              textDecoration: 'none',
            }}
          >
            <span className="ms" style={{ fontSize: 14 }}>
              arrow_back
            </span>
            Releases
          </Link>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 10,
                background: 'var(--color-primary-bg)',
                border: '1px solid rgba(99,102,241,0.25)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
              }}
            >
              <span className="ms" style={{ fontSize: 20, color: 'var(--color-primary)' }}>
                local_shipping
              </span>
            </div>
            <div className="page-header" style={{ marginBottom: 0 }}>
              <h1
                style={{
                  fontFamily: 'var(--font-display)',
                  fontWeight: 800,
                  letterSpacing: '-0.02em',
                }}
              >
                {release.name}
              </h1>
              {release.description && <p>{release.description}</p>}
            </div>
          </div>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              flexWrap: 'wrap',
              paddingLeft: 48,
            }}
          >
            <span className={statusBadgeClass(release.status)}>
              {release.status.replace(/_/g, ' ')}
            </span>
            <span
              className="badge"
              style={{
                background: 'var(--color-bg-elevated)',
                color: 'var(--color-text-secondary)',
              }}
            >
              {release.traffic_percent}% traffic
            </span>
            {release.session_sticky && (
              <span
                className="badge"
                style={{
                  background: 'var(--color-warning-bg)',
                  color: 'var(--color-warning)',
                  gap: 4,
                }}
              >
                <span className="ms" style={{ fontSize: 13 }}>
                  lock
                </span>
                Session sticky: {release.sticky_header}
              </span>
            )}
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <ActionBar {...actions} />
        </div>
      </div>

      {/* Flag Changes */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div
          style={{
            padding: '12px 20px',
            borderBottom: '1px solid var(--color-border)',
            display: 'flex',
            alignItems: 'center',
            gap: 8,
          }}
        >
          <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>
            toggle_on
          </span>
          <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>
            Flag Changes
          </span>
        </div>
        <div className="empty-state" style={{ padding: '48px 24px' }}>
          <span className="ms" style={{ fontSize: 36, color: 'var(--color-text-muted)' }}>
            toggle_off
          </span>
          <h3>No flag changes</h3>
          <p>Flag changes associated with this release will appear here.</p>
        </div>
      </div>
    </div>
  );
}
